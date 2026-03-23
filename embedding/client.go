package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	BaseURL    string
	Model      string
	Dimensions int
	Timeout    time.Duration
}

type Client interface {
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

type HTTPClient struct {
	baseURL    string
	model      string
	dimensions int
	httpClient *http.Client
}

type embedRequest struct {
	Model      string   `json:"model"`
	Dimensions int      `json:"dimensions,omitempty"`
	Input      []string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func NewClient(cfg Config, httpClient *http.Client) *HTTPClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	return &HTTPClient{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
		httpClient: httpClient,
	}
}

func (c *HTTPClient) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	payload, err := json.Marshal(embedRequest{
		Model:      c.model,
		Dimensions: c.dimensions,
		Input:      inputs,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed request failed with status %d", resp.StatusCode)
	}

	var body embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	if len(body.Embeddings) != len(inputs) {
		return nil, fmt.Errorf("embed response count mismatch: got %d want %d", len(body.Embeddings), len(inputs))
	}

	return body.Embeddings, nil
}
