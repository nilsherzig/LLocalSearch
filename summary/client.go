package summary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Result struct {
	URL        string
	Host       string
	Path       string
	Excerpt    string
	Content    string
	Similarity float64
}

type Response struct {
	Text  string
	Stats Stats
}

type Stats struct {
	PromptEvalCount int
	EvalCount       int
	EvalDuration    time.Duration
	TotalDuration   time.Duration
}

func (s Stats) TokensPerSecond() float64 {
	if s.EvalCount <= 0 || s.EvalDuration <= 0 {
		return 0
	}
	return float64(s.EvalCount) / s.EvalDuration.Seconds()
}

type Config struct {
	BaseURL string
	Model   string
	Timeout time.Duration
}

type Client interface {
	Summarize(ctx context.Context, query string, results []Result) (Response, error)
}

type HTTPClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response        string `json:"response"`
	TotalDuration   int64  `json:"total_duration"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
	EvalDuration    int64  `json:"eval_duration"`
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
		httpClient: httpClient,
	}
}

func (c *HTTPClient) Summarize(ctx context.Context, query string, results []Result) (Response, error) {
	if strings.TrimSpace(query) == "" || len(results) == 0 {
		return Response{}, nil
	}

	payload, err := json.Marshal(generateRequest{
		Model:  c.model,
		Prompt: buildPrompt(query, results),
		Stream: false,
	})
	if err != nil {
		return Response{}, fmt.Errorf("marshal summary request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("build summary request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("execute summary request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("summary request failed with status %d", resp.StatusCode)
	}

	var body generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Response{}, fmt.Errorf("decode summary response: %w", err)
	}

	return Response{
		Text: strings.TrimSpace(body.Response),
		Stats: Stats{
			PromptEvalCount: body.PromptEvalCount,
			EvalCount:       body.EvalCount,
			EvalDuration:    time.Duration(body.EvalDuration),
			TotalDuration:   time.Duration(body.TotalDuration),
		},
	}, nil
}

func buildPrompt(query string, results []Result) string {
	var prompt strings.Builder
	prompt.WriteString("You summarize local search results for a web UI.\n")
	prompt.WriteString("Write a concise summary in plain text with no markdown bullets.\n")
	prompt.WriteString("Focus on the main themes and the most relevant pages.\n\n")
	prompt.WriteString("Query: ")
	prompt.WriteString(query)
	prompt.WriteString("\n\nResults:\n")

	for i, result := range results {
		fmt.Fprintf(&prompt, "%d. URL: %s\n", i+1, result.URL)
		if result.Host != "" || result.Path != "" {
			fmt.Fprintf(&prompt, "Host: %s Path: %s\n", result.Host, result.Path)
		}
		fmt.Fprintf(&prompt, "Similarity: %.4f\n", result.Similarity)
		if result.Excerpt != "" {
			fmt.Fprintf(&prompt, "Excerpt: %s\n", result.Excerpt)
		}
		if result.Content != "" {
			fmt.Fprintf(&prompt, "Content: %s\n", truncateText(result.Content, 2000))
		}
		prompt.WriteString("\n")
	}

	return prompt.String()
}

func truncateText(text string, limit int) string {
	normalized := strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if limit <= 0 || len(normalized) <= limit {
		return normalized
	}
	return normalized[:limit] + "..."
}
