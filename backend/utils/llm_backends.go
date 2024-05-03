package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/ollama/ollama/api"
	// "github.com/tmc/langchaingo/httputil"
	"github.com/tmc/langchaingo/llms/ollama"
)

var EmbeddingsModel = os.Getenv("EMBEDDINGS_MODEL_NAME")

func NewOllamaEmbeddingLLM() (*ollama.LLM, error) {
	modelName := EmbeddingsModel
	return NewOllama(modelName, (1024 * 8))
}

func NewOllama(modelName string, contextSize int) (*ollama.LLM, error) {
	return ollama.New(ollama.WithModel(modelName),
		ollama.WithServerURL(os.Getenv("OLLAMA_HOST")),
		ollama.WithRunnerNumCtx(contextSize),
		// ollama.WithHTTPClient(httputil.DebugHTTPClient),
	)
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

func GetOllamaModelList() ([]string, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, err
	}
	models, err := client.List(context.Background())
	if err != nil {
		return nil, err
	}
	modelNames := make([]string, 0)
	for _, model := range models.Models {
		modelNames = append(modelNames, model.Name)
	}
	return modelNames, nil
}

func CheckIfModelExists(requestName string) error {
	modelNames, err := GetOllamaModelList()
	if err != nil {
		return err
	}
	for _, mn := range modelNames {
		if requestName == mn {
			return nil
		}
	}
	return fmt.Errorf("Model %s does not exist", requestName)
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
