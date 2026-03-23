package embeddingjob

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nilsherzig/llocalsearch/scraper"
	"github.com/nilsherzig/llocalsearch/vectorstore"
)

func TestJobEmbedsOnlyPagesMissingEmbeddings(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pages.db")

	db, err := vectorstore.Open(dbPath)
	if err != nil {
		t.Fatalf("open vector store: %v", err)
	}
	if err := db.AutoMigrate(&scraper.SavedPage{}); err != nil {
		t.Fatalf("migrate saved pages: %v", err)
	}
	if err := vectorstore.EnsureSchema(db, "qwen3-embedding", 3); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	pages := []scraper.SavedPage{
		{URL: "https://example.com/one", Host: "example.com", Path: "/one", PageKey: "one", ContentHash: "hash-one", ParseMode: "readability", Content: "<article><p>one</p></article>", ScrapedAt: time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC)},
		{URL: "https://example.com/two", Host: "example.com", Path: "/two", PageKey: "two", ContentHash: "hash-two", ParseMode: "readability", Content: "<article><p>two</p></article>", ScrapedAt: time.Date(2026, 3, 23, 10, 1, 0, 0, time.UTC)},
	}
	for i := range pages {
		if err := db.Create(&pages[i]).Error; err != nil {
			t.Fatalf("create page %d: %v", i, err)
		}
	}
	if err := vectorstore.UpsertEmbedding(db, pages[0].ID, []float32{1, 0, 0}); err != nil {
		t.Fatalf("upsert existing embedding: %v", err)
	}

	embedder := &recordingEmbedder{
		vectors: [][]float32{{0, 1, 0}},
	}
	job, err := New(Config{
		DBPath:     dbPath,
		BatchSize:  2,
		Dimensions: 3,
		Model:      "qwen3-embedding",
		Embedder:   embedder,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	result, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.TotalMissing != 1 {
		t.Fatalf("expected 1 missing page, got %d", result.TotalMissing)
	}
	if result.EmbeddedPages != 1 {
		t.Fatalf("expected 1 embedded page, got %d", result.EmbeddedPages)
	}
	if got := embedder.calls; len(got) != 1 || len(got[0]) != 1 || got[0][0] != "two" {
		t.Fatalf("unexpected embedder calls: %v", got)
	}

	matches, err := vectorstore.Search(db, []float32{0, 1, 0}, 5)
	if err != nil {
		t.Fatalf("search vectors: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected stored embeddings after job run")
	}
}

func TestJobTruncatesPageContentToConfiguredTokenLimit(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pages.db")

	db, err := vectorstore.Open(dbPath)
	if err != nil {
		t.Fatalf("open vector store: %v", err)
	}
	if err := db.AutoMigrate(&scraper.SavedPage{}); err != nil {
		t.Fatalf("migrate saved pages: %v", err)
	}
	if err := vectorstore.EnsureSchema(db, "qwen3-embedding", 3); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	page := scraper.SavedPage{
		URL:         "https://example.com/limited",
		Host:        "example.com",
		Path:        "/limited",
		PageKey:     "limited",
		ContentHash: "hash-limited",
		ParseMode:   "readability",
		Content:     "<article><h1>one two</h1><p>three four five</p></article>",
		ScrapedAt:   time.Date(2026, 3, 23, 10, 2, 0, 0, time.UTC),
	}
	if err := db.Create(&page).Error; err != nil {
		t.Fatalf("create page: %v", err)
	}

	embedder := &recordingEmbedder{
		vectors: [][]float32{{0, 1, 0}},
	}
	job, err := New(Config{
		DBPath:         dbPath,
		BatchSize:      1,
		Dimensions:     3,
		Model:          "qwen3-embedding",
		PageTokenLimit: 3,
		Embedder:       embedder,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if got := embedder.calls; len(got) != 1 || len(got[0]) != 1 || got[0][0] != "one two three" {
		t.Fatalf("unexpected embedder calls: %v", got)
	}
}

func TestJobLogsProgressAndTiming(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pages.db")

	db, err := vectorstore.Open(dbPath)
	if err != nil {
		t.Fatalf("open vector store: %v", err)
	}
	if err := db.AutoMigrate(&scraper.SavedPage{}); err != nil {
		t.Fatalf("migrate saved pages: %v", err)
	}
	if err := vectorstore.EnsureSchema(db, "qwen3-embedding", 3); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	for i, content := range []string{"<article><p>one</p></article>", "<article><p>two</p></article>"} {
		page := scraper.SavedPage{
			URL:         fmt.Sprintf("https://example.com/%d", i),
			Host:        "example.com",
			Path:        fmt.Sprintf("/%d", i),
			PageKey:     fmt.Sprintf("key-%d", i),
			ContentHash: fmt.Sprintf("hash-%d", i),
			ParseMode:   "readability",
			Content:     content,
			ScrapedAt:   time.Date(2026, 3, 23, 10, i, 0, 0, time.UTC),
		}
		if err := db.Create(&page).Error; err != nil {
			t.Fatalf("create page %d: %v", i, err)
		}
	}

	var logOutput bytes.Buffer
	job, err := New(Config{
		DBPath:     dbPath,
		BatchSize:  1,
		Dimensions: 3,
		Model:      "qwen3-embedding",
		Embedder: &recordingEmbedder{
			vectors: [][]float32{{1, 0, 0}, {0, 1, 0}},
		},
		Logger: slog.New(slog.NewTextHandler(&logOutput, nil)),
		Now: func() time.Time {
			return time.Date(2026, 3, 23, 10, 5, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	logs := logOutput.String()
	if !strings.Contains(logs, `msg="embedding job started"`) {
		t.Fatalf("expected start log, got %q", logs)
	}
	if !strings.Contains(logs, `embedding_progress=`) {
		t.Fatalf("expected progress field, got %q", logs)
	}
	if !strings.Contains(logs, `batch_duration=`) {
		t.Fatalf("expected batch duration field, got %q", logs)
	}
	if !strings.Contains(logs, `msg="embedding job finished"`) {
		t.Fatalf("expected finish log, got %q", logs)
	}
}

type recordingEmbedder struct {
	vectors [][]float32
	calls   [][]string
}

func (r *recordingEmbedder) Embed(_ context.Context, inputs []string) ([][]float32, error) {
	r.calls = append(r.calls, append([]string(nil), inputs...))
	if len(r.vectors) < len(inputs) {
		return nil, fmt.Errorf("not enough vectors")
	}
	vectors := append([][]float32(nil), r.vectors[:len(inputs)]...)
	r.vectors = r.vectors[len(inputs):]
	return vectors, nil
}
