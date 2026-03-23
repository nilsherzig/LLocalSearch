package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunStartsFrontendServer(t *testing.T) {
	tempDir := t.TempDir()

	embedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"embeddings":[[` + strings.Repeat("0,", 4095) + `1]]}`))
	}))
	defer embedServer.Close()

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

	var stderr bytes.Buffer
	var called bool
	var gotAddr string

	err := run([]string{"-config", configPath, "-addr", ":9090", "-templates-dir", templatesDir}, &stderr, func(addr string, handler http.Handler) error {
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

func TestRunRejectsPositionalArguments(t *testing.T) {
	var stderr bytes.Buffer

	err := run([]string{"unexpected"}, &stderr, func(addr string, handler http.Handler) error {
		t.Fatal("listen function should not be called")
		return nil
	})
	if err == nil {
		t.Fatal("expected positional argument error")
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("expected usage in stderr, got %q", stderr.String())
	}
}
