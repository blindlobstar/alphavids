# Alphavids

Transparent WEBM to MP4 converter

## Setup

* Set `SECRET_FILE` env with path to sops secrets
* Set `SOPS_AGE_KEY_FILE` env with path to AGE key
* Login to `ghcr.io` docker registry
* Add `sandbox` docker context

## Debug

```
docker build -t alphavids . --platform=linux/amd64
docker run --platform=linux/amd64 -p 8080:8080 alphavids
```
