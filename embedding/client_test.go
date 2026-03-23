package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientEmbedSendsConfiguredDimensionsAndInputs(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotRequest embedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embeddings":[[1,0.5],[0.1,0.9]]}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:    server.URL,
		Model:      "qwen3-embedding",
		Dimensions: 4096,
		Timeout:    time.Second,
	}, server.Client())

	vectors, err := client.Embed(context.Background(), []string{"First sentence", "Second sentence"})
	if err != nil {
		t.Fatalf("Embed returned error: %v", err)
	}

	if gotPath != "/api/embed" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
	if gotRequest.Model != "qwen3-embedding" {
		t.Fatalf("unexpected model %q", gotRequest.Model)
	}
	if gotRequest.Dimensions != 4096 {
		t.Fatalf("unexpected dimensions %d", gotRequest.Dimensions)
	}
	if len(gotRequest.Input) != 2 || gotRequest.Input[0] != "First sentence" || gotRequest.Input[1] != "Second sentence" {
		t.Fatalf("unexpected inputs %#v", gotRequest.Input)
	}
	if len(vectors) != 2 || len(vectors[0]) != 2 || len(vectors[1]) != 2 {
		t.Fatalf("unexpected vectors %#v", vectors)
	}
}

func TestClientEmbedReturnsErrorOnNonOKResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "embedding offline", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:    server.URL,
		Model:      "qwen3-embedding",
		Dimensions: 4096,
		Timeout:    time.Second,
	}, server.Client())

	_, err := client.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("expected status in error, got %v", err)
	}
}
