FROM alpine:3.19 AS x265-builder

RUN apk add --no-cache \
    build-base \
    cmake \
    git \
    pkgconf \
    yasm \
    nasm

WORKDIR /src

RUN git clone https://bitbucket.org/multicoreware/x265_git.git x265
WORKDIR /src/x265/build

RUN cmake ../source \
    -DENABLE_SHARED=ON \
    -DENABLE_CLI=ON \
    -DENABLE_ALPHA=ON \
    -DCMAKE_SYSTEM_PROCESSOR=x86_64

RUN make && make install

FROM alpine:3.19 AS ffmpeg-builder

COPY --from=x265-builder /usr/local /usr/local

RUN apk add --no-cache \
    build-base \
    pkgconf \
    yasm \
    nasm \
    git \
    coreutils \
    zlib-dev \
    libvpx-dev


WORKDIR /src

RUN git clone https://git.ffmpeg.org/ffmpeg.git ffmpeg
WORKDIR /src/ffmpeg

RUN ./configure \
    --prefix=/usr/local \
    --enable-gpl \
    --enable-nonfree \
    --enable-libvpx \
    --enable-libx265 \
    --enable-small \
    --disable-debug \
    --disable-doc \
    --disable-ffplay \
    --disable-network \
    --disable-xlib \
    --disable-libxcb \
    --extra-ldflags="-L/usr/local/lib" \
    --extra-cflags="-I/usr/local/include" \
    && make \
    && make install

#build css
FROM node:22.1-alpine AS npm-build
ENV NODE_ENV=production
WORKDIR /usr/src/app

COPY package-lock.json .
COPY package.json .

COPY tailwind.config.js .
COPY static/* static/

RUN npm install --production=false

RUN npx tailwindcss -i static/styles.dev.css -o static/styles.css --minify

#build stage
FROM golang:1.24.1-alpine AS builder
RUN apk add --no-cache git
WORKDIR /go/src/app
COPY go.mod go.mod
COPY go.sum go.sum
COPY *.go .
RUN go get -d -v ./...
RUN go build -o /go/bin/app 

#final stage
FROM alpine:3.19

RUN apk --no-cache add libstdc++ libvpx

COPY --from=x265-builder /usr/local/bin/x265 /usr/local/bin/
COPY --from=x265-builder /usr/local/lib/libx265.so* /usr/local/lib/
COPY --from=ffmpeg-builder /usr/local/bin/ffmpeg /usr/local/bin/
COPY --from=ffmpeg-builder /usr/local/bin/ffprobe /usr/local/bin/

RUN ldconfig || true

COPY --from=npm-build /usr/src/app/static /static

COPY --from=builder /go/bin/app /app
ENTRYPOINT ["/app"]
LABEL Name=alphavids Version=0.0.1
EXPOSE 8080
