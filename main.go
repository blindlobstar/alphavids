package main

import (
	"html/template"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	videos_path = "videos"
)

var templates *template.Template
var notFoundFile, notFoundErr = http.Dir("dummy").Open("does-not-exist")

type noDirFS struct {
	http.Dir
}

func (m noDirFS) Open(name string) (result http.File, err error) {
	f, err := m.Dir.Open(name)
	if err != nil {
		return
	}

	fi, err := f.Stat()
	if err != nil {
		return
	}
	if fi.IsDir() {
		return notFoundFile, notFoundErr
	}
	return f, nil
}

func main() {
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

	var err error
	templates, err = template.ParseGlob("templates/*.html")
	if err != nil {
		slog.Error("error parsing templates", "error", err)
		return
	}

	http.HandleFunc("POST /transcode", transcodeHandler)
	http.Handle("GET /videos/", http.StripPrefix("/videos/", http.FileServer(noDirFS{http.Dir(videos_path)})))
	http.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(noDirFS{http.Dir("./static")})))
	http.HandleFunc("GET /", indexHandler)
	http.HandleFunc("GET /upload-form", uploadFormHandler)
	slog.Info("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func deleteOldFiles(path string) error {
	afterDate := time.Now().Add(-10 * time.Minute)
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

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if err := templates.ExecuteTemplate(w, "index", nil); err != nil {
		slog.Error("error executing template", "error", err)
	}
}

func uploadFormHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if err := templates.ExecuteTemplate(w, "form", nil); err != nil {
		slog.Error("error executing template", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func transcodeHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		slog.Error("error parsing multipart form", "error", err)
		writeResponse(w, http.StatusBadRequest, "File is too large, max size if 20mb")
		return
	}

	file, fileHeader, err := r.FormFile("upload")
	if err != nil {
		slog.Error("error reading file from form", "error", err)
		writeResponse(w, http.StatusBadRequest, "No file attached")
		return
	}
	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		slog.Error("error reading file into buffer", "error", err)
		writeResponse(w, http.StatusInternalServerError, "Something went wrong. Please try again later")
		return
	}

	dir, err := os.MkdirTemp(videos_path, "*")
	if err != nil {
		slog.Error("error creating tempdir", "error", err)
		writeResponse(w, http.StatusInternalServerError, "Something went wrong. Please try again later")
		return
	}
	fpath := path.Join(dir, strings.TrimSuffix(fileHeader.Filename, filepath.Ext(fileHeader.Filename))+".mov")

	cmd := exec.Command("ffmpeg",
		"-vcodec", "libvpx-vp9",
		"-i", "pipe:0",
		"-vf", "format=rgba",
		"-c:v", "prores_ks",
		"-pix_fmt", "yuva444p10le",
		"-alpha_bits", "16",
		"-profile:v", "4444",
		"-f", "mov",
		"-vframes", "150",
		"-movflags", "frag_keyframe",
		"-flush_packets", "0",
		fpath,
	)

	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		slog.Error("error creating stdin pipe", "error", err)
		writeResponse(w, http.StatusInternalServerError, "Something went wrong. Please try again later")
		return
	}

	if err := cmd.Start(); err != nil {
		slog.Error("error starting command", "error", err)
		writeResponse(w, http.StatusInternalServerError, "Something went wrong. Please try again later")
		return
	}

	if _, err := stdin.Write(buf); err != nil {
		slog.Error("error writing buffer into stdin pipe", "error", err)
		writeResponse(w, http.StatusInternalServerError, "Something went wrong. Please try again later")
		return
	}

	if err := stdin.Close(); err != nil {
		slog.Error("error closing stdin pipe", "error", err)
		writeResponse(w, http.StatusInternalServerError, "Something went wrong. Please try again later")
		return
	}

	if err := cmd.Wait(); err != nil {
		slog.Error("error waiting for command to exit", "error", err)
		writeResponse(w, http.StatusInternalServerError, "Something went wrong. Please try again later")
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := templates.ExecuteTemplate(w, "video-ready", fpath); err != nil {
		slog.Error("error executing template", "error", err)
	}
}

type Form struct {
	ErrorMessage string
}

func writeResponse(w http.ResponseWriter, statusCode int, errorMessage string) {
	var data *Form
	if len(errorMessage) != 0 {
		w.WriteHeader(statusCode)
		data = &Form{ErrorMessage: errorMessage}
	}
	if err := templates.ExecuteTemplate(w, "form", data); err != nil {
		slog.Error("error executing template", "error", err)
	}
}
