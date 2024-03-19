PHONY: build-dev
build-dev:
	docker-compose -f ./docker-compose.dev.yaml build

PHONY: dev
dev: build-dev
	docker-compose -f ./docker-compose.dev.yaml up

PHONY: run
run: build-dev
	docker-compose -f ./docker-compose.dev.yaml up -d

PHONY: stop
stop: 
	docker-compose -f ./docker-compose.dev.yaml down 

PHONY: update
update:  
	git pull

PHONY: upgrade
upgrade: stop update build-dev run
