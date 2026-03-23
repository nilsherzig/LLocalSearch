package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/nilsherzig/llocalsearch/frontend"
	"github.com/nilsherzig/llocalsearch/scraper"
)

func main() {
	if err := run(os.Args[1:], os.Stderr, http.ListenAndServe); err != nil {
		logger := newLogger(os.Stderr)
		logger.Error("frontend failed", "err", err)
		os.Exit(1)
	}
}

func newLogger(stderr io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(stderr, nil))
}

func run(args []string, stderr io.Writer, listen func(string, http.Handler) error) error {
	logger := newLogger(stderr)

	fs := flag.NewFlagSet("llocalsearch-frontend", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", "scraper.example.yaml", "path to scraper config yaml")
	addr := fs.String("addr", ":8080", "frontend listen address")
	templatesDir := fs.String("templates-dir", filepath.Join("frontend", "templates"), "path to frontend templates")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "usage: llocalsearch-frontend [-config path] [-addr address] [-templates-dir dir]\n")
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

	server, err := frontend.NewServer(frontend.Config{
		DBPath:         filepath.Join(cfg.CacheDir, "pages.db"),
		TemplatesDir:   *templatesDir,
		WhitelistPages: cfg.Websites,
		Logger:         logger,
	})
	if err != nil {
		return fmt.Errorf("create frontend server: %w", err)
	}

	logger.Info("starting frontend server", "addr", *addr, "db_path", filepath.Join(cfg.CacheDir, "pages.db"))
	return listen(*addr, server.Handler())
}
