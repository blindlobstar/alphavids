package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const (
	videos_path = "videos"
)

var (
	meter              = otel.Meter("github.com/blindlobstar/alphavids")
	transcodeHistogram api.Int64Histogram
)

func main() {
	var err error
	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName("alphavids"),
			semconv.ServiceVersion("0.0.1"),
		))
	if err != nil {
		slog.Error("error creating resource", "error", err)
		return
	}

	exporter, err := prometheus.New()
	if err != nil {
		slog.Error("error creating exporter", "error", err)
		return
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(exporter),
	)

	// Handle shutdown properly so nothing leaks.
	defer func() {
		if err := meterProvider.Shutdown(context.Background()); err != nil {
			log.Println(err)
		}
	}()

	otel.SetMeterProvider(meterProvider)

	transcodeHistogram, err = meter.Int64Histogram("alphavids.transcode",
		api.WithDescription("video transcoding"),
		api.WithUnit("ms"))
	if err != nil {
		slog.Error("error creating metrics", "error", err)
		return
	}

	if err := os.MkdirAll(videos_path, os.ModeDir); err != nil {
		slog.Error("error creating path for storing videos", "error", err)
		return
	}

	ticker := time.NewTicker(time.Minute)
	go func() {
		for range ticker.C {
			if err := deleteOldFiles(videos_path); err != nil {
				slog.Error("error deleting old files", "error", err)
			}
		}
	}()

	http.HandleFunc("POST /transcode", transcodeHandler)
	http.Handle("GET /", http.FileServer(http.Dir("./static")))
	http.Handle("GET /metrics", promhttp.Handler())
	http.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	slog.Info("server started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("error running server", "error", err)
	}
}

func deleteOldFiles(path string) error {
	afterDate := time.Now().Add(-120 * time.Second)
	return filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			// skip file
			return err
		}

		// if file created within the last 10 minutes - skip
		if info.ModTime().After(afterDate) {
			return nil
		}

		return os.RemoveAll(filepath.Dir(path))
	})
}

func transcodeHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		slog.Error("error parsing multipart form", "error", err)
		writeFailedResponse(w, http.StatusBadRequest, "File is too large, max size is 50mb")
		return
	}

	file, fileHeader, err := r.FormFile("upload")
	if err != nil {
		slog.Error("error reading file from form", "error", err)
		writeFailedResponse(w, http.StatusBadRequest, "No file attached")
		return
	}
	defer file.Close()

	start := time.Now()
	fpath, err := transcodeWebmToMOV(file, fileHeader.Filename)
	duration := time.Since(start)
	if err != nil {
		transcodeHistogram.Record(r.Context(), duration.Milliseconds(), api.WithAttributes(attribute.String("status", "ERROR")))
		slog.Error("error transcoding file", "filename", fileHeader.Filename, "file_size", fileHeader.Size, "error", err)
		writeFailedResponse(w, http.StatusInternalServerError, "Something went wrong. Please try again later")
		return
	}
	transcodeHistogram.Record(r.Context(), duration.Milliseconds(), api.WithAttributes(attribute.String("status", "OK")))

	defer func() {
		if err := os.RemoveAll(filepath.Dir(fpath)); err != nil {
			slog.Error("error removing processed file", "error", err)
		}
	}()

	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(fpath))
	http.ServeFile(w, r, fpath)
}

func transcodeWebmToMOV(file multipart.File, name string) (string, error) {
	dir, err := os.MkdirTemp(videos_path, "*")
	if err != nil {
		return "", fmt.Errorf("error creating tempdir: %w", err)
	}
	fpath := filepath.Join(dir, strings.TrimSuffix(name, filepath.Ext(name))+".mp4")

	cmd := exec.Command("ffmpeg",
		"-vcodec", "libvpx-vp9",
		"-i", "pipe:0",
		"-c:v", "libx265",
		"-tag:v", "hvc1",
		fpath,
	)
	cmd.Stderr = os.Stderr
	cmd.Stdin = file
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error running ffmpeg command: %w", err)
	}

	return fpath, nil
}

type ErrorResponse struct {
	Error string
}

func writeFailedResponse(w http.ResponseWriter, statusCode int, errorMessage string) {
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: errorMessage}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("error writing response", "error", err)
	}
}
