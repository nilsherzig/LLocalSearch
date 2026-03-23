package frontend

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/nilsherzig/llocalsearch/summary"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestIndexHandlerShowsDashboardMetrics(t *testing.T) {
	server, dbPath := newTestServer(t)
	if err := insertTestEmbedding(dbPath, 1, vectorWithLead(1, 0, 0)); err != nil {
		t.Fatalf("insert first embedding: %v", err)
	}
	if err := insertTestEmbedding(dbPath, 2, vectorWithLead(0, 1, 0)); err != nil {
		t.Fatalf("insert second embedding: %v", err)
	}

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
	if !strings.Contains(body, "2 embedded pages") {
		t.Fatalf("expected embedded page count in dashboard, got %q", body)
	}
	if !strings.Contains(body, "https://example.com/article") {
		t.Fatalf("expected recent page url in dashboard, got %q", body)
	}
	if !strings.Contains(body, "<strong>example.com</strong>: 2 scraped pages, 2 embedded pages") {
		t.Fatalf("expected embedded host summary for example.com, got %q", body)
	}
	if !strings.Contains(body, "<strong>news.example.org</strong>: 1 scraped pages, 0 embedded pages") {
		t.Fatalf("expected embedded host summary for news.example.org, got %q", body)
	}
	if !strings.Contains(body, "https://example.com") || !strings.Contains(body, "2 scraped pages") {
		t.Fatalf("expected whitelist seed summary for example.com, got %q", body)
	}
	if !strings.Contains(body, "https://news.example.org/start") || !strings.Contains(body, "1 scraped pages") {
		t.Fatalf("expected whitelist seed summary for news.example.org/start, got %q", body)
	}
	if !strings.Contains(body, "<strong>https://example.com</strong>: 2 scraped pages, 2 embedded pages") {
		t.Fatalf("expected embedded whitelist summary for example.com, got %q", body)
	}
	if !strings.Contains(body, "<strong>https://news.example.org/start</strong>: 1 scraped pages, 0 embedded pages") {
		t.Fatalf("expected embedded whitelist summary for news.example.org/start, got %q", body)
	}
	if !strings.Contains(body, `action="/search"`) {
		t.Fatalf("expected search form on dashboard, got %q", body)
	}
	if !strings.Contains(body, "2 of 3 pages embedded") {
		t.Fatalf("expected embedding progress summary in dashboard, got %q", body)
	}
	if !strings.Contains(body, `role="progressbar"`) {
		t.Fatalf("expected embedding progress bar in dashboard, got %q", body)
	}
	if !strings.Contains(body, `aria-valuenow="66"`) {
		t.Fatalf("expected embedding progress value in dashboard, got %q", body)
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

func TestPageDetailHandlerConstrainsImagesToContentWidth(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/pages/1", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, ".content img") {
		t.Fatalf("expected image styling in detail view, got %q", body)
	}
	if !strings.Contains(body, "max-width: 100%") {
		t.Fatalf("expected images constrained to content width, got %q", body)
	}
	if !strings.Contains(body, "height: auto") {
		t.Fatalf("expected image aspect ratio to be preserved, got %q", body)
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
	if !strings.Contains(body, "Similarity: 1.0000") {
		t.Fatalf("expected rendered similarity score, got %q", body)
	}
}

func TestSearchHandlerRendersSummaryForWebResults(t *testing.T) {
	server, dbPath := newTestServer(t, testServerOptions{
		summarizer: testSummarizer{
			response: summary.Response{
				Text: "The results focus on Kubernetes guidance from the scraped pages.",
				Stats: summary.Stats{
					PromptEvalCount: 42,
					EvalCount:       18,
					EvalDuration:    1500 * time.Millisecond,
					TotalDuration:   2 * time.Second,
				},
			},
		},
	})
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
	if !strings.Contains(body, "Summary") {
		t.Fatalf("expected summary heading, got %q", body)
	}
	if !strings.Contains(body, "The results focus on Kubernetes guidance from the scraped pages.") {
		t.Fatalf("expected summary text, got %q", body)
	}
	if !strings.Contains(body, "12.0 tok/s") {
		t.Fatalf("expected tokens per second below summary, got %q", body)
	}
	if !strings.Contains(body, "18 output tok") {
		t.Fatalf("expected output token count below summary, got %q", body)
	}
}

func TestSearchHandlerPassesPlainContentContextToSummarizer(t *testing.T) {
	capturing := &capturingSummarizer{response: summary.Response{Text: "summary"}}
	server, dbPath := newTestServer(t, testServerOptions{
		summarizer: capturing,
	})
	if err := insertTestEmbedding(dbPath, 1, vectorWithLead(1, 0, 0)); err != nil {
		t.Fatalf("insert first embedding: %v", err)
	}

	longContent := "<article><p>Intro text before the main topic. kubernetes guide explains cluster setup and operations in detail.</p><p>Additional context that should reach the summarizer even when the excerpt is shorter. Unique tail marker.</p></article>"
	if err := server.db.Model(&scraper.SavedPage{}).Where("id = ?", 1).Update("content", longContent).Error; err != nil {
		t.Fatalf("update page content: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/search?q=kubernetes+guide", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}
	if len(capturing.gotResults) != 1 {
		t.Fatalf("expected one summarized result, got %d", len(capturing.gotResults))
	}
	if !strings.Contains(capturing.gotResults[0].Content, "Unique tail marker.") {
		t.Fatalf("expected full plain text content in summarizer input, got %#v", capturing.gotResults[0])
	}
}

func TestSearchAPIHandlerReturnsJSONResults(t *testing.T) {
	server, dbPath := newTestServer(t, testServerOptions{
		summarizer: testSummarizer{
			response: summary.Response{
				Text: "The API response must not include this summary.",
			},
		},
	})
	if err := insertTestEmbedding(dbPath, 1, vectorWithLead(1, 0, 0)); err != nil {
		t.Fatalf("insert first embedding: %v", err)
	}
	if err := insertTestEmbedding(dbPath, 2, vectorWithLead(0, 1, 0)); err != nil {
		t.Fatalf("insert second embedding: %v", err)
	}
	if err := insertTestEmbedding(dbPath, 3, vectorWithLead(0, 0, 1)); err != nil {
		t.Fatalf("insert third embedding: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=kubernetes+guide", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type %q", got)
	}

	var response searchAPIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Query != "kubernetes guide" {
		t.Fatalf("expected query in response, got %q", response.Query)
	}
	if len(response.Results) == 0 {
		t.Fatalf("expected at least one search result, got %+v", response)
	}
	if response.Results[0].URL != "https://example.com/article" {
		t.Fatalf("expected matched page url in response, got %q", response.Results[0].URL)
	}
	if response.Results[0].Excerpt != "clean article body" {
		t.Fatalf("expected matched page excerpt in response, got %q", response.Results[0].Excerpt)
	}
	if response.Results[0].Similarity != 1 {
		t.Fatalf("expected similarity 1 for exact vector match, got %v", response.Results[0].Similarity)
	}
	if strings.Contains(rec.Body.String(), "The API response must not include this summary.") {
		t.Fatalf("expected summary to stay out of api response, got %q", rec.Body.String())
	}
}

func TestPageContentAPIHandlerReturnsFullPageContent(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/pages/1", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type %q", got)
	}

	var response struct {
		ID        uint      `json:"id"`
		URL       string    `json:"url"`
		Host      string    `json:"host"`
		Path      string    `json:"path"`
		ScrapedAt time.Time `json:"scraped_at"`
		Content   string    `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ID != 1 {
		t.Fatalf("expected page id 1, got %d", response.ID)
	}
	if response.URL != "https://example.com/article" {
		t.Fatalf("expected page url in response, got %q", response.URL)
	}
	if response.Content != "<article><p>clean article body</p></article>" {
		t.Fatalf("expected full page content in response, got %q", response.Content)
	}
}

func TestPageContentAPIHandlerReturnsNotFoundForUnknownPage(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/pages/999", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type %q", got)
	}
	if !strings.Contains(rec.Body.String(), `"error":"page not found"`) {
		t.Fatalf("expected json not found body, got %q", rec.Body.String())
	}
}

func TestSearchHandlerReturnsInternalServerErrorWhenEmbeddingFails(t *testing.T) {
	server, _ := newTestServer(t, testServerOptions{embedder: testEmbedder{err: context.DeadlineExceeded}})

	req := httptest.NewRequest(http.MethodGet, "/search?q=test", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status %d", rec.Code)
	}
}

func TestSearchAPIHandlerReturnsJSONErrorWhenEmbeddingFails(t *testing.T) {
	server, _ := newTestServer(t, testServerOptions{embedder: testEmbedder{err: context.DeadlineExceeded}})

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=test", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type %q", got)
	}
	if !strings.Contains(rec.Body.String(), `"error":"internal server error"`) {
		t.Fatalf("expected json error body, got %q", rec.Body.String())
	}
}

type testServerOptions struct {
	embedder   testEmbedder
	summarizer summary.Client
}

func newTestServer(t *testing.T, overrides ...testServerOptions) (*Server, string) {
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
	var summarizer summary.Client
	if len(overrides) > 0 {
		if len(overrides[0].embedder.vectors) > 0 || overrides[0].embedder.err != nil {
			embedder = overrides[0].embedder
		}
		summarizer = overrides[0].summarizer
	}

	server, err := NewServer(Config{
		DBPath:       dbPath,
		TemplatesDir: filepath.Join(filepath.Dir(testFile), "templates"),
		Logger:       slog.New(slog.NewTextHandler(os.Stderr, nil)),
		Embedder:     embedder,
		Summarizer:   summarizer,
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

type testSummarizer struct {
	response summary.Response
	err      error
}

type capturingSummarizer struct {
	response   summary.Response
	gotResults []summary.Result
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

func (t testSummarizer) Summarize(_ context.Context, query string, results []summary.Result) (summary.Response, error) {
	if t.err != nil {
		return summary.Response{}, t.err
	}
	if strings.TrimSpace(query) == "" || len(results) == 0 {
		return summary.Response{}, nil
	}
	return t.response, nil
}

func (c *capturingSummarizer) Summarize(_ context.Context, query string, results []summary.Result) (summary.Response, error) {
	c.gotResults = append([]summary.Result(nil), results...)
	return c.response, nil
}

func vectorWithLead(values ...float32) []float32 {
	vector := make([]float32, 2560)
	copy(vector, values)
	return vector
}
