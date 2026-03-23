CONFIG ?= scraper.example.yaml
ADDR ?= :8080
TEMPLATES_DIR ?= ./frontend/templates

.PHONY: scrape embed web start-ollama

scrape:
	go run ./cmd/llocalsearch scrape -config $(CONFIG)

embed:
	go run ./cmd/llocalsearch embed -config $(CONFIG)

web:
	go run ./cmd/llocalsearch web -config $(CONFIG) -addr $(ADDR) -templates-dir $(TEMPLATES_DIR)

start-ollama:
	docker run -d -e OLLAMA_CONTEXT_LENGTH=20000 -e HSA_OVERRIDE_GFX_VERSION="10.3.0" --device /dev/kfd --device /dev/dri -v ollama:/root/.ollama -p 11434:11434 --name ollama ollama/ollama:rocm
