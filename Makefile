#TODO use this as a version
GIT_HASH := $(shell git rev-parse --short HEAD) 

LATEST_TAG := $(shell git describe --tags --abbrev=0)
CURRENT_TIMESTAMP := $(shell date +%s)

# used for local testing, so i can save the platform build time
PHONY: build-containers
build-containers:
	(cd ./metrics/ && docker buildx build --build-arg="VERSION=$(CURRENT_TIMESTAMP)" . -t nilsherzig/lsm:latest --load)
	docker buildx build --build-arg="PUBLIC_VERSION=$(CURRENT_TIMESTAMP)" . -t nilsherzig/llocalsearch-frontend:latest --load
	(cd ./backend/ && docker buildx build . -t nilsherzig/llocalsearch-backend:latest --load)

# containers which will be published
PHONY: build-containers-multi
build-containers-multi:
	(cd ./metrics/ && docker buildx build --build-arg="VERSION=$(CURRENT_TIMESTAMP)" . -t nilsherzig/lsm:latest --push --platform linux/amd64,linux/arm64)
	docker buildx build --build-arg="PUBLIC_VERSION=$(CURRENT_TIMESTAMP)" . -t nilsherzig/llocalsearch-frontend:latest --push --platform linux/amd64,linux/arm64
	(cd ./backend/ && docker buildx build . -t nilsherzig/llocalsearch-backend:latest --push --platform linux/amd64,linux/arm64)

PHONY: new-release
new-release: build-containers-multi 
	@echo "New release pushed to Docker Hub"

PHONY: e2e-backend
e2e-backend:
	(cd ./backend && ginkgo -v -r ./...)

# dev run commands
PHONY: build-dev
build-dev:
	docker-compose -f ./docker-compose.dev.yaml build
	
PHONY: dev
dev: build-dev
	docker-compose -f ./docker-compose.dev.yaml up $(ARGS)

PHONY: dev-bg
dev-bg: build-dev
	docker-compose -f ./docker-compose.dev.yaml up -d
