DOCKER_HUB_USER ?= nilsherzig
BACKEND_NAME ?= llocalsearch-backend
FRONTEND_NAME ?= llocalsearch-frontend
GIT_HASH := $(shell git rev-parse --short HEAD)
LATEST_TAG := $(shell git describe --tags --abbrev=0)
CURRENT_TIMESTAMP := $(shell date +%s)

PHONY: build-container
build-container:
	docker buildx build --build-arg="PUBLIC_VERSION=$(CURRENT_TIMESTAMP)" . -t nilsherzig/llocalsearch-frontend:latest --load
	(cd ./backend/ && docker buildx build . -t nilsherzig/llocalsearch-backend:latest --load)

	(cd ./metrics/ && docker buildx build --build-arg="VERSION=$(CURRENT_TIMESTAMP)" . -t nilsherzig/lsm:latest --load)

PHONY: release-container
release-container: build-container
	docker push nilsherzig/llocalsearch-frontend:latest
	docker push nilsherzig/llocalsearch-backend:latest
	docker push nilsherzig/lsm:latest

PHONY: new-release
new-release: release-container
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

PHONY: dev-bg-stop
dev-bg-stop: 
	docker-compose -f ./docker-compose.dev.yaml down

# normal hosting commands
PHONY: run
run: 
	docker-compose up -d

PHONY: stop
stop: 
	docker-compose -f ./docker-compose.dev.yaml down 

PHONY: update
update:  
	git pull

PHONY: upgrade
upgrade: stop update run

# release / docker build commands
release-stable: build-stable tag-git-hash push
	@echo "Release stable version with git hash $(GIT_HASH)"

release-latest: build-latest tag-git-hash push
	@echo "Release latest version with git hash $(GIT_HASH)"

build-latest:
	docker build -t $(DOCKER_HUB_USER)/$(FRONTEND_NAME):latest .
	(cd ./backend && docker build -t $(DOCKER_HUB_USER)/$(BACKEND_NAME):latest .)

build-stable:
	docker build -t $(DOCKER_HUB_USER)/$(FRONTEND_NAME):stable .
	(cd ./backend && docker build -t $(DOCKER_HUB_USER)/$(BACKEND_NAME):stable .)

tag-git-hash:
	docker tag $(DOCKER_HUB_USER)/$(BACKEND_NAME):latest $(DOCKER_HUB_USER)/$(BACKEND_NAME):$(GIT_HASH)
	docker tag $(DOCKER_HUB_USER)/$(FRONTEND_NAME):latest $(DOCKER_HUB_USER)/$(FRONTEND_NAME):$(GIT_HASH)

push:
	@echo "Pushing images to Docker Hub"
	# docker push $(DOCKER_HUB_USER)$/$(BACKEND_NAME):$(GIT_HASH)
	# docker push $(DOCKER_HUB_USER)$/$(FRONTEND_NAME):$(GIT_HASH)
	docker push $(DOCKER_HUB_USER)/$(BACKEND_NAME):latest
	docker push $(DOCKER_HUB_USER)/$(FRONTEND_NAME):latest
	docker push $(DOCKER_HUB_USER)/$(BACKEND_NAME):stable
	docker push $(DOCKER_HUB_USER)/$(FRONTEND_NAME):stable

