package utils

import (
	"os"

	"github.com/google/uuid"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

func NewGPT35() (*openai.LLM, error) {
	return openai.New(openai.WithModel("gpt-3.5-turbo-0125"))
}

func NewGPT4() (*openai.LLM, error) {
	return openai.New(openai.WithModel("gpt-4-1106-preview"))
}

func NewEmbeddingsLLM() (*ollama.LLM, error) {
	modelName := "all-minilm"
	return newOllama(modelName)
}

func NewOllama() (*ollama.LLM, error) {
	// modelName := "search:latest"
	// modelName := "mistral:latest"
	modelName := os.Getenv("OLLAMA_MODEL_NAME")
	return newOllama(modelName)
}

func newOllama(modelName string) (*ollama.LLM, error) {
	return ollama.New(ollama.WithModel(modelName), ollama.WithServerURL(os.Getenv("OLLAMA_URL")), ollama.WithRunnerNumCtx(10000))
}

func GetSessionString() string {
	return uuid.New().String()
}
