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
