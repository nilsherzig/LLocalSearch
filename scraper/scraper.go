package scraper

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/gocolly/colly/v2"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Config struct {
	CacheDir  string   `yaml:"cache_dir"`
	HostDelay Duration `yaml:"host_delay"`
	Websites  []string `yaml:"websites"`
}

type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Value == "" {
		*d = 0
		return nil
	}

	parsed, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value.Value, err)
	}

	*d = Duration(parsed)
	return nil
}

type Scraper struct {
	cacheDir  string
	dbPath    string
	websites  []string
	logger    *slog.Logger
	now       func() time.Time
	hostDelay time.Duration
	db        *gorm.DB
}

type SavedPage struct {
	ID          uint      `gorm:"primaryKey"`
	URL         string    `gorm:"not null;index;uniqueIndex:idx_saved_pages_url_hash"`
	Host        string    `gorm:"not null;index"`
	Path        string    `gorm:"not null"`
	PageKey     string    `gorm:"not null;index"`
	ContentHash string    `gorm:"not null;index;uniqueIndex:idx_saved_pages_url_hash"`
	ParseMode   string    `gorm:"not null;default:readability"`
	Content     string    `gorm:"type:text;not null"`
	ScrapedAt   time.Time `gorm:"not null;index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const firefoxUserAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:136.0) Gecko/20100101 Firefox/136.0"

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	return normalizeConfig(cfg)
}

type Option func(*Scraper)

func WithLogger(logger *slog.Logger) Option {
	return func(s *Scraper) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func WithNow(now func() time.Time) Option {
	return func(s *Scraper) {
		if now != nil {
			s.now = now
		}
	}
}

func WithHostDelay(delay time.Duration) Option {
	return func(s *Scraper) {
		if delay >= 0 {
			s.hostDelay = delay
		}
	}
}

func New(cfg Config, opts ...Option) (*Scraper, error) {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}

	allowedHosts := make([]string, 0, len(normalized.Websites))
	for _, website := range normalized.Websites {
		_, host, err := normalizeTarget(website)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(allowedHosts, host) {
			allowedHosts = append(allowedHosts, host)
		}
	}

	s := &Scraper{
		cacheDir:  normalized.CacheDir,
		dbPath:    filepath.Join(normalized.CacheDir, "pages.db"),
		websites:  allowedHosts,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:       time.Now,
		hostDelay: 500 * time.Millisecond,
	}
	if normalized.HostDelay > 0 {
		s.hostDelay = time.Duration(normalized.HostDelay)
	}

	for _, opt := range opts {
		opt(s)
	}

	db, err := openDatabase(s.dbPath)
	if err != nil {
		return nil, err
	}
	s.db = db

	return s, nil
}

func (s *Scraper) Fetch(rawURL string) ([]SavedPage, error) {
	return s.FetchAll([]string{rawURL})
}

func (s *Scraper) FetchAll(rawURLs []string) ([]SavedPage, error) {
	if len(rawURLs) == 0 {
		return nil, nil
	}

	for _, rawURL := range rawURLs {
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("parse url: %w", err)
		}

		host := normalizeHost(parsedURL.Hostname())
		if !slices.Contains(s.websites, host) {
			return nil, fmt.Errorf("host %q is not in whitelist", host)
		}
	}

	collector := colly.NewCollector(
		colly.Async(true),
		colly.AllowedDomains(s.websites...),
		colly.CacheDir(s.cacheDir),
		colly.UserAgent(firefoxUserAgent),
	)
	if err := collector.Limits(buildLimitRules(s.websites, s.hostDelay)); err != nil {
		return nil, fmt.Errorf("configure rate limit: %w", err)
	}

	scrapeTime := s.now().UTC()
	savedPages := make([]SavedPage, 0)
	savedByURL := make(map[string]SavedPage)
	var crawlErrs []error
	var mu sync.Mutex

	collector.OnRequest(func(r *colly.Request) {
		s.logger.Info("requesting url", "url", r.URL.String())
	})

	collector.OnResponse(func(r *colly.Response) {
		s.logger.Info("received response", "url", r.Request.URL.String(), "status", r.StatusCode, "bytes", len(r.Body))

		savedPage, err := s.saveReadablePage(r.Request.URL.String(), r.Body, scrapeTime)
		if err != nil {
			s.logger.Error("could not save page", "url", r.Request.URL.String(), "err", err)
			mu.Lock()
			crawlErrs = append(crawlErrs, err)
			mu.Unlock()
			return
		}

		mu.Lock()
		if _, exists := savedByURL[savedPage.URL]; exists {
			mu.Unlock()
			return
		}

		savedByURL[savedPage.URL] = savedPage
		savedPages = append(savedPages, savedPage)
		mu.Unlock()
		s.logger.Info("saved page", "url", savedPage.URL, "page_key", savedPage.PageKey, "id", savedPage.ID)
	})

	collector.OnError(func(r *colly.Response, err error) {
		url := ""
		if r != nil && r.Request != nil {
			url = r.Request.URL.String()
		}
		if url == "" {
			url = "unknown"
		}
		if isHTMLStackLimitError(err) {
			s.logger.Warn("html parser hit stack limit during crawl, skipping link discovery", "url", url)
			return
		}
		s.logger.Error("request failed", "url", url, "err", err)
		mu.Lock()
		crawlErrs = append(crawlErrs, err)
		mu.Unlock()
	})

	collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		nextURL := e.Request.AbsoluteURL(e.Attr("href"))
		if nextURL == "" {
			return
		}
		nextHost := normalizeHost(mustHostname(nextURL))
		if !slices.Contains(s.websites, nextHost) {
			return
		}

		if err := e.Request.Visit(nextURL); err != nil {
			if isAlreadyVisitedError(err) {
				return
			}
			s.logger.Warn("could not follow url", "url", nextURL, "err", err)
		}
	})

	for _, rawURL := range rawURLs {
		s.logger.Info("starting crawl", "url", rawURL)
		if err := collector.Visit(rawURL); err != nil {
			mu.Lock()
			crawlErrs = append(crawlErrs, fmt.Errorf("visit url %s: %w", rawURL, err))
			mu.Unlock()
		}
	}
	collector.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(crawlErrs) > 0 {
		return savedPages, fmt.Errorf("crawl failed: %w", errors.Join(crawlErrs...))
	}

	return savedPages, nil
}

func buildLimitRules(hosts []string, delay time.Duration) []*colly.LimitRule {
	rules := make([]*colly.LimitRule, 0, len(hosts))
	for _, host := range hosts {
		rules = append(rules, &colly.LimitRule{
			DomainRegexp: "^" + regexp.QuoteMeta(host) + "(?::\\d+)?$",
			Delay:        delay,
		})
	}

	return rules
}

func normalizeConfig(cfg Config) (Config, error) {
	if len(cfg.Websites) == 0 {
		return Config{}, errors.New("config must include at least one website")
	}

	normalized := Config{
		CacheDir:  cfg.CacheDir,
		HostDelay: cfg.HostDelay,
		Websites:  make([]string, 0, len(cfg.Websites)),
	}

	if normalized.CacheDir == "" {
		normalized.CacheDir = ".cache/colly"
	}

	for _, website := range cfg.Websites {
		target, _, err := normalizeTarget(website)
		if err != nil {
			return Config{}, err
		}
		if !slices.Contains(normalized.Websites, target) {
			normalized.Websites = append(normalized.Websites, target)
		}
	}

	return normalized, nil
}

func normalizeTarget(raw string) (string, string, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", "", errors.New("config contains empty website entry")
	}

	if !strings.Contains(target, "://") {
		target = "https://" + target
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return "", "", fmt.Errorf("parse website target %q: %w", raw, err)
	}

	host := normalizeHost(parsed.Hostname())
	if parsed.Scheme == "" || host == "" {
		return "", "", fmt.Errorf("website target %q must include a valid host", raw)
	}

	return parsed.String(), host, nil
}

func normalizeHost(raw string) string {
	host := strings.TrimSpace(raw)
	if host == "" {
		return ""
	}

	if strings.Contains(host, "://") {
		if parsed, err := url.Parse(host); err == nil {
			host = parsed.Hostname()
		}
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	return strings.ToLower(strings.TrimSpace(host))
}

func mustHostname(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return parsed.Hostname()
}

func (s *Scraper) saveReadablePage(pageURL string, body []byte, scrapeTime time.Time) (SavedPage, error) {
	parsedURL, err := url.ParseRequestURI(pageURL)
	if err != nil {
		return SavedPage{}, fmt.Errorf("parse page url: %w", err)
	}

	parser := readability.NewParser()
	parser.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	article, err := parser.Parse(bytes.NewReader(body), parsedURL)
	parseMode := "readability"
	if err != nil {
		if isHTMLStackLimitError(err) {
			s.logger.Warn("readability parser hit html stack limit, using raw fallback", "url", pageURL)
			return s.savePageRecord(parsedURL, scrapeTime, safeRawHTMLFallback(body), "raw_fallback")
		}
		return SavedPage{}, fmt.Errorf("parse readability content: %w", err)
	}

	var content bytes.Buffer
	if err := article.RenderHTML(&content); err != nil {
		return SavedPage{}, fmt.Errorf("render cleaned html: %w", err)
	}

	return s.savePageRecord(parsedURL, scrapeTime, content.String(), parseMode)
}

func (s *Scraper) savePageRecord(parsedURL *url.URL, scrapeTime time.Time, content string, parseMode string) (SavedPage, error) {
	contentHash := hashContent(content)

	page := SavedPage{
		URL:         parsedURL.String(),
		Host:        parsedURL.Host,
		Path:        normalizedPagePath(parsedURL),
		PageKey:     buildPageKey(parsedURL, scrapeTime),
		ContentHash: contentHash,
		ParseMode:   parseMode,
		Content:     content,
		ScrapedAt:   scrapeTime,
	}

	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "url"}, {Name: "content_hash"}},
		DoNothing: true,
	}).Create(&page).Error; err != nil {
		return SavedPage{}, fmt.Errorf("insert page: %w", err)
	}

	if page.ID != 0 {
		return page, nil
	}

	var existing SavedPage
	if err := s.db.Where("url = ? AND content_hash = ?", page.URL, page.ContentHash).First(&existing).Error; err != nil {
		return SavedPage{}, fmt.Errorf("load existing page: %w", err)
	}

	return existing, nil
}

func isHTMLStackLimitError(err error) bool {
	return strings.Contains(err.Error(), "open stack of elements exceeds 512 nodes")
}

func isAlreadyVisitedError(err error) bool {
	return strings.Contains(err.Error(), "already visited")
}

func safeRawHTMLFallback(body []byte) string {
	return "<pre>" + html.EscapeString(string(body)) + "</pre>"
}

func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", sum)
}

func buildPageKey(pageURL *url.URL, scrapeTime time.Time) string {
	return url.QueryEscape(pageURL.Host+normalizedPagePath(pageURL)) + "_" + scrapeTime.Format("20060102T150405Z")
}

func normalizedPagePath(pageURL *url.URL) string {
	path := pageURL.EscapedPath()
	if path == "" {
		return "/"
	}
	if strings.HasSuffix(pageURL.Path, "/") && !strings.HasSuffix(path, "/") {
		return path + "/"
	}
	return path
}

func openDatabase(path string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database dir: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := db.AutoMigrate(&SavedPage{}); err != nil {
		return nil, fmt.Errorf("migrate sqlite database: %w", err)
	}

	return db, nil
}
