package summary

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientSummarizeSendsConfiguredModelAndSearchContext(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotRequest generateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"  Concise result summary.  "}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Model:   "qwen3.5:9b",
		Timeout: time.Second,
	}, server.Client())

	text, err := client.Summarize(context.Background(), "kubernetes guide", []Result{
		{
			URL:        "https://example.com/article",
			Host:       "example.com",
			Path:       "/article",
			Excerpt:    "clean article body",
			Content:    "clean article body with more detailed plain text context",
			Similarity: 0.98,
		},
	})
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}

	if gotPath != "/api/generate" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
	if gotRequest.Model != "qwen3.5:9b" {
		t.Fatalf("unexpected model %q", gotRequest.Model)
	}
	if gotRequest.Stream {
		t.Fatal("expected non-streaming summary request")
	}
	if !strings.Contains(gotRequest.Prompt, "kubernetes guide") {
		t.Fatalf("expected query in prompt, got %q", gotRequest.Prompt)
	}
	if !strings.Contains(gotRequest.Prompt, "https://example.com/article") {
		t.Fatalf("expected url in prompt, got %q", gotRequest.Prompt)
	}
	if !strings.Contains(gotRequest.Prompt, "clean article body") {
		t.Fatalf("expected excerpt in prompt, got %q", gotRequest.Prompt)
	}
	if !strings.Contains(gotRequest.Prompt, "more detailed plain text context") {
		t.Fatalf("expected content context in prompt, got %q", gotRequest.Prompt)
	}
	if text != "Concise result summary." {
		t.Fatalf("unexpected summary text %q", text)
	}
}

func TestClientSummarizeReturnsErrorOnNonOKResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "summary offline", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Model:   "qwen3.5:9b",
		Timeout: time.Second,
	}, server.Client())

	_, err := client.Summarize(context.Background(), "test", []Result{{URL: "https://example.com"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("expected status in error, got %v", err)
	}
}
