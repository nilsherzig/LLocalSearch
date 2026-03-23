package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/nilsherzig/llocalsearch/scraper"
)

func main() {
	logger := newLogger(os.Stderr)
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		logger.Error("scrape failed", "err", err)
		os.Exit(1)
	}
}

func newLogger(stderr io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(stderr, nil))
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	logger := newLogger(stderr)

	fs := flag.NewFlagSet("llocalsearch", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", "scraper.example.yaml", "path to scraper config yaml")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "usage: llocalsearch [-config path]\n")
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
