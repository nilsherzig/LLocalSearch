DOCKER_HUB_USER ?= nilsherzig
BACKEND_NAME ?= llocalsearch-backend
FRONTEND_NAME ?= llocalsearch-frontend
GIT_HASH := $(shell git rev-parse --short HEAD)

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
