package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFetchesTargetsFromConfig(t *testing.T) {
	t.Parallel()

	embedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"embeddings":[[` + strings.Repeat("0,", 4095) + `1]]}`))
	}))
	defer embedServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><article><h1>CLI</h1><p>cli-response</p></article></body></html>`))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configData := []byte("cache_dir: " + filepath.Join(tempDir, "cache") + "\nembeddings:\n  base_url: " + embedServer.URL + "\n  model: qwen3-embedding\n  dimensions: 4096\nwebsites:\n  - " + server.URL + "\n")
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"-config", configPath}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if got := stdout.String(); got != "" {
		t.Fatalf("unexpected stdout %q", got)
	}
	if !strings.Contains(stderr.String(), "scraping configured targets") {
		t.Fatalf("expected slog action logs in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "saved page") {
		t.Fatalf("expected saved page logs in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "pages_per_minute=") {
		t.Fatalf("expected pages per minute status in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "median_embedding_time=") {
		t.Fatalf("expected median embedding time status in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "pages_scraped_in_session=") {
		t.Fatalf("expected pages scraped in session status in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "failed_requests_per_minute=") {
		t.Fatalf("expected failed requests per minute status in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "failed_requests_last_minute=") {
		t.Fatalf("expected failed requests last minute status in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "session_elapsed=") {
		t.Fatalf("expected session elapsed status in stderr, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "cli-response") {
		t.Fatalf("logs should not contain html content, got %q", stderr.String())
	}
}

func TestRunTracksFailedRequestMetrics(t *testing.T) {
	t.Parallel()

	embedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"embeddings":[[` + strings.Repeat("0,", 4095) + `1]]}`))
	}))
	defer embedServer.Close()

	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><article><p>bad</p></article></body></html>`))
	}))
	badURL := badServer.URL
	badServer.Close()

	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><article><h1>CLI</h1><p>cli-response</p></article></body></html>`))
	}))
	defer goodServer.Close()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configData := []byte("cache_dir: " + filepath.Join(tempDir, "cache") + "\nembeddings:\n  base_url: " + embedServer.URL + "\n  model: qwen3-embedding\n  dimensions: 4096\nwebsites:\n  - " + badURL + "\n  - " + goodServer.URL + "\n")
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"-config", configPath}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected crawl error")
	}

	if !strings.Contains(stderr.String(), "failed_requests_per_minute=") {
		t.Fatalf("expected failed requests per minute status in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "failed_requests_last_minute=") {
		t.Fatalf("expected failed requests last minute status in stderr, got %q", stderr.String())
	}
}

func TestRunRejectsPositionalArguments(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"https://example.com"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected positional argument error")
	}

	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("expected usage in stderr, got %q", stderr.String())
	}
}
