package scraper

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoadConfigParsesWhitelistAndCacheDir(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configData := []byte("cache_dir: ./tmp/cache\nhost_delay: 25ms\nallowed_languages:\n  - en\n  - de\nembeddings:\n  base_url: http://localhost:11434\n  model: qwen3-embedding\n  dimensions: 2560\n  timeout: 3s\n  queue_size: 7\n  batch_size: 5\n  page_token_limit: 1234\nwebsites:\n  - example.com\n  - https://sub.example.org/start\n")

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
	if cfg.Embeddings.BaseURL != "http://localhost:11434" {
		t.Fatalf("unexpected embedding base url %q", cfg.Embeddings.BaseURL)
	}
	if cfg.Embeddings.Model != "qwen3-embedding" {
		t.Fatalf("unexpected embedding model %q", cfg.Embeddings.Model)
	}
	if cfg.Embeddings.Dimensions != 2560 {
		t.Fatalf("unexpected embedding dimensions %d", cfg.Embeddings.Dimensions)
	}
	if time.Duration(cfg.Embeddings.Timeout) != 3*time.Second {
		t.Fatalf("unexpected embedding timeout %v", time.Duration(cfg.Embeddings.Timeout))
	}
	if cfg.Embeddings.QueueSize != 7 {
		t.Fatalf("unexpected embedding queue size %d", cfg.Embeddings.QueueSize)
	}
	if cfg.Embeddings.BatchSize != 5 {
		t.Fatalf("unexpected embedding batch size %d", cfg.Embeddings.BatchSize)
	}
	if cfg.Embeddings.PageTokenLimit != 1234 {
		t.Fatalf("unexpected embedding page token limit %d", cfg.Embeddings.PageTokenLimit)
	}

	wantWebsites := []string{"https://example.com", "https://sub.example.org/start"}
	if !slices.Equal(cfg.Websites, wantWebsites) {
		t.Fatalf("unexpected websites %v", cfg.Websites)
	}
	if !slices.Equal(cfg.AllowedLanguages, []string{"en", "de"}) {
		t.Fatalf("unexpected allowed languages %v", cfg.AllowedLanguages)
	}
}

func TestNormalizeConfigDefaultsEmbeddingWorkerSettings(t *testing.T) {
	t.Parallel()

	cfg, err := normalizeConfig(Config{
		CacheDir: t.TempDir(),
		Websites: []string{"https://example.com"},
	})
	if err != nil {
		t.Fatalf("normalizeConfig returned error: %v", err)
	}

	if cfg.Embeddings.QueueSize != 32 {
		t.Fatalf("expected default queue size 32, got %d", cfg.Embeddings.QueueSize)
	}
	if cfg.Embeddings.Dimensions != 2560 {
		t.Fatalf("expected default embedding dimensions 2560, got %d", cfg.Embeddings.Dimensions)
	}
	if cfg.Embeddings.BatchSize != 8 {
		t.Fatalf("expected default batch size 8, got %d", cfg.Embeddings.BatchSize)
	}
	if cfg.Embeddings.PageTokenLimit != 3000 {
		t.Fatalf("expected default page token limit 3000, got %d", cfg.Embeddings.PageTokenLimit)
	}
	if !slices.Equal(cfg.AllowedLanguages, []string{"en"}) {
		t.Fatalf("expected default allowed languages [en], got %v", cfg.AllowedLanguages)
	}
}

func TestNormalizeConfigPreservesExplicitlyDisabledLanguageFilter(t *testing.T) {
	t.Parallel()

	cfg, err := normalizeConfig(Config{
		CacheDir:         t.TempDir(),
		AllowedLanguages: []string{},
		Websites:         []string{"https://example.com"},
	})
	if err != nil {
		t.Fatalf("normalizeConfig returned error: %v", err)
	}

	if cfg.AllowedLanguages == nil {
		t.Fatal("expected explicit empty language filter to be preserved")
	}
	if len(cfg.AllowedLanguages) != 0 {
		t.Fatalf("expected disabled language filter, got %v", cfg.AllowedLanguages)
	}
}

func TestNormalizeConfigStripsURLFragmentsFromWebsiteTargets(t *testing.T) {
	t.Parallel()

	cfg, err := normalizeConfig(Config{
		CacheDir: t.TempDir(),
		Websites: []string{"https://docs.k3s.io/cli/token#token-format"},
	})
	if err != nil {
		t.Fatalf("normalizeConfig returned error: %v", err)
	}

	if !slices.Equal(cfg.Websites, []string{"https://docs.k3s.io/cli/token"}) {
		t.Fatalf("expected fragment-free website target, got %v", cfg.Websites)
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

func TestNewDeletesPagesForHostsRemovedFromConfig(t *testing.T) {
	t.Parallel()

	cacheDir := filepath.Join(t.TempDir(), "cache")

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{"https://keep.example", "https://remove.example"},
	}, WithEmbedder(staticEmbedder{
		vectors: [][]float32{
			testVectorWithLead(0.1, 0.2, 0.3),
			testVectorWithLead(0.4, 0.5, 0.6),
		},
	}))
	if err != nil {
		t.Fatalf("first New returned error: %v", err)
	}

	keepURL, err := url.Parse("https://keep.example/guide")
	if err != nil {
		t.Fatalf("parse keep url: %v", err)
	}
	removeURL, err := url.Parse("https://remove.example/manual")
	if err != nil {
		t.Fatalf("parse remove url: %v", err)
	}

	keepResult, err := s.savePageRecord(keepURL, time.Date(2026, 3, 23, 10, 11, 12, 0, time.UTC), "<article><p>keep</p></article>", "readability")
	if err != nil {
		t.Fatalf("save keep page: %v", err)
	}
	removeResult, err := s.savePageRecord(removeURL, time.Date(2026, 3, 23, 10, 12, 12, 0, time.UTC), "<article><p>remove</p></article>", "readability")
	if err != nil {
		t.Fatalf("save removed page: %v", err)
	}

	_, err = New(Config{
		CacheDir: cacheDir,
		Websites: []string{"https://keep.example"},
	}, WithEmbedder(staticEmbedder{}))
	if err != nil {
		t.Fatalf("second New returned error: %v", err)
	}

	pages, err := loadPagesFromDB(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load pages from db: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one page after startup cleanup, got %d", len(pages))
	}
	if pages[0].ID != keepResult.Page.ID {
		t.Fatalf("expected kept page id %d, got %d", keepResult.Page.ID, pages[0].ID)
	}
	if pages[0].ID == removeResult.Page.ID {
		t.Fatalf("expected removed page id %d to be deleted", removeResult.Page.ID)
	}
	if pages[0].URL != keepURL.String() {
		t.Fatalf("expected kept page url %q, got %q", keepURL.String(), pages[0].URL)
	}

	vectorRowIDs, err := loadVectorRowIDs(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load vector row ids: %v", err)
	}
	if len(vectorRowIDs) != 0 {
		t.Fatalf("expected no embedding rows after startup cleanup, got %d", len(vectorRowIDs))
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
	}, WithEmbedder(staticEmbedder{}))
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
	}, WithHostDelay(delay), WithEmbedder(staticEmbedder{}))
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
	}, WithHostDelay(delay), WithEmbedder(staticEmbedder{}))
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

func TestFetchDoesNotFollowAnchorLinksAsSeparatePages(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	requestsByPath := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsByPath[r.URL.Path]++

		switch r.URL.Path {
		case "/cli/token":
			_, _ = w.Write([]byte(`<html><body><article><p>token page</p><a href="#token-format">anchor</a></article></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	startURL := server.URL + "/cli/token"
	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{startURL},
	}, WithEmbedder(staticEmbedder{}))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(startURL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one saved page, got %d", len(pages))
	}
	if pages[0].URL != startURL {
		t.Fatalf("expected saved url %q, got %q", startURL, pages[0].URL)
	}

	dbPages, err := loadPagesFromDB(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load pages from db: %v", err)
	}
	if len(dbPages) != 1 {
		t.Fatalf("expected one stored page, got %d", len(dbPages))
	}
	if got := requestsByPath["/cli/token"]; got != 1 {
		t.Fatalf("expected anchor link not to trigger another request, got %d requests", got)
	}
}

func TestFetchRespectsHostDelayWhileFollowingManyDiscoveredLinks(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	delay := 25 * time.Millisecond
	pageCount := 10

	var (
		mu           sync.Mutex
		requestTimes []time.Time
	)

	pageLinks := make(map[string][]string, pageCount)
	for i := 0; i < pageCount; i++ {
		path := fmt.Sprintf("/page-%d", i)
		pageLinks[path] = randomNextPaths(path, pageCount)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestTimes = append(requestTimes, time.Now())
		mu.Unlock()

		links, ok := pageLinks[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}

		var body strings.Builder
		body.WriteString(`<html><body><article><h1>`)
		body.WriteString(r.URL.Path)
		body.WriteString(`</h1><p>content</p>`)
		for _, link := range links {
			body.WriteString(`<a href="`)
			body.WriteString(link)
			body.WriteString(`">next</a>`)
		}
		body.WriteString(`</article></body></html>`)

		_, _ = w.Write([]byte(body.String()))
	}))
	defer server.Close()

	startURL := server.URL + "/page-0"
	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{startURL},
	}, WithHostDelay(delay), WithEmbedder(staticEmbedder{}))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(startURL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(pages) < 6 {
		t.Fatalf("expected crawler to follow several discovered pages, got %d", len(pages))
	}

	mu.Lock()
	gotTimes := append([]time.Time(nil), requestTimes...)
	mu.Unlock()

	if len(gotTimes) < 6 {
		t.Fatalf("expected at least six requests, got %d", len(gotTimes))
	}

	minGap := delay - (8 * time.Millisecond)
	for i := 1; i < len(gotTimes); i++ {
		gap := gotTimes[i].Sub(gotTimes[i-1])
		if gap < minGap {
			t.Fatalf("expected at least %v between request %d and %d, got %v", minGap, i-1, i, gap)
		}
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
	}, WithEmbedder(staticEmbedder{}))
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
	}, WithEmbedder(staticEmbedder{}))
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
	}, WithNow(func() time.Time { return scrapeTime }), WithEmbedder(staticEmbedder{}))
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
	}, WithNow(func() time.Time { return scrapeTime }), WithEmbedder(staticEmbedder{}))
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
	}, WithNow(func() time.Time { return firstTime }), WithEmbedder(staticEmbedder{}))
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
	}, WithNow(func() time.Time { return secondTime }), WithEmbedder(staticEmbedder{}))
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

func TestSavePageRecordReplacesOlderSavedVersionWhenContentHashChanges(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{"https://example.com"},
	}, WithEmbedder(staticEmbedder{vectors: [][]float32{testVectorWithLead(0.1, 0.2, 0.3)}}))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pageURL, err := url.Parse("https://example.com/article")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	firstTime := time.Date(2026, 3, 23, 10, 11, 12, 0, time.UTC)
	firstResult, err := s.savePageRecord(pageURL, firstTime, "<article><p>old version</p></article>", "readability")
	if err != nil {
		t.Fatalf("first savePageRecord returned error: %v", err)
	}
	firstPage := firstResult.Page

	secondTime := firstTime.Add(2 * time.Hour)
	secondResult, err := s.savePageRecord(pageURL, secondTime, "<article><p>new version</p></article>", "readability")
	if err != nil {
		t.Fatalf("second savePageRecord returned error: %v", err)
	}
	secondPage := secondResult.Page

	dbPages, err := loadPagesFromDB(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load pages from db: %v", err)
	}
	if len(dbPages) != 1 {
		t.Fatalf("expected one page in db after replacement, got %d", len(dbPages))
	}
	if dbPages[0].ID != secondPage.ID {
		t.Fatalf("expected latest page id %d, got %d", secondPage.ID, dbPages[0].ID)
	}
	if dbPages[0].ID == firstPage.ID {
		t.Fatalf("expected old page id %d to be replaced", firstPage.ID)
	}
	if !strings.Contains(dbPages[0].Content, "new version") {
		t.Fatalf("expected replacement content in db, got %q", dbPages[0].Content)
	}
	if dbPages[0].ContentHash == firstPage.ContentHash {
		t.Fatalf("expected content hash to change after replacement")
	}
	if !dbPages[0].ScrapedAt.Equal(secondTime) {
		t.Fatalf("expected latest scraped time %v, got %v", secondTime, dbPages[0].ScrapedAt)
	}

	vectorRowIDs, err := loadVectorRowIDs(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load vector row ids: %v", err)
	}
	if len(vectorRowIDs) != 0 {
		t.Fatalf("expected no embedding rows after replacement, got %d", len(vectorRowIDs))
	}
}

func TestSavePageRecordDoesNotStoreEmbeddingVector(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{"https://example.com"},
	}, WithEmbedder(staticEmbedder{vectors: [][]float32{testVectorWithLead(0.9, 0.1, 0.2)}}))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pageURL, err := url.Parse("https://example.com/article")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	_, err = s.savePageRecord(pageURL, time.Date(2026, 3, 23, 10, 11, 12, 0, time.UTC), "<article><p>vector body</p></article>", "readability")
	if err != nil {
		t.Fatalf("savePageRecord returned error: %v", err)
	}

	vectorRowIDs, err := loadVectorRowIDs(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load vector row ids: %v", err)
	}
	if len(vectorRowIDs) != 0 {
		t.Fatalf("expected no vector rows, got %d", len(vectorRowIDs))
	}
}

func TestSavePageRecordPersistsPageWithoutEmbedding(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{"https://example.com"},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pageURL, err := url.Parse("https://example.com/article")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	result, err := s.savePageRecord(pageURL, time.Date(2026, 3, 23, 10, 11, 12, 0, time.UTC), "<article><p>vector body</p></article>", "readability")
	if err != nil {
		t.Fatalf("savePageRecord returned error: %v", err)
	}

	dbPages, err := loadPagesFromDB(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load pages from db: %v", err)
	}
	if len(dbPages) != 1 {
		t.Fatalf("expected one saved page, got %d", len(dbPages))
	}
	if dbPages[0].ID != result.Page.ID {
		t.Fatalf("expected saved page id %d, got %d", result.Page.ID, dbPages[0].ID)
	}

	vectorRowIDs, err := loadVectorRowIDs(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load vector row ids: %v", err)
	}
	if len(vectorRowIDs) != 0 {
		t.Fatalf("expected no embedding rows, got %d", len(vectorRowIDs))
	}
}

func TestFetchNotifiesSessionObserverOnSavedPage(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	observer := &recordingSessionObserver{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><article><p>observer body</p></article></body></html>`))
	}))
	defer server.Close()

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{server.URL},
	}, WithEmbedder(staticEmbedder{vectors: [][]float32{testVectorWithLead(0.3, 0.2, 0.1)}}), WithSessionObserver(observer))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if len(observer.events) != 1 {
		t.Fatalf("expected one observer event, got %d", len(observer.events))
	}
	if observer.events[0].Page.ID != pages[0].ID {
		t.Fatalf("expected observer page id %d, got %d", pages[0].ID, observer.events[0].Page.ID)
	}
	if observer.events[0].Reused {
		t.Fatal("expected first observer event to be non-reused")
	}
}

func TestFetchAllNotifiesSessionObserverOnRequestFailure(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	observer := &recordingSessionObserver{}

	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><article><p>bad</p></article></body></html>`))
	}))
	badURL := badServer.URL
	badServer.Close()

	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><article><p>good</p></article></body></html>`))
	}))
	defer goodServer.Close()

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{badURL, goodServer.URL},
	}, WithEmbedder(staticEmbedder{}), WithSessionObserver(observer))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := s.FetchAll([]string{badURL, goodServer.URL}); err == nil {
		t.Fatal("expected crawl error")
	}

	if len(observer.failures) == 0 {
		t.Fatal("expected observer to record at least one failure")
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
	}, WithNow(func() time.Time { return scrapeTime }), WithEmbedder(staticEmbedder{}))
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

func TestFetchOnlyFollowsHTMLLinks(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	pdfRequests := 0
	htmlRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><article><p>root-page</p><a href="/next.html">next</a><a href="/manual.pdf">manual</a></article></body></html>`))
		case "/next.html":
			htmlRequests++
			_, _ = w.Write([]byte(`<html><body><article><p>child-page</p></article></body></html>`))
		case "/manual.pdf":
			pdfRequests++
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.4 fake pdf"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{server.URL},
	}, WithEmbedder(staticEmbedder{}))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("expected only html pages to be saved, got %d", len(pages))
	}
	if htmlRequests != 1 {
		t.Fatalf("expected html child to be requested once, got %d", htmlRequests)
	}
	if pdfRequests != 0 {
		t.Fatalf("expected pdf link not to be requested, got %d", pdfRequests)
	}
}

func TestFetchDoesNotFollowLinksWithQueryParameters(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	queryRequests := 0
	cleanRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/" && r.URL.RawQuery == "":
			_, _ = w.Write([]byte(`<html><body><article><p>root-page</p><a href="/next?login=1">query-link</a><a href="/clean">clean-link</a></article></body></html>`))
		case r.URL.Path == "/next" && r.URL.RawQuery == "login=1":
			queryRequests++
			_, _ = w.Write([]byte(`<html><body><article><p>query-page</p></article></body></html>`))
		case r.URL.Path == "/clean" && r.URL.RawQuery == "":
			cleanRequests++
			_, _ = w.Write([]byte(`<html><body><article><p>clean-page</p></article></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{server.URL},
	}, WithEmbedder(staticEmbedder{}))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("expected only root and clean pages to be saved, got %d", len(pages))
	}
	if cleanRequests != 1 {
		t.Fatalf("expected clean link to be requested once, got %d", cleanRequests)
	}
	if queryRequests != 0 {
		t.Fatalf("expected query-parameter link not to be requested, got %d", queryRequests)
	}
}

func TestFetchSkipsSeedURLsWithQueryParameters(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`<html><body><article><p>query-seed</p></article></body></html>`))
	}))
	defer server.Close()

	s, err := New(Config{
		CacheDir: cacheDir,
		Websites: []string{server.URL},
	}, WithEmbedder(staticEmbedder{}))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(server.URL + "?login=1")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if len(pages) != 0 {
		t.Fatalf("expected query-parameter seed url to be skipped, got %d saved pages", len(pages))
	}
	if requests != 0 {
		t.Fatalf("expected query-parameter seed url not to be requested, got %d", requests)
	}
}

func TestFetchLogsActionsWithoutHTMLContent(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	htmlBody := `<html><body>TOP-SECRET-HTML</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(htmlBody + `<a href="https://blocked.example/path">blocked</a>`))
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
		WithEmbedder(staticEmbedder{}),
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
	if strings.Contains(logs, "skipping url outside whitelist") {
		t.Fatalf("logs should not contain outside-whitelist skip message, got %q", logs)
	}
}

func TestFetchSkipsNonHTMLResponses(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4 fake pdf"))
	}))
	defer server.Close()

	s, err := New(
		Config{
			CacheDir: cacheDir,
			Websites: []string{server.URL},
		},
		WithEmbedder(staticEmbedder{}),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(pages) != 0 {
		t.Fatalf("expected non-html response to be skipped, got %d saved pages", len(pages))
	}

	dbPages, err := loadPagesFromDB(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load pages from db: %v", err)
	}
	if len(dbPages) != 0 {
		t.Fatalf("expected no pages in db for non-html response, got %d", len(dbPages))
	}
}

func TestFetchSkipsNonEnglishHTMLResponsesUsingLangMetadata(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	childRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html lang="de"><body><article><p>wurzel-seite</p><a href="/child">child</a></article></body></html>`))
		case "/child":
			childRequests++
			_, _ = w.Write([]byte(`<html lang="en"><body><article><p>child-page</p></article></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s, err := New(
		Config{
			CacheDir: cacheDir,
			Websites: []string{server.URL},
		},
		WithEmbedder(staticEmbedder{}),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(pages) != 0 {
		t.Fatalf("expected non-english html response to be skipped, got %d saved pages", len(pages))
	}
	if childRequests != 0 {
		t.Fatalf("expected links from non-english page not to be followed, got %d child requests", childRequests)
	}

	dbPages, err := loadPagesFromDB(filepath.Join(cacheDir, "pages.db"))
	if err != nil {
		t.Fatalf("load pages from db: %v", err)
	}
	if len(dbPages) != 0 {
		t.Fatalf("expected no pages in db for non-english html response, got %d", len(dbPages))
	}
}

func TestFetchKeepsEnglishHTMLResponsesUsingLangMetadata(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html lang="en-US"><body><article><p>english-page</p></article></body></html>`))
	}))
	defer server.Close()

	s, err := New(
		Config{
			CacheDir: cacheDir,
			Websites: []string{server.URL},
		},
		WithEmbedder(staticEmbedder{}),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected english html response to be saved, got %d pages", len(pages))
	}
	if !strings.Contains(pages[0].Content, "english-page") {
		t.Fatalf("expected saved english page content, got %q", pages[0].Content)
	}
}

func TestFetchAllowsConfiguredNonEnglishHTMLResponsesUsingLanguageFilter(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html lang="de-DE"><body><article><p>deutsche-seite</p></article></body></html>`))
	}))
	defer server.Close()

	s, err := New(
		Config{
			CacheDir:         cacheDir,
			AllowedLanguages: []string{"de"},
			Websites:         []string{server.URL},
		},
		WithEmbedder(staticEmbedder{}),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	pages, err := s.Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected configured german html response to be saved, got %d pages", len(pages))
	}
	if !strings.Contains(pages[0].Content, "deutsche-seite") {
		t.Fatalf("expected saved configured-language page content, got %q", pages[0].Content)
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
		WithEmbedder(staticEmbedder{}),
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
	}, WithEmbedder(staticEmbedder{}))
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

func loadVectorRowIDs(path string) ([]int64, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`select rowid from page_embeddings order by rowid asc`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table: page_embeddings") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
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

func randomNextPaths(path string, total int) []string {
	indexText := strings.TrimPrefix(path, "/page-")
	index, err := strconv.Atoi(indexText)
	if err != nil || index >= total-1 {
		return nil
	}

	links := []string{fmt.Sprintf("/page-%d", index+1)}
	if index >= total-2 {
		return links
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(path))
	extraOffset := int(hasher.Sum32()%uint32(total-index-2)) + 2
	links = append(links, fmt.Sprintf("/page-%d", index+extraOffset))
	return links
}

type staticEmbedder struct {
	vectors [][]float32
	err     error
}

type recordingSessionObserver struct {
	events   []SessionPageEvent
	failures []SessionFailureEvent
}

func (r *recordingSessionObserver) OnPageSaved(event SessionPageEvent) {
	r.events = append(r.events, event)
}

func (r *recordingSessionObserver) OnRequestFailed(event SessionFailureEvent) {
	r.failures = append(r.failures, event)
}

func (s staticEmbedder) Embed(_ context.Context, inputs []string) ([][]float32, error) {
	if s.err != nil {
		return nil, s.err
	}

	vectors := s.vectors
	if len(vectors) == 0 {
		vectors = [][]float32{testVectorWithLead(0.1, 0.2, 0.3)}
	}

	result := make([][]float32, 0, len(inputs))
	for i := range inputs {
		result = append(result, append([]float32(nil), vectors[i%len(vectors)]...))
	}
	return result, nil
}

func testVectorWithLead(values ...float32) []float32 {
	vector := make([]float32, 2560)
	copy(vector, values)
	return vector
}
