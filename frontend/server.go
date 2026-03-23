package frontend

import (
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

	"github.com/nilsherzig/llocalsearch/scraper"
	"golang.org/x/net/html"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Config struct {
	DBPath         string
	TemplatesDir   string
	WhitelistPages []string
	Logger         *slog.Logger
}

type Server struct {
	db             *gorm.DB
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
	Page    scraper.SavedPage
	Excerpt string
}

type searchData struct {
	Query   string
	Results []searchResult
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

	db, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	if err := db.AutoMigrate(&scraper.SavedPage{}); err != nil {
		return nil, fmt.Errorf("migrate sqlite database: %w", err)
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

	server := &Server{
		db:             db,
		logger:         logger,
		templates:      templates,
		mux:            http.NewServeMux(),
		whitelistPages: cfg.WhitelistPages,
	}

	server.mux.HandleFunc("/", server.handleDashboard)
	server.mux.HandleFunc("/pages", server.handlePages)
	server.mux.HandleFunc("/pages/", server.handlePageDetail)
	server.mux.HandleFunc("/search", server.handleSearch)

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
	idText := strings.TrimPrefix(r.URL.Path, "/pages/")
	id, err := strconv.ParseUint(idText, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var page scraper.SavedPage
	if err := s.db.First(&page, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.NotFound(w, r)
			return
		}
		s.logger.Error("load page failed", "id", id, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
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

func (s *Server) searchPages(query string) ([]searchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	var pages []scraper.SavedPage
	if err := s.db.Order("scraped_at desc").Find(&pages).Error; err != nil {
		return nil, err
	}

	type scoredResult struct {
		searchResult
		score int
	}

	scored := make([]scoredResult, 0, len(pages))
	for _, page := range pages {
		plainContent := extractPlainText(page.Content)
		score := scorePageSearch(query, page, plainContent)
		if score <= 0 {
			continue
		}

		scored = append(scored, scoredResult{
			searchResult: searchResult{
				Page:    page,
				Excerpt: buildExcerpt(plainContent, query),
			},
			score: score,
		})
	}

	slices.SortFunc(scored, func(a, b scoredResult) int {
		if a.score != b.score {
			return b.score - a.score
		}
		if !a.Page.ScrapedAt.Equal(b.Page.ScrapedAt) {
			if a.Page.ScrapedAt.After(b.Page.ScrapedAt) {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Page.URL, b.Page.URL)
	})

	if len(scored) > 25 {
		scored = scored[:25]
	}

	results := make([]searchResult, 0, len(scored))
	for _, item := range scored {
		results = append(results, item.searchResult)
	}

	return results, nil
}

func scorePageSearch(query string, page scraper.SavedPage, plainContent string) int {
	queryTerms := tokenizeSearchText(query)
	if len(queryTerms) == 0 {
		return 0
	}

	fieldWeights := []struct {
		text   string
		weight int
	}{
		{text: page.URL, weight: 7},
		{text: page.Host, weight: 6},
		{text: page.Path, weight: 5},
		{text: plainContent, weight: 3},
	}

	total := 0
	for _, term := range queryTerms {
		termBest := 0
		for _, field := range fieldWeights {
			score := scoreField(term, field.text) * field.weight
			if score > termBest {
				termBest = score
			}
		}
		if termBest == 0 {
			return 0
		}
		total += termBest
	}

	return total
}

func scoreField(query string, text string) int {
	normalizedText := normalizeSearchText(text)
	if normalizedText == "" {
		return 0
	}
	if idx := strings.Index(normalizedText, query); idx >= 0 {
		return 1000 - min(idx, 300)
	}

	best := 0
	for _, token := range strings.Fields(normalizedText) {
		if token == query {
			return 950
		}
		if strings.HasPrefix(token, query) {
			best = max(best, 900-len(token)+len(query))
		}
		if score := fuzzyTokenScore(query, token); score > best {
			best = score
		}
	}

	return best
}

func fuzzyTokenScore(query string, token string) int {
	if token == "" {
		return 0
	}

	dist := levenshteinDistance(query, token)
	maxDistance := max(1, len(query)/3)
	if dist <= maxDistance {
		return 760 - dist*120 - abs(len(token)-len(query))*15
	}

	if matched, gaps := subsequenceGapScore(query, token); matched {
		return 520 - gaps*10 - abs(len(token)-len(query))*5
	}

	return 0
}

func buildExcerpt(text string, query string) string {
	normalizedText := strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if normalizedText == "" {
		return "No text excerpt available."
	}

	if len(normalizedText) <= 220 {
		return normalizedText
	}

	normalizedQuery := normalizeSearchText(query)
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

func tokenizeSearchText(input string) []string {
	normalized := normalizeSearchText(input)
	if normalized == "" {
		return nil
	}
	return strings.Fields(normalized)
}

func normalizeSearchText(input string) string {
	var builder strings.Builder
	builder.Grow(len(input))
	lastSpace := true
	for _, r := range strings.ToLower(input) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			builder.WriteByte(' ')
			lastSpace = true
		}
	}

	return strings.TrimSpace(builder.String())
}

func levenshteinDistance(a string, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr := make([]int, len(b)+1)
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			curr[j] = min(
				min(curr[j-1]+1, prev[j]+1),
				prev[j-1]+cost,
			)
		}
		prev = curr
	}

	return prev[len(b)]
}

func subsequenceGapScore(query string, token string) (bool, int) {
	if len(query) == 0 {
		return false, 0
	}

	queryIndex := 0
	lastMatch := -1
	gaps := 0
	for i := 0; i < len(token) && queryIndex < len(query); i++ {
		if token[i] != query[queryIndex] {
			continue
		}
		if lastMatch >= 0 {
			gaps += i - lastMatch - 1
		}
		lastMatch = i
		queryIndex++
	}

	return queryIndex == len(query), gaps
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
