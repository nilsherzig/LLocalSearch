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
	if strings.Contains(stderr.String(), "cli-response") {
		t.Fatalf("logs should not contain html content, got %q", stderr.String())
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
