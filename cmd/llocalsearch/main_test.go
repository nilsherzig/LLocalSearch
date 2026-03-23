package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunScrapeFetchesTargetsFromConfigWithoutEmbeddings(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><article><h1>CLI</h1><p>cli-response</p></article></body></html>`))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configData := []byte("cache_dir: " + filepath.Join(tempDir, "cache") + "\nwebsites:\n  - " + server.URL + "\n")
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"scrape", "-config", configPath}, &stdout, &stderr)
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

func TestRunEmbedBackfillsMissingEmbeddings(t *testing.T) {
	t.Parallel()

	var gotInput []string
	embedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode embed request: %v", err)
		}
		gotInput = append([]string(nil), request.Input...)
		_, _ = w.Write([]byte(`{"embeddings":[[` + strings.Repeat("0,", 4095) + `1]]}`))
	}))
	defer embedServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><article><h1>one two</h1><p>three four</p></article></body></html>`))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configData := []byte("cache_dir: " + filepath.Join(tempDir, "cache") + "\nembeddings:\n  base_url: " + embedServer.URL + "\n  model: qwen3-embedding\n  dimensions: 4096\n  page_token_limit: 2\nwebsites:\n  - " + server.URL + "\n")
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := run([]string{"scrape", "-config", configPath}, &stdout, &stderr); err != nil {
		t.Fatalf("scrape run returned error: %v", err)
	}

	stderr.Reset()
	if err := run([]string{"embed", "-config", configPath}, &stdout, &stderr); err != nil {
		t.Fatalf("embed run returned error: %v", err)
	}

	if !strings.Contains(stderr.String(), "embedding_progress=") {
		t.Fatalf("expected embedding progress status in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "embedded_pages=") {
		t.Fatalf("expected embedded pages status in stderr, got %q", stderr.String())
	}
	if len(gotInput) != 1 || gotInput[0] != "one two" {
		t.Fatalf("expected capped embed input, got %v", gotInput)
	}
}

func TestRunWebStartsFrontendServer(t *testing.T) {
	t.Parallel()

	embedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"embeddings":[[` + strings.Repeat("0,", 4095) + `1]]}`))
	}))
	defer embedServer.Close()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configData := []byte("cache_dir: " + filepath.Join(tempDir, "cache") + "\nembeddings:\n  base_url: " + embedServer.URL + "\n  model: qwen3-embedding\n  dimensions: 4096\nwebsites:\n  - https://example.com\n")
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}
	templatesDir := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(testFile))), "frontend", "templates")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var called bool
	var gotAddr string

	err := run([]string{"web", "-config", configPath, "-addr", ":9090", "-templates-dir", templatesDir}, &stdout, &stderr, func(addr string, handler http.Handler) error {
		called = true
		gotAddr = addr
		if handler == nil {
			t.Fatal("expected non-nil handler")
		}
		req := httptest.NewRequest(http.MethodGet, "/search?q=test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected search handler to work, got status %d and body %q", rec.Code, rec.Body.String())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !called {
		t.Fatal("expected listen function to be called")
	}
	if gotAddr != ":9090" {
		t.Fatalf("unexpected addr %q", gotAddr)
	}
	if !strings.Contains(stderr.String(), "starting frontend server") {
		t.Fatalf("expected startup log in stderr, got %q", stderr.String())
	}
}

func TestRunScrapeTracksFailedRequestMetrics(t *testing.T) {
	t.Parallel()

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
	configData := []byte("cache_dir: " + filepath.Join(tempDir, "cache") + "\nwebsites:\n  - " + badURL + "\n  - " + goodServer.URL + "\n")
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"scrape", "-config", configPath}, &stdout, &stderr)
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

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"https://example.com"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected subcommand error")
	}

	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("expected usage in stderr, got %q", stderr.String())
	}
}
