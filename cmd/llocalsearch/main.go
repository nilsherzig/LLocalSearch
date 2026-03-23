package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nilsherzig/llocalsearch/embeddingjob"
	"github.com/nilsherzig/llocalsearch/frontend"
	"github.com/nilsherzig/llocalsearch/scraper"
)

type listenFunc func(string, http.Handler) error

func main() {
	logger := newLogger(os.Stderr)
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		logger.Error("command failed", "err", err)
		os.Exit(1)
	}
}

func newLogger(stderr io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(stderr, nil))
}

func run(args []string, stdout io.Writer, stderr io.Writer, listeners ...listenFunc) error {
	listen := http.ListenAndServe
	if len(listeners) > 0 && listeners[0] != nil {
		listen = listeners[0]
	}

	if len(args) == 0 {
		printRootUsage(stderr)
		return fmt.Errorf("subcommand is required")
	}

	switch args[0] {
	case "scrape":
		return runScrape(args[1:], stdout, stderr)
	case "embed":
		return runEmbed(args[1:], stdout, stderr)
	case "web":
		return runWeb(args[1:], stderr, listen)
	default:
		printRootUsage(stderr)
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func printRootUsage(stderr io.Writer) {
	_, _ = fmt.Fprintf(stderr, "usage: llocalsearch <scrape|embed|web> [flags]\n")
}

func runScrape(args []string, stdout io.Writer, stderr io.Writer) error {
	logger := newLogger(stderr)

	fs := flag.NewFlagSet("llocalsearch scrape", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", "scraper.example.yaml", "path to scraper config yaml")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "usage: llocalsearch scrape [-config path]\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		fs.Usage()
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("positional url arguments are not supported")
	}

	cfg, err := scraper.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger.Info("loaded config", "targets", len(cfg.Websites), "cache_dir", cfg.CacheDir)

	statusReporter := newSessionStatusReporter(stderr)
	defer statusReporter.Finish()

	s, err := scraper.New(cfg, scraper.WithLogger(logger), scraper.WithSessionObserver(statusReporter))
	if err != nil {
		return fmt.Errorf("create scraper: %w", err)
	}

	logger.Info("scraping configured targets", "count", len(cfg.Websites))
	pages, err := s.FetchAll(cfg.Websites)
	if err != nil {
		return fmt.Errorf("fetch configured targets: %w", err)
	}
	logger.Info("finished configured targets", "saved_pages", len(pages))

	_ = stdout
	return nil
}

func runEmbed(args []string, stdout io.Writer, stderr io.Writer) error {
	logger := newLogger(stderr)

	fs := flag.NewFlagSet("llocalsearch embed", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", "scraper.example.yaml", "path to scraper config yaml")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "usage: llocalsearch embed [-config path]\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		fs.Usage()
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("positional arguments are not supported")
	}

	cfg, err := scraper.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Embeddings.BaseURL == "" || cfg.Embeddings.Model == "" {
		return fmt.Errorf("embed requires embeddings.base_url and embeddings.model")
	}

	job, err := embeddingjob.New(embeddingjob.Config{
		DBPath:     filepath.Join(cfg.CacheDir, "pages.db"),
		BaseURL:    cfg.Embeddings.BaseURL,
		Model:      cfg.Embeddings.Model,
		Dimensions: cfg.Embeddings.Dimensions,
		Timeout:    time.Duration(cfg.Embeddings.Timeout),
		BatchSize:  cfg.Embeddings.BatchSize,
		Logger:     logger,
	})
	if err != nil {
		return fmt.Errorf("create embedding job: %w", err)
	}

	result, err := job.Run(context.Background())
	if err != nil {
		return fmt.Errorf("run embedding job: %w", err)
	}
	logger.Info("embedding job completed", "embedded_pages", result.EmbeddedPages, "total_missing", result.TotalMissing)

	_ = stdout
	return nil
}

func runWeb(args []string, stderr io.Writer, listen listenFunc) error {
	logger := newLogger(stderr)

	fs := flag.NewFlagSet("llocalsearch web", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", "scraper.example.yaml", "path to scraper config yaml")
	addr := fs.String("addr", ":8080", "frontend listen address")
	templatesDir := fs.String("templates-dir", filepath.Join("frontend", "templates"), "path to frontend templates")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "usage: llocalsearch web [-config path] [-addr address] [-templates-dir dir]\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		fs.Usage()
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("positional arguments are not supported")
	}

	cfg, err := scraper.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Embeddings.BaseURL == "" || cfg.Embeddings.Model == "" {
		return fmt.Errorf("web requires embeddings.base_url and embeddings.model")
	}

	server, err := frontend.NewServer(frontend.Config{
		DBPath:         filepath.Join(cfg.CacheDir, "pages.db"),
		TemplatesDir:   *templatesDir,
		Embeddings:     cfg.Embeddings,
		WhitelistPages: cfg.Websites,
		Logger:         logger,
	})
	if err != nil {
		return fmt.Errorf("create frontend server: %w", err)
	}

	logger.Info("starting frontend server", "addr", *addr, "db_path", filepath.Join(cfg.CacheDir, "pages.db"))
	return listen(*addr, server.Handler())
}
