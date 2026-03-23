package scraper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	stdhtml "html"
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
	"github.com/nilsherzig/llocalsearch/embedding"
	"github.com/nilsherzig/llocalsearch/vectorstore"
	htmlnode "golang.org/x/net/html"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type Config struct {
	CacheDir   string          `yaml:"cache_dir"`
	HostDelay  Duration        `yaml:"host_delay"`
	Embeddings EmbeddingConfig `yaml:"embeddings"`
	Websites   []string        `yaml:"websites"`
}

type EmbeddingConfig struct {
	BaseURL    string   `yaml:"base_url"`
	Model      string   `yaml:"model"`
	Dimensions int      `yaml:"dimensions"`
	Timeout    Duration `yaml:"timeout"`
	QueueSize  int      `yaml:"queue_size"`
	BatchSize  int      `yaml:"batch_size"`
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
	cacheDir  string
	dbPath    string
	websites  []string
	logger    *slog.Logger
	now       func() time.Time
	hostDelay time.Duration
	db        *gorm.DB
	embedder  embedding.Client
	observer  SessionObserver
	saveQueue chan saveRequest

	embeddingBatchSize int
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
	Page              SavedPage
	EmbeddingDuration time.Duration
	Reused            bool
}

type saveRequest struct {
	parsedURL  *url.URL
	scrapeTime time.Time
	content    string
	parseMode  string
	result     chan saveResponse
}

type saveResponse struct {
	result saveResult
	err    error
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

	if s.embedder == nil && normalized.Embeddings.BaseURL != "" && normalized.Embeddings.Model != "" {
		s.embedder = embedding.NewClient(embedding.Config{
			BaseURL:    normalized.Embeddings.BaseURL,
			Model:      normalized.Embeddings.Model,
			Dimensions: normalized.Embeddings.Dimensions,
			Timeout:    time.Duration(normalized.Embeddings.Timeout),
		}, nil)
	}

	db, err := vectorstore.Open(s.dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&SavedPage{}); err != nil {
		return nil, fmt.Errorf("migrate sqlite database: %w", err)
	}
	if err := vectorstore.EnsureSchema(db, normalized.Embeddings.Model, normalized.Embeddings.Dimensions); err != nil {
		return nil, err
	}
	s.db = db
	s.saveQueue = make(chan saveRequest, normalized.Embeddings.QueueSize)
	s.embeddingBatchSize = normalized.Embeddings.BatchSize
	go s.runSaveWorker()

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
		s.logger.Info("saved page", "url", saveResult.Page.URL, "page_key", saveResult.Page.PageKey, "id", saveResult.Page.ID)
		if s.observer != nil {
			s.observer.OnPageSaved(SessionPageEvent{
				Page:              saveResult.Page,
				EmbeddingDuration: saveResult.EmbeddingDuration,
				Reused:            saveResult.Reused,
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
		CacheDir:   cfg.CacheDir,
		HostDelay:  cfg.HostDelay,
		Embeddings: cfg.Embeddings,
		Websites:   make([]string, 0, len(cfg.Websites)),
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
	if normalized.Embeddings.QueueSize <= 0 {
		normalized.Embeddings.QueueSize = 32
	}
	if normalized.Embeddings.BatchSize <= 0 {
		normalized.Embeddings.BatchSize = 8
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

func (s *Scraper) saveReadablePage(pageURL string, body []byte, scrapeTime time.Time) (saveResult, error) {
	parsedURL, err := url.ParseRequestURI(pageURL)
	if err != nil {
		return saveResult{}, fmt.Errorf("parse page url: %w", err)
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
		return saveResult{}, fmt.Errorf("parse readability content: %w", err)
	}

	var content bytes.Buffer
	if err := article.RenderHTML(&content); err != nil {
		return saveResult{}, fmt.Errorf("render cleaned html: %w", err)
	}

	return s.savePageRecord(parsedURL, scrapeTime, content.String(), parseMode)
}

func (s *Scraper) savePageRecord(parsedURL *url.URL, scrapeTime time.Time, content string, parseMode string) (saveResult, error) {
	responseCh := make(chan saveResponse, 1)
	s.saveQueue <- saveRequest{
		parsedURL:  cloneURL(parsedURL),
		scrapeTime: scrapeTime,
		content:    content,
		parseMode:  parseMode,
		result:     responseCh,
	}
	response := <-responseCh
	return response.result, response.err
}

func (s *Scraper) runSaveWorker() {
	for request := range s.saveQueue {
		s.processSaveBatch(s.collectSaveBatch(request))
	}
}

func (s *Scraper) collectSaveBatch(first saveRequest) []saveRequest {
	batchSize := s.embeddingBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}

	batch := []saveRequest{first}
	for len(batch) < batchSize {
		select {
		case request, ok := <-s.saveQueue:
			if !ok {
				return batch
			}
			batch = append(batch, request)
		default:
			return batch
		}
	}

	return batch
}

type preparedSave struct {
	request saveRequest
	page    SavedPage
	text    string
}

func (s *Scraper) processSaveBatch(batch []saveRequest) {
	if len(batch) == 0 {
		return
	}

	prepared := make([]preparedSave, 0, len(batch))
	inputs := make([]string, 0, len(batch))
	for _, request := range batch {
		pending, existingResult, err := s.prepareSaveRequest(request)
		if err != nil {
			request.result <- saveResponse{err: err}
			continue
		}
		if existingResult != nil {
			request.result <- saveResponse{result: *existingResult}
			continue
		}

		prepared = append(prepared, pending)
		inputs = append(inputs, pending.text)
	}

	if len(prepared) == 0 {
		if len(batch) > 0 && len(batch) != len(prepared) {
			s.logInfo("embedding batch skipped", "queued", len(batch), "reused", len(batch)-len(prepared))
		}
		return
	}

	s.logInfo("embedding batch started", "queued", len(batch), "to_embed", len(prepared), "reused", len(batch)-len(prepared))

	if s.embedder == nil {
		err := fmt.Errorf("embedder not configured")
		s.logError("embedding batch failed", "queued", len(batch), "to_embed", len(prepared), "err", err)
		for _, pending := range prepared {
			pending.request.result <- saveResponse{err: err}
		}
		return
	}

	embeddingStart := time.Now()
	vectors, err := s.embedder.Embed(context.Background(), inputs)
	if err != nil {
		wrapped := fmt.Errorf("embed page content: %w", err)
		s.logError("embedding batch failed", "queued", len(batch), "to_embed", len(prepared), "err", wrapped)
		for _, pending := range prepared {
			pending.request.result <- saveResponse{err: wrapped}
		}
		return
	}
	embeddingDuration := time.Since(embeddingStart)
	if len(vectors) != len(prepared) {
		err := fmt.Errorf("embed page content returned %d vectors", len(vectors))
		s.logError("embedding batch failed", "queued", len(batch), "to_embed", len(prepared), "err", err)
		for _, pending := range prepared {
			pending.request.result <- saveResponse{err: err}
		}
		return
	}
	s.logInfo("embedding batch finished", "queued", len(batch), "to_embed", len(prepared), "duration", embeddingDuration)

	for i, pending := range prepared {
		result, err := s.persistPreparedPage(pending.page, vectors[i], embeddingDuration)
		pending.request.result <- saveResponse{result: result, err: err}
	}
}

func (s *Scraper) logInfo(msg string, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Info(msg, args...)
}

func (s *Scraper) logError(msg string, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Error(msg, args...)
}

func (s *Scraper) prepareSaveRequest(request saveRequest) (preparedSave, *saveResult, error) {
	page := buildSavedPage(request.parsedURL, request.scrapeTime, request.content, request.parseMode)

	if existing, ok, err := s.findSavedPage(page.URL, page.ContentHash); err != nil {
		return preparedSave{}, nil, err
	} else if ok {
		result := saveResult{Page: existing, Reused: true}
		return preparedSave{}, &result, nil
	}

	return preparedSave{
		request: request,
		page:    page,
		text:    extractPlainText(request.content),
	}, nil, nil
}

func (s *Scraper) persistPreparedPage(page SavedPage, vector []float32, embeddingDuration time.Duration) (saveResult, error) {
	var saved SavedPage
	if len(vector) == 0 {
		return saveResult{}, fmt.Errorf("embed page content returned empty vector")
	}

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
		if err := vectorstore.UpsertEmbedding(tx, pageToSave.ID, vector); err != nil {
			return err
		}
		saved = pageToSave
		return nil
	}); err != nil {
		return saveResult{}, err
	}

	return saveResult{Page: saved, EmbeddingDuration: embeddingDuration}, nil
}

func buildSavedPage(parsedURL *url.URL, scrapeTime time.Time, content string, parseMode string) SavedPage {
	contentHash := hashContent(content)

	return SavedPage{
		URL:         parsedURL.String(),
		Host:        parsedURL.Host,
		Path:        normalizedPagePath(parsedURL),
		PageKey:     buildPageKey(parsedURL, scrapeTime),
		ContentHash: contentHash,
		ParseMode:   parseMode,
		Content:     content,
		ScrapedAt:   scrapeTime,
	}
}

func cloneURL(source *url.URL) *url.URL {
	if source == nil {
		return nil
	}

	cloned := *source
	return &cloned
}

func (s *Scraper) findSavedPage(pageURL string, contentHash string) (SavedPage, bool, error) {
	var existing SavedPage
	result := s.db.Where("url = ? AND content_hash = ?", pageURL, contentHash).Limit(1).Find(&existing)
	if result.Error != nil {
		return SavedPage{}, false, fmt.Errorf("load existing page: %w", result.Error)
	}
	return existing, result.RowsAffected > 0, nil
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
