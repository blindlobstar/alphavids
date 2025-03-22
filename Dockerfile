ARG FFMPEG_IMAGE="ghcr.io/blindlobstar/alphavids/ffmpeg"
ARG FFMPEG_TAG="latest"

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
FROM ${FFMPEG_IMAGE}:${FFMPEG_TAG}

COPY --from=npm-build /usr/src/app/static /static
COPY --from=builder /go/bin/app /app

ENTRYPOINT ["/app"]
LABEL Name=alphavids Version=0.0.1
EXPOSE 8080
