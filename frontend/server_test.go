package frontend

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nilsherzig/llocalsearch/scraper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestIndexHandlerShowsDashboardMetrics(t *testing.T) {
	server := newTestServer(t)

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
	server := newTestServer(t)

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
	server := newTestServer(t)

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
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/pages/999", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status %d", rec.Code)
	}
}

func TestSearchHandlerFindsFuzzyMatchesAcrossDownloadedPages(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/search?q=artcle", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `Results for "artcle"`) {
		t.Fatalf("expected search heading, got %q", body)
	}
	if !strings.Contains(body, "https://example.com/article") {
		t.Fatalf("expected fuzzy search to find article page, got %q", body)
	}
	if !strings.Contains(body, "clean article body") {
		t.Fatalf("expected search snippet to include matched content, got %q", body)
	}
}

func newTestServer(t *testing.T) *Server {
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

	server, err := NewServer(Config{
		DBPath:       dbPath,
		TemplatesDir: filepath.Join(filepath.Dir(testFile), "templates"),
		Logger:       slog.New(slog.NewTextHandler(os.Stderr, nil)),
		WhitelistPages: []string{
			"https://example.com",
			"https://news.example.org/start",
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	return server
}
