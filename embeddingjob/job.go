package embeddingjob

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/nilsherzig/llocalsearch/embedding"
	"github.com/nilsherzig/llocalsearch/scraper"
	"github.com/nilsherzig/llocalsearch/vectorstore"
	htmlnode "golang.org/x/net/html"
	"gorm.io/gorm"
)

type Config struct {
	DBPath     string
	BaseURL    string
	Model      string
	Dimensions int
	Timeout    time.Duration
	BatchSize  int
	Embedder   embedding.Client
	Logger     *slog.Logger
	Now        func() time.Time
}

type Job struct {
	db        *gorm.DB
	embedder  embedding.Client
	logger    *slog.Logger
	now       func() time.Time
	batchSize int
}

type Result struct {
	TotalMissing  int
	EmbeddedPages int
}

type pendingPage struct {
	ID      uint
	Content string
}

func New(cfg Config) (*Job, error) {
	if cfg.DBPath == "" {
		return nil, fmt.Errorf("db path is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("embedding model is required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 8
	}

	embedder := cfg.Embedder
	if embedder == nil {
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("embedding base url is required")
		}
		embedder = embedding.NewClient(embedding.Config{
			BaseURL:    cfg.BaseURL,
			Model:      cfg.Model,
			Dimensions: cfg.Dimensions,
			Timeout:    cfg.Timeout,
		}, nil)
	}

	db, err := vectorstore.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&scraper.SavedPage{}); err != nil {
		return nil, fmt.Errorf("migrate sqlite database: %w", err)
	}
	if err := vectorstore.EnsureSchema(db, cfg.Model, cfg.Dimensions); err != nil {
		return nil, err
	}

	return &Job{
		db:        db,
		embedder:  embedder,
		logger:    logger,
		now:       now,
		batchSize: batchSize,
	}, nil
}

func (j *Job) Run(ctx context.Context) (Result, error) {
	totalMissing, err := j.countMissingEmbeddings()
	if err != nil {
		return Result{}, err
	}

	startedAt := time.Now()
	j.logger.Info("embedding job started", "total_missing", totalMissing, "batch_size", j.batchSize)

	result := Result{TotalMissing: int(totalMissing)}
	for {
		batch, err := j.loadMissingBatch()
		if err != nil {
			return result, err
		}
		if len(batch) == 0 {
			break
		}

		inputs := make([]string, 0, len(batch))
		for _, page := range batch {
			inputs = append(inputs, extractPlainText(page.Content))
		}

		batchStartedAt := time.Now()
		vectors, err := j.embedder.Embed(ctx, inputs)
		if err != nil {
			return result, fmt.Errorf("embed page content: %w", err)
		}
		if len(vectors) != len(batch) {
			return result, fmt.Errorf("embed page content returned %d vectors", len(vectors))
		}

		if err := j.db.Transaction(func(tx *gorm.DB) error {
			for i, page := range batch {
				if err := vectorstore.UpsertEmbedding(tx, page.ID, vectors[i]); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return result, err
		}

		result.EmbeddedPages += len(batch)
		batchDuration := time.Since(batchStartedAt)
		remaining := result.TotalMissing - result.EmbeddedPages
		perPage := time.Duration(0)
		if len(batch) > 0 {
			perPage = batchDuration / time.Duration(len(batch))
		}
		j.logger.Info(
			"embedding batch finished",
			"embedded_pages", result.EmbeddedPages,
			"remaining_pages", remaining,
			"batch_size", len(batch),
			"embedding_progress", fmt.Sprintf("%d/%d", result.EmbeddedPages, result.TotalMissing),
			"batch_duration", batchDuration,
			"avg_page_duration", perPage,
		)
	}

	j.logger.Info(
		"embedding job finished",
		"embedded_pages", result.EmbeddedPages,
		"remaining_pages", result.TotalMissing-result.EmbeddedPages,
		"embedding_progress", fmt.Sprintf("%d/%d", result.EmbeddedPages, result.TotalMissing),
		"total_duration", time.Since(startedAt),
		"finished_at", j.now().UTC().Format(time.RFC3339),
	)
	return result, nil
}

func (j *Job) countMissingEmbeddings() (int64, error) {
	var count int64
	result := j.db.Raw(`
		select count(*)
		from saved_pages
		left join page_embeddings on page_embeddings.rowid = saved_pages.id
		where page_embeddings.rowid is null
	`).Scan(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("count pages without embeddings: %w", result.Error)
	}

	return count, nil
}

func (j *Job) loadMissingBatch() ([]pendingPage, error) {
	var pages []pendingPage
	result := j.db.Raw(`
		select saved_pages.id, saved_pages.content
		from saved_pages
		left join page_embeddings on page_embeddings.rowid = saved_pages.id
		where page_embeddings.rowid is null
		order by saved_pages.id asc
		limit ?
	`, j.batchSize).Scan(&pages)
	if result.Error != nil {
		return nil, fmt.Errorf("load pages without embeddings: %w", result.Error)
	}
	return pages, nil
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
