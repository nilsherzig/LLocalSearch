.PHONY: start-ollama
start-ollama:
	docker run -d -e OLLAMA_CONTEXT_LENGTH=20000 -e HSA_OVERRIDE_GFX_VERSION="10.3.0" --device /dev/kfd --device /dev/dri -v ollama:/root/.ollama -p 11434:11434 --name ollama ollama/ollama:rocm
