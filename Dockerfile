#build css
FROM node:22.2-alpine AS npm-build
ENV NODE_ENV=production
WORKDIR /usr/src/app

COPY package-lock.json .
COPY package.json .

COPY tailwind.config.js .
COPY templates/*.html templates/
COPY static/* static/

RUN npm install --production=false

RUN npx tailwindcss -i static/styles.dev.css -o static/styles.css --minify

#build stage
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /go/src/app
COPY go.mod go.mod
COPY go.sum go.sum
COPY *.go .
COPY --from=npm-build /usr/src/app/static ./static
RUN go get -d -v ./...
RUN go build -o /go/bin/app 

#final stage
FROM alpine:latest
RUN apk --no-cache add ffmpeg
COPY templates /templates
COPY --from=builder /go/bin/app /app
COPY --from=npm-build /usr/src/app/static /static
ENTRYPOINT ["/app"]
LABEL Name=alphavids Version=0.0.1
EXPOSE 8080
