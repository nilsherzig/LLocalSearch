package scraper

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/gocolly/colly/v2"
	"github.com/nilsherzig/llocalsearch/embedding"
	"github.com/nilsherzig/llocalsearch/vectorstore"
	htmlnode "golang.org/x/net/html"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type Config struct {
	CacheDir         string           `yaml:"cache_dir"`
	HostDelay        Duration         `yaml:"host_delay"`
	AllowedLanguages []string         `yaml:"allowed_languages"`
	Embeddings       EmbeddingConfig  `yaml:"embeddings"`
	SummaryLLM       SummaryLLMConfig `yaml:"summary_llm"`
	Websites         []string         `yaml:"websites"`
}

type EmbeddingConfig struct {
	BaseURL        string   `yaml:"base_url"`
	Model          string   `yaml:"model"`
	Dimensions     int      `yaml:"dimensions"`
	Timeout        Duration `yaml:"timeout"`
	BatchSize      int      `yaml:"batch_size"`
	PageTokenLimit int      `yaml:"page_token_limit"`
}

type SummaryLLMConfig struct {
	Model   string   `yaml:"model"`
	Timeout Duration `yaml:"timeout"`
}

type SessionPageEvent struct {
	Page              SavedPage
	EmbeddingDuration time.Duration
	Reused            bool
}

type SessionFailureEvent struct {
	URL      string
	FailedAt time.Time
	Err      error
}

type SessionObserver interface {
	OnPageSaved(SessionPageEvent)
	OnRequestFailed(SessionFailureEvent)
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
	cacheDir         string
	dbPath           string
	websites         []string
	allowedLanguages []string
	logger           *slog.Logger
	now              func() time.Time
	hostDelay        time.Duration
	db               *gorm.DB
	store            *PageStore
	embedder         embedding.Client
	observer         SessionObserver
	saveMu           sync.Mutex
}

type SavedPage struct {
	ID             uint      `gorm:"primaryKey"`
	URL            string    `gorm:"not null;index;uniqueIndex:idx_saved_pages_url_hash"`
	Host           string    `gorm:"not null;index"`
	Path           string    `gorm:"not null"`
	Title          string    `gorm:"not null;default:''"`
	PageKey        string    `gorm:"not null;index"`
	ContentHash    string    `gorm:"not null;index;uniqueIndex:idx_saved_pages_url_hash"`
	ParseMode      string    `gorm:"not null;default:readability"`
	SourceType     string    `gorm:"not null;default:scraper;index"`
	SourceBrowser  string    `gorm:"not null;default:'';index"`
	SourceDeviceID string    `gorm:"not null;default:'';index"`
	Content        string    `gorm:"type:text;not null"`
	ScrapedAt      time.Time `gorm:"not null;index"`
	CapturedAt     time.Time `gorm:"not null;index"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

const firefoxUserAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:136.0) Gecko/20100101 Firefox/136.0"
const skippedLanguageContextKey = "skipped_language"

const (
	SourceTypeScraper          = "scraper"
	SourceTypeBrowserExtension = "browser_extension"
)

type PageMetadata struct {
	Title          string
	SourceType     string
	SourceBrowser  string
	SourceDeviceID string
	CapturedAt     time.Time
}

type HTMLCapture struct {
	URL            string
	HTML           []byte
	Title          string
	SourceType     string
	SourceBrowser  string
	SourceDeviceID string
	CapturedAt     time.Time
}

type SaveResult struct {
	Page   SavedPage
	Reused bool
}

type PageStore struct {
	db     *gorm.DB
	logger *slog.Logger
	saveMu sync.Mutex
}

var nonHTMLResourceExtensions = map[string]struct{}{
	".7z":   {},
	".avi":  {},
	".bmp":  {},
	".csv":  {},
	".doc":  {},
	".docx": {},
	".epub": {},
	".gif":  {},
	".gz":   {},
	".jpeg": {},
	".jpg":  {},
	".json": {},
	".m4a":  {},
	".mkv":  {},
	".mov":  {},
	".mp3":  {},
	".mp4":  {},
	".ods":  {},
	".odt":  {},
	".pdf":  {},
	".png":  {},
	".ppt":  {},
	".pptx": {},
	".rar":  {},
	".rss":  {},
	".svg":  {},
	".tar":  {},
	".tgz":  {},
	".tif":  {},
	".tiff": {},
	".txt":  {},
	".wav":  {},
	".webm": {},
	".webp": {},
	".xls":  {},
	".xlsx": {},
	".xml":  {},
	".zip":  {},
}

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

func WithEmbedder(embedder embedding.Client) Option {
	return func(s *Scraper) {
		if embedder != nil {
			s.embedder = embedder
		}
	}
}

func WithSessionObserver(observer SessionObserver) Option {
	return func(s *Scraper) {
		if observer != nil {
			s.observer = observer
		}
	}
}

type saveResult struct {
	SaveResult
	EmbeddingDuration time.Duration
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
		cacheDir:         normalized.CacheDir,
		dbPath:           filepath.Join(normalized.CacheDir, "pages.db"),
		websites:         allowedHosts,
		allowedLanguages: append([]string(nil), normalized.AllowedLanguages...),
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:              time.Now,
		hostDelay:        500 * time.Millisecond,
	}
	if normalized.HostDelay > 0 {
		s.hostDelay = time.Duration(normalized.HostDelay)
	}

	for _, opt := range opts {
		opt(s)
	}

	db, err := vectorstore.Open(s.dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&SavedPage{}); err != nil {
		return nil, fmt.Errorf("migrate sqlite database: %w", err)
	}
	s.db = db
	s.store = newPageStore(db, s.logger)
	if err := s.deletePagesForUnconfiguredHosts(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Scraper) Fetch(rawURL string) ([]SavedPage, error) {
	return s.FetchAll([]string{rawURL})
}

func (s *Scraper) FetchAll(rawURLs []string) ([]SavedPage, error) {
	if len(rawURLs) == 0 {
		return nil, nil
	}

	normalizedRawURLs := make([]string, 0, len(rawURLs))
	for _, rawURL := range rawURLs {
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("parse url: %w", err)
		}
		parsedURL = stripURLFragment(parsedURL)

		host := normalizeHost(parsedURL.Hostname())
		if !slices.Contains(s.websites, host) {
			return nil, fmt.Errorf("host %q is not in whitelist", host)
		}
		normalizedRawURLs = append(normalizedRawURLs, parsedURL.String())
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
		// s.logger.Info("requesting url", "url", r.URL.String())
	})

	collector.OnResponse(func(r *colly.Response) {
		// s.logger.Info("received response", "url", r.Request.URL.String(), "status", r.StatusCode, "bytes", len(r.Body))
		if !isHTMLResponse(r.Headers.Get("Content-Type"), r.Body) {
			s.logger.Info("skipping non-html response", "url", r.Request.URL.String(), "content_type", normalizedContentType(r.Headers.Get("Content-Type"), r.Body))
			return
		}
		if lang, ok := htmlDocumentLanguage(r.Body); ok && !matchesAllowedLanguageTag(lang, s.allowedLanguages) {
			r.Ctx.Put(skippedLanguageContextKey, lang)
			s.logger.Info("skipping disallowed html response language", "url", r.Request.URL.String(), "lang", lang)
			return
		}

		saveResult, err := s.saveReadablePage(r.Request.URL.String(), r.Body, scrapeTime)
		if err != nil {
			s.logger.Error("could not save page", "url", r.Request.URL.String(), "err", err)
			mu.Lock()
			crawlErrs = append(crawlErrs, err)
			mu.Unlock()
			return
		}

		mu.Lock()
		if _, exists := savedByURL[saveResult.Page.URL]; exists {
			mu.Unlock()
			return
		}

		savedByURL[saveResult.Page.URL] = saveResult.Page
		savedPages = append(savedPages, saveResult.Page)
		mu.Unlock()
		// s.logger.Info("saved page", "url", saveResult.Page.URL, "page_key", saveResult.Page.PageKey, "id", saveResult.Page.ID)
		if s.observer != nil {
			s.observer.OnPageSaved(SessionPageEvent{
				Page:              saveResult.Page,
				EmbeddingDuration: saveResult.EmbeddingDuration,
				Reused:            saveResult.SaveResult.Reused,
			})
		}
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
		if s.observer != nil {
			s.observer.OnRequestFailed(SessionFailureEvent{
				URL:      url,
				FailedAt: time.Now(),
				Err:      err,
			})
		}
		mu.Lock()
		crawlErrs = append(crawlErrs, err)
		mu.Unlock()
	})

	collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		if e.Request.Ctx.Get(skippedLanguageContextKey) != "" {
			return
		}

		nextURL := stripURLFragmentString(e.Request.AbsoluteURL(e.Attr("href")))
		if nextURL == "" {
			return
		}
		if !shouldVisitURL(nextURL) {
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

	for _, rawURL := range normalizedRawURLs {
		if !shouldVisitURL(rawURL) {
			s.logger.Info("skipping non-html url", "url", rawURL)
			continue
		}
		s.logger.Info("starting crawl", "url", rawURL)
		if err := collector.Visit(rawURL); err != nil {
			if s.observer != nil {
				s.observer.OnRequestFailed(SessionFailureEvent{
					URL:      rawURL,
					FailedAt: time.Now(),
					Err:      err,
				})
			}
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
		CacheDir:         cfg.CacheDir,
		HostDelay:        cfg.HostDelay,
		AllowedLanguages: nil,
		Embeddings:       cfg.Embeddings,
		SummaryLLM:       cfg.SummaryLLM,
		Websites:         make([]string, 0, len(cfg.Websites)),
	}

	if normalized.CacheDir == "" {
		normalized.CacheDir = ".cache/colly"
	}
	if normalized.Embeddings.Dimensions <= 0 {
		normalized.Embeddings.Dimensions = 2560
	}
	if normalized.Embeddings.Timeout <= 0 {
		normalized.Embeddings.Timeout = Duration(30 * time.Second)
	}
	if normalized.Embeddings.BatchSize <= 0 {
		normalized.Embeddings.BatchSize = 8
	}
	if normalized.Embeddings.PageTokenLimit <= 0 {
		normalized.Embeddings.PageTokenLimit = 3000
	}
	if normalized.SummaryLLM.Model == "" {
		normalized.SummaryLLM.Model = "qwen3.5:9b"
	}
	if normalized.SummaryLLM.Timeout <= 0 {
		normalized.SummaryLLM.Timeout = Duration(30 * time.Second)
	}
	if cfg.AllowedLanguages == nil {
		normalized.AllowedLanguages = []string{"en"}
	} else {
		normalized.AllowedLanguages = make([]string, 0, len(cfg.AllowedLanguages))
		for _, lang := range cfg.AllowedLanguages {
			normalizedLang := normalizeLanguageTag(lang)
			if normalizedLang == "" {
				return Config{}, errors.New("config contains empty allowed language entry")
			}
			if !slices.Contains(normalized.AllowedLanguages, normalizedLang) {
				normalized.AllowedLanguages = append(normalized.AllowedLanguages, normalizedLang)
			}
		}
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
	parsed = stripURLFragment(parsed)

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

func shouldVisitURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	parsed = stripURLFragment(parsed)
	if parsed.RawQuery != "" || parsed.ForceQuery {
		return false
	}

	ext := strings.ToLower(path.Ext(parsed.Path))
	if ext == "" {
		return true
	}

	_, blocked := nonHTMLResourceExtensions[ext]
	return !blocked
}

func isHTMLResponse(contentType string, body []byte) bool {
	normalized := normalizedContentType(contentType, body)
	return normalized == "text/html" || normalized == "application/xhtml+xml"
}

func normalizedContentType(contentType string, body []byte) string {
	value := strings.TrimSpace(contentType)
	if value == "" {
		value = http.DetectContentType(body)
	}

	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return strings.ToLower(value)
	}

	return strings.ToLower(mediaType)
}

func htmlDocumentLanguage(body []byte) (string, bool) {
	tokenizer := htmlnode.NewTokenizer(bytes.NewReader(body))
	for {
		switch tokenizer.Next() {
		case htmlnode.ErrorToken:
			return "", false
		case htmlnode.StartTagToken, htmlnode.SelfClosingTagToken:
			token := tokenizer.Token()
			if !strings.EqualFold(token.Data, "html") {
				continue
			}

			for _, attr := range token.Attr {
				if !strings.EqualFold(attr.Key, "lang") && !strings.EqualFold(attr.Key, "xml:lang") {
					continue
				}

				lang := normalizeLanguageTag(attr.Val)
				if lang == "" {
					return "", false
				}
				return lang, true
			}

			return "", false
		}
	}
}

func normalizeLanguageTag(lang string) string {
	return strings.ToLower(strings.TrimSpace(lang))
}

func matchesAllowedLanguageTag(lang string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}

	normalized := normalizeLanguageTag(lang)
	if normalized == "" {
		return false
	}

	for _, candidate := range allowed {
		allowedTag := normalizeLanguageTag(candidate)
		if allowedTag == "" {
			continue
		}
		if normalized == allowedTag ||
			strings.HasPrefix(normalized, allowedTag+"-") ||
			strings.HasPrefix(normalized, allowedTag+"_") {
			return true
		}
	}

	return false
}

func (s *Scraper) saveReadablePage(pageURL string, body []byte, scrapeTime time.Time) (saveResult, error) {
	parsedURL, err := parseAndValidatePageURL(pageURL)
	if err != nil {
		return saveResult{}, err
	}

	content, parseMode, err := parseReadableContent(s.logger, parsedURL, body)
	if err != nil {
		return saveResult{}, err
	}

	return s.savePageRecord(parsedURL, scrapeTime, content, parseMode)
}

func (s *Scraper) savePageRecord(parsedURL *url.URL, scrapeTime time.Time, content string, parseMode string) (saveResult, error) {
	result, err := s.store.savePageRecord(parsedURL, scrapeTime, content, parseMode, PageMetadata{})
	if err != nil {
		return saveResult{}, err
	}
	return saveResult{SaveResult: result}, nil
}

func (s *Scraper) logInfo(msg string, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Info(msg, args...)
}

func (s *Scraper) deletePagesForUnconfiguredHosts() error {
	var pages []SavedPage
	if err := s.db.Find(&pages).Error; err != nil {
		return fmt.Errorf("load saved pages for startup cleanup: %w", err)
	}

	staleIDs := make([]uint, 0)
	for _, page := range pages {
		if slices.Contains(s.websites, savedPageHost(page)) {
			continue
		}
		staleIDs = append(staleIDs, page.ID)
	}
	if len(staleIDs) == 0 {
		return nil
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := vectorstore.DeleteEmbeddings(tx, staleIDs); err != nil {
			return err
		}
		if err := tx.Delete(&SavedPage{}, staleIDs).Error; err != nil {
			return fmt.Errorf("delete pages for unconfigured hosts: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	s.logInfo("deleted pages for unconfigured hosts", "pages", len(staleIDs))
	return nil
}

func buildSavedPage(parsedURL *url.URL, scrapeTime time.Time, content string, parseMode string, metadata PageMetadata) SavedPage {
	metadata = normalizePageMetadata(scrapeTime, metadata)
	contentHash := hashContent(content)

	return SavedPage{
		URL:            parsedURL.String(),
		Host:           parsedURL.Host,
		Path:           normalizedPagePath(parsedURL),
		Title:          strings.TrimSpace(metadata.Title),
		PageKey:        buildPageKey(parsedURL, scrapeTime),
		ContentHash:    contentHash,
		ParseMode:      parseMode,
		SourceType:     metadata.SourceType,
		SourceBrowser:  strings.TrimSpace(metadata.SourceBrowser),
		SourceDeviceID: strings.TrimSpace(metadata.SourceDeviceID),
		Content:        content,
		ScrapedAt:      scrapeTime,
		CapturedAt:     metadata.CapturedAt.UTC(),
	}
}

func NewPageStore(db *gorm.DB) *PageStore {
	return newPageStore(db, nil)
}

func newPageStore(db *gorm.DB, logger *slog.Logger) *PageStore {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &PageStore{
		db:     db,
		logger: logger,
	}
}

func (s *PageStore) SaveHTMLCapture(capture HTMLCapture) (SaveResult, error) {
	if s == nil || s.db == nil {
		return SaveResult{}, fmt.Errorf("page store database is required")
	}

	parsedURL, err := parseAndValidatePageURL(capture.URL)
	if err != nil {
		return SaveResult{}, err
	}
	content, parseMode, err := parseReadableContent(s.logger, parsedURL, capture.HTML)
	if err != nil {
		return SaveResult{}, err
	}

	return s.savePageRecord(parsedURL, normalizeCaptureTime(capture.CapturedAt), content, parseMode, PageMetadata{
		Title:          capture.Title,
		SourceType:     capture.SourceType,
		SourceBrowser:  capture.SourceBrowser,
		SourceDeviceID: capture.SourceDeviceID,
		CapturedAt:     capture.CapturedAt,
	})
}

func (s *PageStore) savePageRecord(parsedURL *url.URL, scrapeTime time.Time, content string, parseMode string, metadata PageMetadata) (SaveResult, error) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	page := buildSavedPage(parsedURL, scrapeTime, content, parseMode, metadata)
	if existing, ok, err := s.findSavedPage(page.URL, page.ContentHash); err != nil {
		return SaveResult{}, err
	} else if ok {
		return SaveResult{Page: existing, Reused: true}, nil
	}

	var saved SavedPage
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var stalePages []SavedPage
		if err := tx.Where("url = ?", page.URL).Find(&stalePages).Error; err != nil {
			return fmt.Errorf("load stale page versions: %w", err)
		}

		staleIDs := make([]uint, 0, len(stalePages))
		for _, stalePage := range stalePages {
			staleIDs = append(staleIDs, stalePage.ID)
		}
		if err := vectorstore.DeleteEmbeddings(tx, staleIDs); err != nil {
			return err
		}
		if err := tx.Where("url = ?", page.URL).Delete(&SavedPage{}).Error; err != nil {
			return fmt.Errorf("delete old page versions: %w", err)
		}

		pageToSave := page
		if err := tx.Create(&pageToSave).Error; err != nil {
			return fmt.Errorf("insert page: %w", err)
		}
		saved = pageToSave
		return nil
	}); err != nil {
		return SaveResult{}, err
	}

	return SaveResult{Page: saved}, nil
}

func (s *PageStore) findSavedPage(pageURL string, contentHash string) (SavedPage, bool, error) {
	var existing SavedPage
	result := s.db.Where("url = ? AND content_hash = ?", pageURL, contentHash).Limit(1).Find(&existing)
	if result.Error != nil {
		return SavedPage{}, false, fmt.Errorf("load existing page: %w", result.Error)
	}
	return existing, result.RowsAffected > 0, nil
}

func parseAndValidatePageURL(rawURL string) (*url.URL, error) {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse page url: %w", err)
	}
	parsedURL = stripURLFragment(parsedURL)
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported page url scheme %q", parsedURL.Scheme)
	}
	if parsedURL.Host == "" {
		return nil, errors.New("page url must include a host")
	}
	return parsedURL, nil
}

func parseReadableContent(logger *slog.Logger, pageURL *url.URL, body []byte) (string, string, error) {
	parser := readability.NewParser()
	parser.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	article, err := parser.Parse(bytes.NewReader(body), pageURL)
	parseMode := "readability"
	if err != nil {
		if isHTMLStackLimitError(err) {
			if logger != nil {
				logger.Warn("readability parser hit html stack limit, using raw fallback", "url", pageURL.String())
			}
			return safeRawHTMLFallback(body), "raw_fallback", nil
		}
		return "", "", fmt.Errorf("parse readability content: %w", err)
	}

	var content bytes.Buffer
	if err := article.RenderHTML(&content); err != nil {
		return "", "", fmt.Errorf("render cleaned html: %w", err)
	}

	return content.String(), parseMode, nil
}

func normalizeCaptureTime(capturedAt time.Time) time.Time {
	if capturedAt.IsZero() {
		return time.Now().UTC()
	}
	return capturedAt.UTC()
}

func normalizePageMetadata(scrapeTime time.Time, metadata PageMetadata) PageMetadata {
	normalized := metadata
	normalized.Title = strings.TrimSpace(metadata.Title)
	normalized.SourceType = strings.TrimSpace(metadata.SourceType)
	normalized.SourceBrowser = strings.TrimSpace(metadata.SourceBrowser)
	normalized.SourceDeviceID = strings.TrimSpace(metadata.SourceDeviceID)
	if normalized.SourceType == "" {
		normalized.SourceType = SourceTypeScraper
	}
	if normalized.CapturedAt.IsZero() {
		normalized.CapturedAt = scrapeTime.UTC()
	} else {
		normalized.CapturedAt = normalized.CapturedAt.UTC()
	}
	return normalized
}

func cloneURL(source *url.URL) *url.URL {
	if source == nil {
		return nil
	}

	cloned := *source
	return &cloned
}

func stripURLFragment(source *url.URL) *url.URL {
	if source == nil {
		return nil
	}

	normalized := cloneURL(source)
	normalized.Fragment = ""
	return normalized
}

func stripURLFragmentString(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	return stripURLFragment(parsed).String()
}

func savedPageHost(page SavedPage) string {
	if host := normalizeHost(page.Host); host != "" {
		return host
	}
	return normalizeHost(mustHostname(page.URL))
}

func isHTMLStackLimitError(err error) bool {
	return strings.Contains(err.Error(), "open stack of elements exceeds 512 nodes")
}

func isAlreadyVisitedError(err error) bool {
	return strings.Contains(err.Error(), "already visited")
}

func safeRawHTMLFallback(body []byte) string {
	return "<pre>" + stdhtml.EscapeString(string(body)) + "</pre>"
}

func extractPlainText(content string) string {
	tokenizer := htmlnode.NewTokenizer(strings.NewReader(content))
	var builder strings.Builder
	for {
		switch tokenizer.Next() {
		case htmlnode.ErrorToken:
			text := strings.TrimSpace(strings.Join(strings.Fields(builder.String()), " "))
			if text == "" {
				return content
			}
			return text
		case htmlnode.TextToken:
			token := strings.TrimSpace(string(tokenizer.Text()))
			if token == "" {
				continue
			}
			if builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			builder.WriteString(token)
		}
	}
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
