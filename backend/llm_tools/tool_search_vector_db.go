package llm_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/nilsherzig/LLocalSearch/utils"
	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/tools"
	"github.com/tmc/langchaingo/vectorstores"
	"github.com/tmc/langchaingo/vectorstores/chroma"
)

// ReadWebsite is a tool that can do math.
type SearchVectorDB struct {
	CallbacksHandler callbacks.Handler
	SessionString    string
	Settings         utils.ClientSettings
}

var _ tools.Tool = SearchVectorDB{}

type Result struct {
	Text string
}

var usedResults = make(map[string][]string)
var usedSourcesInSession = make(map[string][]schema.Document)

func (c SearchVectorDB) Description() string {
	return "Use this tool to search through already added files or websites within a vector database. The most similar websites or documents to your input will be returned to you."
}

func (c SearchVectorDB) Name() string {
	return "database_search"
}

func (c SearchVectorDB) Call(ctx context.Context, input string) (string, error) {
	if c.CallbacksHandler != nil {
		c.CallbacksHandler.HandleToolStart(ctx, input)
	}

	searchIdentifier := fmt.Sprintf("%s-%s", c.SessionString, input)

	llm, err := utils.NewOllamaEmbeddingLLM()
	if err != nil {
		return "", err
	}

	ollamaEmbeder, err := embeddings.NewEmbedder(llm)
	if err != nil {
		return "", err
	}

	store, errNs := chroma.New(
		chroma.WithChromaURL(os.Getenv("CHROMA_DB_URL")),
		chroma.WithEmbedder(ollamaEmbeder),
		chroma.WithDistanceFunction("cosine"),
		chroma.WithNameSpace(c.SessionString),
	)

	if errNs != nil {
		return "", errNs
	}

	options := []vectorstores.Option{
		vectorstores.WithScoreThreshold(float32(c.Settings.MinResultScore)),
	}

	retriver := vectorstores.ToRetriever(store, c.Settings.AmountOfResults, options...)
	docs, err := retriver.GetRelevantDocuments(context.Background(), input)
	if err != nil {
		return "", err
	}

	var results []Result

	for _, doc := range docs {
		newResult := Result{
			Text: doc.PageContent,
		}

		skip := false
		for _, usedLink := range usedResults[searchIdentifier] {
			if usedLink == newResult.Text {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		usedSourcesInSession[c.SessionString] = append(usedSourcesInSession[c.SessionString], doc)

		ch, ok := c.CallbacksHandler.(utils.CustomHandler)
		if ok {
			slog.Info("found vector", "source", doc.Metadata["URL"])
			ch.HandleSourceAdded(ctx, utils.Source{
				Name:    "DatabaseSearch",
				Link:    "none",
				Summary: doc.PageContent,
				Engine:  "DatabaseSearch",
				Title:   "DatabaseSearch",
			})
		}
		results = append(results, newResult)
		usedResults[searchIdentifier] = append(usedResults[searchIdentifier], newResult.Text)
	}

	if len(docs) == 0 {
		response := "No new results found. Try other db search keywords, download more websites or write your final answer."
		slog.Warn("No new results found", "input", input)
		results = append(results, Result{Text: response})
	}

	if c.CallbacksHandler != nil {
		c.CallbacksHandler.HandleToolEnd(ctx, input)
	}

	resultJson, err := json.Marshal(results)
	if err != nil {
		return "", err
	}

	return string(resultJson), nil
}

func extractBaseDomain(inputURL string) (string, error) {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", err
	}
	return parsedURL.Host, nil
}
