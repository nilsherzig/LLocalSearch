package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/microcosm-cc/bluemonday"

	"github.com/tmc/langchaingo/documentloaders"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/textsplitter"
	"github.com/tmc/langchaingo/vectorstores/chroma"
)

func saveToVectorDb(timeoutCtx context.Context, docs []schema.Document, sessionString string) error {
	llm, err := NewOllamaEmbeddingLLM()
	if err != nil {
		return err
	}

	embeder, err := embeddings.NewEmbedder(llm)
	if err != nil {
		return err
	}

	store, errNs := chroma.New(
		chroma.WithChromaURL(os.Getenv("CHROMA_DB_URL")),
		chroma.WithEmbedder(embeder),
		chroma.WithDistanceFunction("cosine"),
		chroma.WithNameSpace(sessionString),
	)

	if errNs != nil {
		return errNs
	}

	type meta = map[string]any
	for i := range docs {
		if len(docs[i].PageContent) == 0 {
			// remove the document from the list
			docs = append(docs[:i], docs[i+1:]...)
		}
	}

	_, errAd := store.AddDocuments(timeoutCtx, docs)

	if errAd != nil {
		slog.Warn("Error adding document: %v\n", errAd)
		return fmt.Errorf("Error adding document: %v\n", errAd)
	}

	// log.Printf("Added %d documents\n", len(res))
	return nil
}

func DownloadWebsiteToVectorDB(ctx context.Context, url string, sessionString string, chunkSize int, chunkOverlap int) error {
	// log.Printf("downloading: %s", url)
	html, err := DownloadWebsiteText(url)
	if err != nil {
		fmt.Printf("error from evaluator: %s", err.Error())
		return err
	}

	sanitizedHtml := stripHtml(html)
	if len(sanitizedHtml) == 0 {
		return fmt.Errorf("no content found")
	}

	vectorLoader := documentloaders.NewText(strings.NewReader(sanitizedHtml))
	splitter := textsplitter.NewTokenSplitter(
		textsplitter.WithSeparators([]string{"\n\n", "\n"}),
	)
	splitter.ChunkSize = chunkSize
	splitter.ChunkOverlap = chunkOverlap
	docs, err := vectorLoader.LoadAndSplit(ctx, splitter)

	for i := range docs {
		docs[i].Metadata = map[string]interface{}{
			"url": url,
		}
	}

	// timeoutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	// defer cancel()

	err = saveToVectorDb(context.Background(), docs, sessionString)
	if err != nil {
		return err
	}
	return nil
}

func stripHtml(html string) string {
	policy := bluemonday.StrictPolicy()
	result := policy.Sanitize(html)
	// result = strings.ReplaceAll(result, "\n", "")
	result = strings.ReplaceAll(result, "\t", "")
	result = strings.ReplaceAll(result, "&#34;", "")
	result = strings.ReplaceAll(result, "&#39;", "")
	return result
}
