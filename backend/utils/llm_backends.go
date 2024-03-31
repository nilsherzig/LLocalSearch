package utils

import (
	"os"

	"github.com/google/uuid"
	"github.com/tmc/langchaingo/llms/ollama"
)

func NewOllamaEmbeddingLLM() (*ollama.LLM, error) {
	modelName := "all-minilm"
	return newOllama(modelName)
}

func NewOllamaLLM() (*ollama.LLM, error) {
	modelName := os.Getenv("OLLAMA_MODEL_NAME")
	return newOllama(modelName)
}

func newOllama(modelName string) (*ollama.LLM, error) {
	return ollama.New(ollama.WithModel(modelName), ollama.WithServerURL(os.Getenv("OLLAMA_URL")), ollama.WithRunnerNumCtx(10000))
}

func GetSessionString() string {
	return uuid.New().String()
}
