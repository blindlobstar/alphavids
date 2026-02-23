export REGISTRY := ghcr.io/blindlobstar

static/styles.css:
	npx tailwindcss -i static/styles.dev.css -o static/styles.css

.PHONY: deploy
deploy:
	cicdez deploy -f docker-compose.deploy.yaml

.PHONY: build-ffmpeg
build-ffmpeg:
	cicdez build --push -f docker-compose.ffmpeg.yaml 
