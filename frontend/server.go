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
	"github.com/nilsherzig/llocalsearch/vectorstore"
	"golang.org/x/net/html"
	"gorm.io/gorm"
)

type Config struct {
	DBPath         string
	TemplatesDir   string
	Embeddings     scraper.EmbeddingConfig
	Embedder       embedding.Client
	WhitelistPages []string
	Logger         *slog.Logger
}

type Server struct {
	db             *gorm.DB
	embedder       embedding.Client
	logger         *slog.Logger
	templates      *template.Template
	mux            *http.ServeMux
	whitelistPages []string
}

type countSummary struct {
	Label string
	Count int
}

type dashboardData struct {
	TotalPages        int64
	EmbeddedPages     int64
	EmbeddingProgress int
	LatestScraped     string
	RecentPages       []scraper.SavedPage
	PagesPerHost      []countSummary
	PagesPerWhitelist []countSummary
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
	Query   string
	Results []searchResult
}

type searchAPIResponse struct {
	Query   string                 `json:"query"`
	Results []searchAPIResultEntry `json:"results"`
}

type searchAPIResultEntry struct {
	ID         uint      `json:"id"`
	URL        string    `json:"url"`
	Host       string    `json:"host"`
	Path       string    `json:"path"`
	ScrapedAt  time.Time `json:"scraped_at"`
	Excerpt    string    `json:"excerpt"`
	Similarity float64   `json:"similarity"`
}

type pageAPIResponse struct {
	ID        uint      `json:"id"`
	URL       string    `json:"url"`
	Host      string    `json:"host"`
	Path      string    `json:"path"`
	ScrapedAt time.Time `json:"scraped_at"`
	Content   string    `json:"content"`
}

type errorResponse struct {
	Error string `json:"error"`
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

	server := &Server{
		db:             db,
		embedder:       embedder,
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
	var allPages []scraper.SavedPage
	if err := s.db.Find(&allPages).Error; err != nil {
		s.logger.Error("load pages for dashboard failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	latestScraped := "n/a"
	if len(recentPages) > 0 {
		latestScraped = recentPages[0].ScrapedAt.Format(time.RFC3339)
	}

	data := dashboardData{
		TotalPages:        total,
		EmbeddedPages:     embeddedPages,
		EmbeddingProgress: embeddingProgressPercent(total, embeddedPages),
		LatestScraped:     latestScraped,
		RecentPages:       recentPages,
		PagesPerHost:      summarizePagesPerHost(allPages),
		PagesPerWhitelist: summarizePagesPerWhitelist(allPages, s.whitelistPages),
	}

	s.logger.Info("serve dashboard", "total_pages", total)
	if err := s.templates.ExecuteTemplate(w, "index.gohtml", data); err != nil {
		s.logger.Error("render dashboard failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func summarizePagesPerHost(pages []scraper.SavedPage) []countSummary {
	counts := make(map[string]int)
	for _, page := range pages {
		counts[page.Host]++
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

func embeddingProgressPercent(total int64, embedded int64) int {
	if total <= 0 || embedded <= 0 {
		return 0
	}
	if embedded >= total {
		return 100
	}
	return int((embedded * 100) / total)
}

func summarizePagesPerWhitelist(pages []scraper.SavedPage, whitelistPages []string) []countSummary {
	summaries := make([]countSummary, 0, len(whitelistPages))
	for _, seed := range whitelistPages {
		count := 0
		for _, page := range pages {
			if whitelistMatchesPage(seed, page) {
				count++
			}
		}
		summaries = append(summaries, countSummary{
			Label: seed,
			Count: count,
		})
	}

	return summaries
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

func sortedSummaries(counts map[string]int) []countSummary {
	summaries := make([]countSummary, 0, len(counts))
	for label, count := range counts {
		summaries = append(summaries, countSummary{
			Label: label,
			Count: count,
		})
	}

	slices.SortFunc(summaries, func(a, b countSummary) int {
		if a.Count != b.Count {
			return b.Count - a.Count
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
	if err := s.templates.ExecuteTemplate(w, "search.gohtml", searchData{
		Query:   query,
		Results: results,
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
			ID:         result.Page.ID,
			URL:        result.Page.URL,
			Host:       result.Page.Host,
			Path:       result.Page.Path,
			ScrapedAt:  result.Page.ScrapedAt,
			Excerpt:    result.Excerpt,
			Similarity: result.Similarity,
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
		ID:        page.ID,
		URL:       page.URL,
		Host:      page.Host,
		Path:      page.Path,
		ScrapedAt: page.ScrapedAt,
		Content:   page.Content,
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

	matches, err := vectorstore.Search(s.db, vectors[0], 20)
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
