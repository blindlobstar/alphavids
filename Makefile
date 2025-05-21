REGISTRY:= ghcr.io
IMAGE:= blindlobstar/alphavids

static/styles.css:
	npx tailwindcss -i static/styles.dev.css -o static/styles.css

.PHONY: build
build:
	docker build . -t $(REGISTRY)/$(IMAGE) --platform=linux/amd64
	docker push $(REGISTRY)/$(IMAGE)

.PHONY: build-ffmpeg
build-ffmpeg:
	docker build . -f Dockerfile.ffmpeg -t $(REGISTRY)/$(IMAGE)/ffmpeg --platform=linux/amd64
	docker push $(REGISTRY)/$(IMAGE)/ffmpeg

.PHONY: deploy
deploy:
	REGISTRY=$(REGISTRY) \
	IMAGE=$(IMAGE) \
	sops exec-env $(SECRET_FILE) \
	'docker --context sandbox stack deploy -c docker-compose.deploy.yaml alphavids --with-registry-auth'
