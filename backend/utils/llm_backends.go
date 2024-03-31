package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/ollama/ollama/api"
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
	return ollama.New(ollama.WithModel(modelName), ollama.WithServerURL(os.Getenv("OLLAMA_HOST")), ollama.WithRunnerNumCtx(10000))
}

func GetSessionString() string {
	return uuid.New().String()
}

func CheckIfModelExistsOrPull(modelName string) error {
	if err := CheckIfModelExists(modelName); err != nil {
		slog.Warn("Model does not exist, pulling it", "model", modelName)
		if err := OllamaPullModel(modelName); err != nil {
			return err
		}
	}
	return nil
}

func CheckIfModelExists(modelName string) error {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return err
	}
	models, err := client.List(context.Background())
	if err != nil {
		return err
	}
	for _, model := range models.Models {
		if model.Name == modelName {
			return nil
		}
	}
	return fmt.Errorf("Model %s does not exist", modelName)
}

func OllamaPullModel(modelName string) error {
	pullReq := api.PullRequest{
		Model:    modelName,
		Insecure: false,
		Name:     modelName,
	}
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return err
	}
	return client.Pull(context.Background(), &pullReq, pullProgressHandler)
}

var lastProgress string

func pullProgressHandler(progress api.ProgressResponse) error {
	percentage := progressPercentage(progress)
	if percentage != lastProgress {
		slog.Info("Pulling model", "progress", percentage)
		lastProgress = percentage
	}
	return nil
}

func progressPercentage(progress api.ProgressResponse) string {
	return fmt.Sprintf("%d", (progress.Completed*100)/(progress.Total+1))
}
