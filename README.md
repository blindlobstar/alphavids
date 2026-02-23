# Alphavids

Transparent WEBM to MP4 converter

## Setup

* Install [cicdez](https://github.com/blindlobstar/cicdez)
* Initialize submodules: `git submodule update --init`
* Add your age key to `~/.config/cicdez/age.key`

## Deploy

```
make deploy
```

## Build and push ffmpeg image

```
make build-ffmpeg
```

## Debug

```
docker build -t alphavids . --platform=linux/amd64
docker run --platform=linux/amd64 -p 8080:8080 alphavids
```
