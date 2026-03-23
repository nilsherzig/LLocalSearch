package frontend

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/nilsherzig/llocalsearch/scraper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestIndexHandlerShowsDashboardMetrics(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "3 scraped pages") {
		t.Fatalf("expected scraped page count in dashboard, got %q", body)
	}
	if !strings.Contains(body, "https://example.com/article") {
		t.Fatalf("expected recent page url in dashboard, got %q", body)
	}
	if !strings.Contains(body, "example.com") || !strings.Contains(body, "2 pages") {
		t.Fatalf("expected host summary in dashboard, got %q", body)
	}
	if !strings.Contains(body, "news.example.org") || !strings.Contains(body, "1 pages") {
		t.Fatalf("expected second host summary in dashboard, got %q", body)
	}
	if !strings.Contains(body, "https://example.com") || !strings.Contains(body, "2 scraped pages") {
		t.Fatalf("expected whitelist seed summary for example.com, got %q", body)
	}
	if !strings.Contains(body, "https://news.example.org/start") || !strings.Contains(body, "1 scraped pages") {
		t.Fatalf("expected whitelist seed summary for news.example.org/start, got %q", body)
	}
	if !strings.Contains(body, `action="/search"`) {
		t.Fatalf("expected search form on dashboard, got %q", body)
	}
}

func TestPagesHandlerListsScrapedPages(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/pages", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "example.com/article") {
		t.Fatalf("expected first page in listing, got %q", body)
	}
	if !strings.Contains(body, "example.com/second") {
		t.Fatalf("expected second page in listing, got %q", body)
	}
}

func TestPageDetailHandlerShowsScrapedContent(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/pages/1", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "clean article body") {
		t.Fatalf("expected cleaned page content, got %q", body)
	}
	if !strings.Contains(body, "https://example.com/article") {
		t.Fatalf("expected page url in detail view, got %q", body)
	}
}

func TestPageDetailHandlerReturnsNotFoundForUnknownPage(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/pages/999", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status %d", rec.Code)
	}
}

func TestSearchHandlerFindsFuzzyMatchesAcrossDownloadedPages(t *testing.T) {
	server, dbPath := newTestServer(t)
	if err := insertTestEmbedding(dbPath, 1, vectorWithLead(1, 0, 0)); err != nil {
		t.Fatalf("insert first embedding: %v", err)
	}
	if err := insertTestEmbedding(dbPath, 2, vectorWithLead(0, 1, 0)); err != nil {
		t.Fatalf("insert second embedding: %v", err)
	}
	if err := insertTestEmbedding(dbPath, 3, vectorWithLead(0, 0, 1)); err != nil {
		t.Fatalf("insert third embedding: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/search?q=kubernetes+guide", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `Results for "kubernetes guide"`) {
		t.Fatalf("expected search heading, got %q", body)
	}
	if !strings.Contains(body, "https://example.com/article") {
		t.Fatalf("expected vector search to find article page, got %q", body)
	}
	if !strings.Contains(body, "clean article body") {
		t.Fatalf("expected search snippet to include matched content, got %q", body)
	}
}

func TestSearchHandlerReturnsInternalServerErrorWhenEmbeddingFails(t *testing.T) {
	server, _ := newTestServer(t, testEmbedder{err: context.DeadlineExceeded})

	req := httptest.NewRequest(http.MethodGet, "/search?q=test", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status %d", rec.Code)
	}
}

func newTestServer(t *testing.T, overrides ...testEmbedder) (*Server, string) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pages.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&scraper.SavedPage{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	scrapedAt := time.Date(2026, 3, 23, 10, 11, 12, 0, time.UTC)
	pages := []scraper.SavedPage{
		{
			ID:        1,
			URL:       "https://example.com/article",
			Host:      "example.com",
			Path:      "/article",
			PageKey:   "example.com%2Farticle_20260323T101112Z",
			Content:   "<article><p>clean article body</p></article>",
			ScrapedAt: scrapedAt,
		},
		{
			ID:        2,
			URL:       "https://example.com/second",
			Host:      "example.com",
			Path:      "/second",
			PageKey:   "example.com%2Fsecond_20260323T101112Z",
			Content:   "<article><p>second body</p></article>",
			ScrapedAt: scrapedAt.Add(5 * time.Minute),
		},
		{
			ID:        3,
			URL:       "https://news.example.org/start/post",
			Host:      "news.example.org",
			Path:      "/start/post",
			PageKey:   "news.example.org%2Fstart%2Fpost_20260323T101112Z",
			Content:   "<article><p>third body</p></article>",
			ScrapedAt: scrapedAt.Add(10 * time.Minute),
		},
	}
	if err := db.Create(&pages).Error; err != nil {
		t.Fatalf("seed pages: %v", err)
	}

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}

	embedder := testEmbedder{vectors: [][]float32{vectorWithLead(1, 0, 0)}}
	if len(overrides) > 0 {
		embedder = overrides[0]
	}

	server, err := NewServer(Config{
		DBPath:       dbPath,
		TemplatesDir: filepath.Join(filepath.Dir(testFile), "templates"),
		Logger:       slog.New(slog.NewTextHandler(os.Stderr, nil)),
		Embedder:     embedder,
		WhitelistPages: []string{
			"https://example.com",
			"https://news.example.org/start",
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	return server, dbPath
}

func insertTestEmbedding(dbPath string, pageID int64, vector []float32) error {
	sqlite_vec.Auto()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(`create virtual table if not exists page_embeddings using vec0(embedding float[2560] distance_metric=cosine)`); err != nil {
		return err
	}
	blob, err := sqlite_vec.SerializeFloat32(vector)
	if err != nil {
		return err
	}
	if _, err := db.Exec(`insert or replace into page_embeddings(rowid, embedding) values (?, ?)`, pageID, blob); err != nil {
		return err
	}

	return nil
}

type testEmbedder struct {
	vectors [][]float32
	err     error
}

func (t testEmbedder) Embed(_ context.Context, inputs []string) ([][]float32, error) {
	if t.err != nil {
		return nil, t.err
	}

	vectors := t.vectors
	if len(vectors) == 0 {
		vectors = [][]float32{vectorWithLead(1, 0, 0)}
	}

	result := make([][]float32, 0, len(inputs))
	for i := range inputs {
		result = append(result, append([]float32(nil), vectors[i%len(vectors)]...))
	}
	return result, nil
}

func vectorWithLead(values ...float32) []float32 {
	vector := make([]float32, 2560)
	copy(vector, values)
	return vector
}
