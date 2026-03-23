package scraper

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoadConfigParsesWhitelistAndCacheDir(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configData := []byte("cache_dir: ./tmp/cache\nhost_delay: 25ms\nwebsites:\n  - example.com\n  - https://sub.example.org/start\n")

	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.CacheDir != "./tmp/cache" {
		t.Fatalf("unexpected cache dir %q", cfg.CacheDir)
	}
	if time.Duration(cfg.HostDelay) != 25*time.Millisecond {
		t.Fatalf("unexpected host delay %v", time.Duration(cfg.HostDelay))
	}

	wantWebsites := []string{"https://example.com", "https://sub.example.org/start"}
	if !slices.Equal(cfg.Websites, wantWebsites) {
		t.Fatalf("unexpected websites %v", cfg.Websites)
	}
}

func TestNewUsesHostDelayFromConfig(t *testing.T) {
	t.Parallel()

	s, err := New(Config{
		CacheDir:  t.TempDir(),
		HostDelay: Duration(75 * time.Millisecond),
		Websites:  []string{"https://example.com"},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if s.hostDelay != 75*time.Millisecond {
		t.Fatalf("unexpected host delay %v", s.hostDelay)
	}
}

func TestFetchRejectsURLOutsideWhitelist(t *testing.T) {
	t.Parallel()

	s, err := New(Config{
		CacheDir: t.TempDir(),
		Websites: []string{"https://allowed.example"},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = s.Fetch("https://blocked.example/path")
	if err == nil {
		t.Fatal("expected blocked domain error")
	}
}

func TestFetchAllStartsAllConfiguredTargets(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	firstRequests := 0
	firstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstRequests++
		_, _ = w.Write([]byte(`<html><body><article><p>first-target</p></article></body></html>`))
	}))
	defer firstServer.Close()

	secondRequests := 0
	secondServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondRequests++
		_, _ = w.Write([]byte(`<html><body><article><p>second-target</p></article></body></html>`))
	}))
	defer secondServer.Close()

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{firstServer.URL, secondServer.URL},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.FetchAll([]string{firstServer.URL, secondServer.URL})
	if err != nil {
		t.Fatalf("FetchAll returned error: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("expected two saved pages, got %d", len(pages))
	}
	if firstRequests != 1 {
		t.Fatalf("expected first target to be visited once, got %d", firstRequests)
	}
	if secondRequests != 1 {
		t.Fatalf("expected second target to be visited once, got %d", secondRequests)
	}
}

func TestFetchAllStartsDifferentSeedHostsConcurrently(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	delay := 100 * time.Millisecond

	started := make(chan string, 2)
	release := make(chan struct{})

	newBlockingServer := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started <- name
			<-release
			_, _ = w.Write([]byte(`<html><body><article><p>` + name + `</p></article></body></html>`))
		}))
	}

	firstServer := newBlockingServer("first")
	defer firstServer.Close()

	secondServer := newBlockingServer("second")
	defer secondServer.Close()

	secondURL, err := url.Parse(secondServer.URL)
	if err != nil {
		t.Fatalf("parse second server url: %v", err)
	}
	secondURL.Host = "localhost:" + secondURL.Port()

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{firstServer.URL, secondURL.String()},
	}, WithHostDelay(delay))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := s.FetchAll([]string{firstServer.URL, secondURL.String()})
		done <- err
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(150 * time.Millisecond):
			close(release)
			t.Fatal("expected both seed hosts to start before either finished")
		}
	}
	close(release)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("FetchAll returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("FetchAll did not complete")
	}
}

func TestFetchWaitsBetweenRequestsToSameHost(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	delay := 40 * time.Millisecond

	requestTimes := make([]time.Time, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestTimes = append(requestTimes, time.Now())

		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><article><p>root</p><a href="/next">next</a></article></body></html>`))
		case "/next":
			_, _ = w.Write([]byte(`<html><body><article><p>next</p></article></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{server.URL},
	}, WithHostDelay(delay))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := s.Fetch(server.URL); err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(requestTimes) != 2 {
		t.Fatalf("expected two requests, got %d", len(requestTimes))
	}

	gotDelay := requestTimes[1].Sub(requestTimes[0])
	if gotDelay < delay-(10*time.Millisecond) {
		t.Fatalf("expected at least %v delay between same-host requests, got %v", delay, gotDelay)
	}
}

func TestFetchUsesFirefoxUserAgent(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`<html><body><article><p>ua-check</p></article></body></html>`))
	}))
	defer server.Close()

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{server.URL},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := s.Fetch(server.URL); err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if !strings.Contains(userAgent, "Firefox/") {
		t.Fatalf("expected firefox user agent, got %q", userAgent)
	}
}

func TestFetchAllContinuesStartingSeedsAfterFirstSeedFails(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><article><p>bad</p></article></body></html>`))
	}))
	badURL := badServer.URL
	badServer.Close()

	secondRequests := 0
	secondServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondRequests++
		_, _ = w.Write([]byte(`<html><body><article><p>second-target</p></article></body></html>`))
	}))
	defer secondServer.Close()

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{badURL, secondServer.URL},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.FetchAll([]string{badURL, secondServer.URL})
	if err == nil {
		t.Fatal("expected scrape error for failing first seed")
	}
	if len(pages) != 1 {
		t.Fatalf("expected one successfully saved page, got %d", len(pages))
	}
	if secondRequests != 1 {
		t.Fatalf("expected second target to still be visited, got %d", secondRequests)
	}
	if !strings.Contains(pages[0].Content, "second-target") {
		t.Fatalf("expected successful second seed content, got %q", pages[0].Content)
	}
}

func TestFetchUsesCollyCacheDir(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	scrapeTime := time.Date(2026, 3, 23, 10, 11, 12, 0, time.UTC)

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`<html><body><article><h1>Cached response</h1><p>cached-response</p></article></body></html>`))
	}))

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{server.URL},
	}, WithNow(func() time.Time { return scrapeTime }))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("first Fetch returned error: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one saved page, got %d", len(pages))
	}

	dbPages, err := loadPagesFromDB(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load pages from db: %v", err)
	}
	if len(dbPages) != 1 {
		t.Fatalf("expected one page in db, got %d", len(dbPages))
	}
	if !strings.Contains(dbPages[0].Content, "cached-response") {
		t.Fatalf("unexpected saved page content %q", dbPages[0].Content)
	}

	server.Close()

	s, err = New(Config{
		CacheDir: cacheDir,
		Websites: []string{server.URL},
	}, WithNow(func() time.Time { return scrapeTime }))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err = s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("cached Fetch returned error: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one cached saved page, got %d", len(pages))
	}

	dbPages, err = loadPagesFromDB(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load cached pages from db: %v", err)
	}
	if len(dbPages) != 1 {
		t.Fatalf("expected one page in db after cached fetch, got %d", len(dbPages))
	}
	if !strings.Contains(dbPages[0].Content, "cached-response") {
		t.Fatalf("unexpected cached saved page content %q", dbPages[0].Content)
	}
	if dbPages[0].ContentHash == "" {
		t.Fatal("expected content hash to be stored")
	}

	if requests != 1 {
		t.Fatalf("expected exactly one network request, got %d", requests)
	}
}

func TestFetchDoesNotSaveSameURLAndHashAgainWithDifferentTimestamp(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><article><p>same-content</p></article></body></html>`))
	}))
	defer server.Close()

	firstTime := time.Date(2026, 3, 23, 10, 11, 12, 0, time.UTC)
	secondTime := firstTime.Add(2 * time.Hour)

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{server.URL},
	}, WithNow(func() time.Time { return firstTime }))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	firstPages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("first Fetch returned error: %v", err)
	}

	s, err = New(Config{
		CacheDir: cacheDir,
		Websites: []string{server.URL},
	}, WithNow(func() time.Time { return secondTime }))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	secondPages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("second Fetch returned error: %v", err)
	}

	dbPages, err := loadPagesFromDB(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load pages from db: %v", err)
	}
	if len(dbPages) != 1 {
		t.Fatalf("expected one page in db, got %d", len(dbPages))
	}
	if firstPages[0].ID != secondPages[0].ID {
		t.Fatalf("expected duplicate scrape to return existing row, got %d and %d", firstPages[0].ID, secondPages[0].ID)
	}
	if !dbPages[0].ScrapedAt.Equal(firstTime) {
		t.Fatalf("expected original scraped time to be preserved, got %v", dbPages[0].ScrapedAt)
	}
}

func TestFetchFollowsURLs(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	scrapeTime := time.Date(2026, 3, 23, 10, 11, 12, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><article><h1>Root</h1><p>root-page</p><a href="/next">next</a></article></body></html>`))
		case "/next":
			_, _ = w.Write([]byte(`<html><body><article><h1>Child</h1><p>child-page</p></article></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{server.URL},
	}, WithNow(func() time.Time { return scrapeTime }))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("expected two saved pages, got %d", len(pages))
	}

	joined := ""
	for _, page := range pages {
		joined += page.Content
	}
	if !strings.Contains(joined, "root-page") {
		t.Fatalf("expected root page in saved content, got %q", joined)
	}
	if !strings.Contains(joined, "child-page") {
		t.Fatalf("expected followed page in saved content, got %q", joined)
	}
}

func TestFetchLogsActionsWithoutHTMLContent(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	htmlBody := `<html><body>TOP-SECRET-HTML</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(htmlBody))
	}))
	defer server.Close()

	var logOutput bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))

	s, err := New(
		Config{
			CacheDir: cacheDir,
			Websites: []string{server.URL},
		},
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := s.Fetch(server.URL); err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	logs := logOutput.String()
	if !strings.Contains(logs, server.URL) {
		t.Fatalf("expected logs to mention target url, got %q", logs)
	}
	if strings.Contains(logs, "TOP-SECRET-HTML") {
		t.Fatalf("logs should not contain html body, got %q", logs)
	}
}

func TestFetchSavesCleanPageUsingEscapedURLAndTimestamp(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	scrapeTime := time.Date(2026, 3, 23, 10, 11, 12, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><article><h1>Title</h1><p>clean-body</p></article></body></html>`))
	}))
	defer server.Close()

	s, err := New(
		Config{
			CacheDir: cacheDir,
			Websites: []string{server.URL},
		},
		WithNow(func() time.Time { return scrapeTime }),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one saved page, got %d", len(pages))
	}

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	expectedPageKey := url.QueryEscape(parsedURL.Host+parsedURL.EscapedPath()+"/") + "_" + scrapeTime.UTC().Format("20060102T150405Z")
	if pages[0].PageKey != expectedPageKey {
		t.Fatalf("unexpected page key %q", pages[0].PageKey)
	}

	dbPages, err := loadPagesFromDB(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load pages from db: %v", err)
	}
	if len(dbPages) != 1 {
		t.Fatalf("expected one page in db, got %d", len(dbPages))
	}
	if !strings.Contains(dbPages[0].Content, "clean-body") {
		t.Fatalf("expected clean content in saved page, got %q", dbPages[0].Content)
	}
	if strings.Contains(dbPages[0].Content, "<script") {
		t.Fatalf("expected cleaned content without scripts, got %q", dbPages[0].Content)
	}
	if !dbPages[0].ScrapedAt.Equal(scrapeTime.UTC()) {
		t.Fatalf("unexpected scraped time %v", dbPages[0].ScrapedAt)
	}
}

func TestFetchFallsBackWhenReadabilityParserHitsOpenStackLimit(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(deeplyNestedHTML(600)))
	}))
	defer server.Close()

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{server.URL},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one saved page, got %d", len(pages))
	}
	if pages[0].ParseMode != "raw_fallback" {
		t.Fatalf("expected raw fallback parse mode, got %q", pages[0].ParseMode)
	}
	if !strings.Contains(pages[0].Content, "deep-content") {
		t.Fatalf("expected fallback content to include page body, got %q", pages[0].Content)
	}
	if !strings.Contains(pages[0].Content, "<pre") {
		t.Fatalf("expected fallback content to be wrapped safely, got %q", pages[0].Content)
	}
}

func loadPagesFromDB(path string) ([]SavedPage, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	var pages []SavedPage
	if err := db.Order("url asc").Find(&pages).Error; err != nil {
		return nil, err
	}

	return pages, nil
}

func deeplyNestedHTML(depth int) string {
	var builder strings.Builder
	builder.WriteString("<html><body>")
	for range depth {
		builder.WriteString("<div>")
	}
	builder.WriteString("deep-content")
	for range depth {
		builder.WriteString("</div>")
	}
	builder.WriteString("</body></html>")
	return builder.String()
}
