package frontend

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nilsherzig/llocalsearch/embedding"
	"github.com/nilsherzig/llocalsearch/scraper"
	"github.com/nilsherzig/llocalsearch/summary"
	"github.com/nilsherzig/llocalsearch/vectorstore"
	"golang.org/x/net/html"
	"gorm.io/gorm"
)

type Config struct {
	DBPath         string
	TemplatesDir   string
	Embeddings     scraper.EmbeddingConfig
	SummaryLLM     scraper.SummaryLLMConfig
	Embedder       embedding.Client
	Summarizer     summary.Client
	WhitelistPages []string
	Logger         *slog.Logger
}

type Server struct {
	db             *gorm.DB
	pageStore      *scraper.PageStore
	embedder       embedding.Client
	summarizer     summary.Client
	logger         *slog.Logger
	templates      *template.Template
	mux            *http.ServeMux
	whitelistPages []string
}

type countSummary struct {
	Label         string
	ScrapedCount  int
	EmbeddedCount int
}

type dashboardData struct {
	TotalPages            int64
	EmbeddedPages         int64
	EmbeddingProgress     int
	LatestScraped         string
	LatestBrowserCapture  string
	BrowserIngestEndpoint string
	RecentPages           []scraper.SavedPage
	RecentBrowserPages    []scraper.SavedPage
	PagesPerHost          []countSummary
	PagesPerWhitelist     []countSummary
	PagesPerSource        []countSummary
}

type pagesData struct {
	Pages []scraper.SavedPage
}

type pageDetailData struct {
	Page    scraper.SavedPage
	Content template.HTML
}

type searchResult struct {
	Page       scraper.SavedPage
	Excerpt    string
	Similarity float64
}

type searchData struct {
	Query        string
	Summary      string
	SummaryStats string
	Results      []searchResult
}

type searchAPIResponse struct {
	Query   string                 `json:"query"`
	Results []searchAPIResultEntry `json:"results"`
}

type searchAPIResultEntry struct {
	ID             uint      `json:"id"`
	URL            string    `json:"url"`
	Host           string    `json:"host"`
	Path           string    `json:"path"`
	Title          string    `json:"title"`
	ScrapedAt      time.Time `json:"scraped_at"`
	CapturedAt     time.Time `json:"captured_at"`
	SourceType     string    `json:"source_type"`
	SourceBrowser  string    `json:"source_browser"`
	SourceDeviceID string    `json:"source_device_id"`
	Excerpt        string    `json:"excerpt"`
	Similarity     float64   `json:"similarity"`
}

type pageAPIResponse struct {
	ID             uint      `json:"id"`
	URL            string    `json:"url"`
	Host           string    `json:"host"`
	Path           string    `json:"path"`
	Title          string    `json:"title"`
	ScrapedAt      time.Time `json:"scraped_at"`
	CapturedAt     time.Time `json:"captured_at"`
	SourceType     string    `json:"source_type"`
	SourceBrowser  string    `json:"source_browser"`
	SourceDeviceID string    `json:"source_device_id"`
	Content        string    `json:"content"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type browserIngestRequest struct {
	URL        string    `json:"url"`
	FinalURL   string    `json:"final_url"`
	Title      string    `json:"title"`
	CapturedAt time.Time `json:"captured_at"`
	HTML       string    `json:"html"`
	Browser    string    `json:"browser"`
	DeviceID   string    `json:"device_id"`
}

type browserIngestResponse struct {
	ID         uint      `json:"id"`
	URL        string    `json:"url"`
	Reused     bool      `json:"reused"`
	SourceType string    `json:"source_type"`
	CapturedAt time.Time `json:"captured_at"`
}

func NewServer(cfg Config) (*Server, error) {
	if cfg.DBPath == "" {
		return nil, fmt.Errorf("db path is required")
	}
	if cfg.TemplatesDir == "" {
		return nil, fmt.Errorf("templates dir is required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := vectorstore.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&scraper.SavedPage{}); err != nil {
		return nil, fmt.Errorf("migrate sqlite database: %w", err)
	}
	if err := vectorstore.EnsureSchema(db, cfg.Embeddings.Model, cfg.Embeddings.Dimensions); err != nil {
		return nil, err
	}

	templates, err := template.ParseFiles(
		filepath.Join(cfg.TemplatesDir, "index.gohtml"),
		filepath.Join(cfg.TemplatesDir, "pages.gohtml"),
		filepath.Join(cfg.TemplatesDir, "page.gohtml"),
		filepath.Join(cfg.TemplatesDir, "search.gohtml"),
	)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	embedder := cfg.Embedder
	if embedder == nil && cfg.Embeddings.BaseURL != "" && cfg.Embeddings.Model != "" {
		embedder = embedding.NewClient(embedding.Config{
			BaseURL:    cfg.Embeddings.BaseURL,
			Model:      cfg.Embeddings.Model,
			Dimensions: cfg.Embeddings.Dimensions,
			Timeout:    time.Duration(cfg.Embeddings.Timeout),
		}, nil)
	}

	summarizer := cfg.Summarizer
	if summarizer == nil && cfg.Embeddings.BaseURL != "" && cfg.SummaryLLM.Model != "" {
		summarizer = summary.NewClient(summary.Config{
			BaseURL: cfg.Embeddings.BaseURL,
			Model:   cfg.SummaryLLM.Model,
			Timeout: time.Duration(cfg.SummaryLLM.Timeout),
		}, nil)
	}

	server := &Server{
		db:             db,
		pageStore:      scraper.NewPageStore(db),
		embedder:       embedder,
		summarizer:     summarizer,
		logger:         logger,
		templates:      templates,
		mux:            http.NewServeMux(),
		whitelistPages: cfg.WhitelistPages,
	}

	server.mux.HandleFunc("/", server.handleDashboard)
	server.mux.HandleFunc("/pages", server.handlePages)
	server.mux.HandleFunc("/pages/", server.handlePageDetail)
	server.mux.HandleFunc("/search", server.handleSearch)
	server.mux.HandleFunc("/api/search", server.handleSearchAPI)
	server.mux.HandleFunc("/api/pages/", server.handlePageContentAPI)
	server.mux.HandleFunc("/api/ingest/browser-pages", server.handleBrowserIngestAPI)

	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	var total int64
	if err := s.db.Model(&scraper.SavedPage{}).Count(&total).Error; err != nil {
		s.logger.Error("count pages failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var recentPages []scraper.SavedPage
	if err := s.db.Order("scraped_at desc").Limit(10).Find(&recentPages).Error; err != nil {
		s.logger.Error("load recent pages failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	embeddedPages, err := s.countEmbeddedPages()
	if err != nil {
		s.logger.Error("count embedded pages failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	embeddedPageIDs, err := s.embeddedPageIDs()
	if err != nil {
		s.logger.Error("load embedded page ids failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	var allPages []scraper.SavedPage
	if err := s.db.Find(&allPages).Error; err != nil {
		s.logger.Error("load pages for dashboard failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	recentBrowserPages, latestBrowserCapture := summarizeRecentBrowserPages(allPages)

	latestScraped := "n/a"
	if len(recentPages) > 0 {
		latestScraped = recentPages[0].ScrapedAt.Format(time.RFC3339)
	}
	if latestBrowserCapture == "" {
		latestBrowserCapture = "n/a"
	}

	data := dashboardData{
		TotalPages:            total,
		EmbeddedPages:         embeddedPages,
		EmbeddingProgress:     embeddingProgressPercent(total, embeddedPages),
		LatestScraped:         latestScraped,
		LatestBrowserCapture:  latestBrowserCapture,
		BrowserIngestEndpoint: "/api/ingest/browser-pages",
		RecentPages:           recentPages,
		RecentBrowserPages:    recentBrowserPages,
		PagesPerHost:          summarizePagesPerHost(allPages, embeddedPageIDs),
		PagesPerWhitelist:     summarizePagesPerWhitelist(allPages, embeddedPageIDs, s.whitelistPages),
		PagesPerSource:        summarizePagesPerSource(allPages, embeddedPageIDs),
	}

	s.logger.Info("serve dashboard", "total_pages", total)
	if err := s.templates.ExecuteTemplate(w, "index.gohtml", data); err != nil {
		s.logger.Error("render dashboard failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func summarizePagesPerHost(pages []scraper.SavedPage, embeddedPageIDs map[uint]struct{}) []countSummary {
	counts := make(map[string]countSummary)
	for _, page := range pages {
		summary := counts[page.Host]
		summary.Label = page.Host
		summary.ScrapedCount++
		if _, ok := embeddedPageIDs[page.ID]; ok {
			summary.EmbeddedCount++
		}
		counts[page.Host] = summary
	}

	return sortedSummaries(counts)
}

func (s *Server) countEmbeddedPages() (int64, error) {
	type countRow struct {
		Count int64 `gorm:"column:count"`
	}

	var row countRow
	result := s.db.Raw(`
		select count(*) as count
		from saved_pages
		inner join page_embeddings on page_embeddings.rowid = saved_pages.id
	`).Scan(&row)
	if result.Error != nil {
		return 0, result.Error
	}

	return row.Count, nil
}

func (s *Server) embeddedPageIDs() (map[uint]struct{}, error) {
	type row struct {
		ID uint `gorm:"column:id"`
	}

	var rows []row
	result := s.db.Raw(`
		select saved_pages.id as id
		from saved_pages
		inner join page_embeddings on page_embeddings.rowid = saved_pages.id
	`).Scan(&rows)
	if result.Error != nil {
		return nil, result.Error
	}

	ids := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		ids[row.ID] = struct{}{}
	}
	return ids, nil
}

func embeddingProgressPercent(total int64, embedded int64) int {
	if total <= 0 || embedded <= 0 {
		return 0
	}
	if embedded >= total {
		return 100
	}
	return int((embedded * 100) / total)
}

func summarizePagesPerWhitelist(pages []scraper.SavedPage, embeddedPageIDs map[uint]struct{}, whitelistPages []string) []countSummary {
	summaries := make([]countSummary, 0, len(whitelistPages))
	for _, seed := range whitelistPages {
		summary := countSummary{Label: seed}
		for _, page := range pages {
			if whitelistMatchesPage(seed, page) {
				summary.ScrapedCount++
				if _, ok := embeddedPageIDs[page.ID]; ok {
					summary.EmbeddedCount++
				}
			}
		}
		summaries = append(summaries, summary)
	}

	return summaries
}

func summarizePagesPerSource(pages []scraper.SavedPage, embeddedPageIDs map[uint]struct{}) []countSummary {
	counts := make(map[string]countSummary)
	for _, page := range pages {
		label := sourceLabel(page)
		summary := counts[label]
		summary.Label = label
		summary.ScrapedCount++
		if _, ok := embeddedPageIDs[page.ID]; ok {
			summary.EmbeddedCount++
		}
		counts[label] = summary
	}
	return sortedSummaries(counts)
}

func summarizeRecentBrowserPages(pages []scraper.SavedPage) ([]scraper.SavedPage, string) {
	browserPages := make([]scraper.SavedPage, 0)
	for _, page := range pages {
		if page.SourceType != scraper.SourceTypeBrowserExtension {
			continue
		}
		browserPages = append(browserPages, page)
	}
	slices.SortFunc(browserPages, func(a, b scraper.SavedPage) int {
		return b.CapturedAt.Compare(a.CapturedAt)
	})
	if len(browserPages) == 0 {
		return nil, ""
	}
	if len(browserPages) > 5 {
		browserPages = browserPages[:5]
	}
	return browserPages, browserPages[0].CapturedAt.Format(time.RFC3339)
}

func whitelistMatchesPage(seed string, page scraper.SavedPage) bool {
	parsedSeed, err := url.Parse(seed)
	if err != nil {
		return false
	}
	if parsedSeed.Host != page.Host {
		return false
	}

	seedPath := parsedSeed.EscapedPath()
	if seedPath == "" || seedPath == "/" {
		return true
	}

	return page.Path == seedPath || strings.HasPrefix(page.Path, seedPath+"/")
}

func sortedSummaries(counts map[string]countSummary) []countSummary {
	summaries := make([]countSummary, 0, len(counts))
	for _, summary := range counts {
		summaries = append(summaries, summary)
	}

	slices.SortFunc(summaries, func(a, b countSummary) int {
		if a.ScrapedCount != b.ScrapedCount {
			return b.ScrapedCount - a.ScrapedCount
		}
		return strings.Compare(a.Label, b.Label)
	})

	return summaries
}

func (s *Server) handlePages(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/pages" {
		http.NotFound(w, r)
		return
	}

	var pages []scraper.SavedPage
	if err := s.db.Order("scraped_at desc").Find(&pages).Error; err != nil {
		s.logger.Error("load pages failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Info("serve pages", "count", len(pages))
	if err := s.templates.ExecuteTemplate(w, "pages.gohtml", pagesData{Pages: pages}); err != nil {
		s.logger.Error("render pages failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handlePageDetail(w http.ResponseWriter, r *http.Request) {
	page, found, err := s.loadPageByPathID(r.URL.Path, "/pages/")
	if err != nil {
		s.logger.Error("load page failed", "path", r.URL.Path, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}

	s.logger.Info("serve page detail", "id", page.ID, "url", page.URL)
	if err := s.templates.ExecuteTemplate(w, "page.gohtml", pageDetailData{
		Page:    page,
		Content: template.HTML(page.Content),
	}); err != nil {
		s.logger.Error("render page detail failed", "id", page.ID, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/search" {
		http.NotFound(w, r)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	results, err := s.searchPages(query)
	if err != nil {
		s.logger.Error("search pages failed", "query", query, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Info("serve search", "query", query, "results", len(results))
	summaryResponse, err := s.summarizeSearch(query, results)
	if err != nil {
		s.logger.Error("summarize search failed", "query", query, "err", err)
	}
	if err := s.templates.ExecuteTemplate(w, "search.gohtml", searchData{
		Query:        query,
		Summary:      summaryResponse.Text,
		SummaryStats: formatSummaryStats(summaryResponse.Stats),
		Results:      results,
	}); err != nil {
		s.logger.Error("render search failed", "query", query, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleSearchAPI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/search" {
		http.NotFound(w, r)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	results, err := s.searchPages(query)
	if err != nil {
		s.logger.Error("search api failed", "query", query, "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	response := searchAPIResponse{
		Query:   query,
		Results: make([]searchAPIResultEntry, 0, len(results)),
	}
	for _, result := range results {
		response.Results = append(response.Results, searchAPIResultEntry{
			ID:             result.Page.ID,
			URL:            result.Page.URL,
			Host:           result.Page.Host,
			Path:           result.Page.Path,
			Title:          result.Page.Title,
			ScrapedAt:      result.Page.ScrapedAt,
			CapturedAt:     result.Page.CapturedAt,
			SourceType:     result.Page.SourceType,
			SourceBrowser:  result.Page.SourceBrowser,
			SourceDeviceID: result.Page.SourceDeviceID,
			Excerpt:        result.Excerpt,
			Similarity:     result.Similarity,
		})
	}

	s.logger.Info("serve search api", "query", query, "results", len(response.Results))
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handlePageContentAPI(w http.ResponseWriter, r *http.Request) {
	page, found, err := s.loadPageByPathID(r.URL.Path, "/api/pages/")
	if err != nil {
		s.logger.Error("load page api failed", "path", r.URL.Path, "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "page not found"})
		return
	}

	s.logger.Info("serve page api", "id", page.ID, "url", page.URL)
	writeJSON(w, http.StatusOK, pageAPIResponse{
		ID:             page.ID,
		URL:            page.URL,
		Host:           page.Host,
		Path:           page.Path,
		Title:          page.Title,
		ScrapedAt:      page.ScrapedAt,
		CapturedAt:     page.CapturedAt,
		SourceType:     page.SourceType,
		SourceBrowser:  page.SourceBrowser,
		SourceDeviceID: page.SourceDeviceID,
		Content:        page.Content,
	})
}

func (s *Server) handleBrowserIngestAPI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/ingest/browser-pages" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var request browserIngestRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10<<20))
	if err := decoder.Decode(&request); err != nil {
		status := http.StatusBadRequest
		message := "invalid request body"
		if strings.Contains(strings.ToLower(err.Error()), "too large") {
			status = http.StatusRequestEntityTooLarge
			message = "request body too large"
		}
		writeJSON(w, status, errorResponse{Error: message})
		return
	}

	rawURL := strings.TrimSpace(request.FinalURL)
	if rawURL == "" {
		rawURL = strings.TrimSpace(request.URL)
	}
	if strings.TrimSpace(rawURL) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "page url is required"})
		return
	}
	if strings.TrimSpace(request.HTML) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "html is required"})
		return
	}

	result, err := s.pageStore.SaveHTMLCapture(scraper.HTMLCapture{
		URL:            rawURL,
		HTML:           []byte(request.HTML),
		Title:          request.Title,
		SourceType:     scraper.SourceTypeBrowserExtension,
		SourceBrowser:  strings.TrimSpace(request.Browser),
		SourceDeviceID: strings.TrimSpace(request.DeviceID),
		CapturedAt:     request.CapturedAt,
	})
	if err != nil {
		s.logger.Error("browser ingest failed", "url", rawURL, "err", err)
		if isBrowserIngestBadRequest(err) {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: browserIngestErrorMessage(err)})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	status := http.StatusCreated
	if result.Reused {
		status = http.StatusOK
	}
	writeJSON(w, status, browserIngestResponse{
		ID:         result.Page.ID,
		URL:        result.Page.URL,
		Reused:     result.Reused,
		SourceType: result.Page.SourceType,
		CapturedAt: result.Page.CapturedAt,
	})
}

func (s *Server) searchPages(query string) ([]searchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if s.embedder == nil {
		return nil, fmt.Errorf("embedder not configured")
	}

	vectors, err := s.embedder.Embed(context.Background(), []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("embed query returned %d vectors", len(vectors))
	}

	matches, err := vectorstore.Search(s.db, vectors[0], 10)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}

	ids := make([]uint, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match.RowID)
	}

	var pages []scraper.SavedPage
	if err := s.db.Where("id IN ?", ids).Find(&pages).Error; err != nil {
		return nil, err
	}

	pagesByID := make(map[uint]scraper.SavedPage, len(pages))
	for _, page := range pages {
		pagesByID[page.ID] = page
	}

	results := make([]searchResult, 0, len(matches))
	for _, match := range matches {
		page, ok := pagesByID[match.RowID]
		if !ok {
			continue
		}
		plainContent := extractPlainText(page.Content)
		results = append(results, searchResult{
			Page:       page,
			Excerpt:    buildExcerpt(plainContent, query),
			Similarity: similarityFromDistance(match.Distance),
		})
	}

	return results, nil
}

func (s *Server) summarizeSearch(query string, results []searchResult) (summary.Response, error) {
	if s.summarizer == nil || strings.TrimSpace(query) == "" || len(results) == 0 {
		return summary.Response{}, nil
	}

	summaryResults := make([]summary.Result, 0, len(results))
	for _, result := range results {
		summaryResults = append(summaryResults, summary.Result{
			URL:        result.Page.URL,
			Host:       result.Page.Host,
			Path:       result.Page.Path,
			Excerpt:    result.Excerpt,
			Content:    extractPlainText(result.Page.Content),
			Similarity: result.Similarity,
		})
	}

	return s.summarizer.Summarize(context.Background(), query, summaryResults)
}

func formatSummaryStats(stats summary.Stats) string {
	parts := make([]string, 0, 4)
	if tokensPerSecond := stats.TokensPerSecond(); tokensPerSecond > 0 {
		parts = append(parts, fmt.Sprintf("%.1f tok/s", tokensPerSecond))
	}
	if stats.EvalCount > 0 {
		parts = append(parts, fmt.Sprintf("%d output tok", stats.EvalCount))
	}
	if stats.PromptEvalCount > 0 {
		parts = append(parts, fmt.Sprintf("%d prompt tok", stats.PromptEvalCount))
	}
	if stats.TotalDuration > 0 {
		parts = append(parts, stats.TotalDuration.Round(100*time.Millisecond).String())
	}
	return strings.Join(parts, " · ")
}

func (s *Server) loadPageByPathID(path string, prefix string) (scraper.SavedPage, bool, error) {
	idText := strings.TrimPrefix(path, prefix)
	id, err := strconv.ParseUint(idText, 10, 64)
	if err != nil {
		return scraper.SavedPage{}, false, nil
	}

	var page scraper.SavedPage
	if err := s.db.First(&page, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return scraper.SavedPage{}, false, nil
		}
		return scraper.SavedPage{}, false, err
	}

	return page, true, nil
}

func similarityFromDistance(distance float64) float64 {
	similarity := 1 - distance
	if similarity > 1 {
		return 1
	}
	if similarity < -1 {
		return -1
	}
	return similarity
}

func buildExcerpt(text string, query string) string {
	normalizedText := strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if normalizedText == "" {
		return "No text excerpt available."
	}

	if len(normalizedText) <= 220 {
		return normalizedText
	}

	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery != "" {
		lowerText := strings.ToLower(normalizedText)
		if idx := strings.Index(lowerText, normalizedQuery); idx >= 0 {
			start := max(0, idx-60)
			end := min(len(normalizedText), idx+len(normalizedQuery)+120)
			excerpt := normalizedText[start:end]
			if start > 0 {
				excerpt = "..." + excerpt
			}
			if end < len(normalizedText) {
				excerpt += "..."
			}
			return excerpt
		}
	}

	return normalizedText[:220] + "..."
}

func extractPlainText(content string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(content))
	var builder strings.Builder
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return strings.TrimSpace(strings.Join(strings.Fields(builder.String()), " "))
		case html.TextToken:
			text := strings.TrimSpace(string(tokenizer.Text()))
			if text == "" {
				continue
			}
			if builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			builder.WriteString(text)
		}
	}
}

func sourceLabel(page scraper.SavedPage) string {
	if page.SourceType == scraper.SourceTypeBrowserExtension {
		browser := strings.TrimSpace(page.SourceBrowser)
		if browser != "" {
			return strings.Title(browser)
		}
		return "Browser Extension"
	}
	return "Scraper"
}

func isBrowserIngestBadRequest(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unsupported page url scheme") ||
		strings.Contains(message, "page url must include a host") ||
		strings.Contains(message, "parse page url")
}

func browserIngestErrorMessage(err error) string {
	if err == nil {
		return "invalid browser ingest request"
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unsupported page url scheme") {
		return "unsupported page url scheme"
	}
	if strings.Contains(message, "page url must include a host") {
		return "page url must include a host"
	}
	if strings.Contains(message, "parse page url") {
		return "invalid page url"
	}
	return "invalid browser ingest request"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
