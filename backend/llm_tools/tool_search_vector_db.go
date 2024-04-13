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
	Text   string
	Source string
}

var usedResults = make(map[string][]string)

func (c SearchVectorDB) Description() string {
	return `Usefull for searching through added files and websites. Also contains the results from all old searches. Search for keywords in the text not whole questions, avoid relative words like "yesterday" think about what could be in the text. 
    The input to this tool will be run against a vector db. The top results will be returned as json.`
}

func (c SearchVectorDB) Name() string {
	return "SearchVectorDB"
}

func (c SearchVectorDB) Call(ctx context.Context, input string) (string, error) {
	if c.CallbacksHandler != nil {
		c.CallbacksHandler.HandleToolStart(ctx, input)
	}

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

	for _, r := range docs {
		newResult := Result{
			Text: r.PageContent,
		}

		source, ok := r.Metadata["url"].(string)
		if ok {
			newResult.Source = source
		}

		for _, usedLink := range usedResults[c.SessionString] {
			if usedLink == newResult.Text {
				continue
			}
		}
		ch, ok := c.CallbacksHandler.(utils.CustomHandler)
		if ok {
			ch.HandleVectorFound(ctx, fmt.Sprintf("%s with a score of %f", newResult.Source, r.Score))
		}
		results = append(results, newResult)
		usedResults[c.SessionString] = append(usedResults[c.SessionString], newResult.Text)
	}

	if len(docs) == 0 {

		response := "no results found. Try other db search keywords or download more websites."
		slog.Warn("no results found", "input", input)
		results = append(results, Result{Text: response})

	} else if len(results) == 0 {

		response := "No new results found, all returned results have been used already. Try other db search keywords or download more websites."
		// log.Println(response)
		results = append(results, Result{Text: response})
	}

	// c.CallbacksHandler.HandleVectorFound(ctx, input, resp)
	if c.CallbacksHandler != nil {
		c.CallbacksHandler.HandleToolEnd(ctx, input)
	}

	resultJson, err := json.Marshal(results)
	if err != nil {
		return "", err
	}

	// log.Printf("%s", string(resultJson))
	// charCount := utf8.RuneCountInString(string(resultJson))
	// log.Printf("result tokens %d", (charCount / 4))

	return string(resultJson), nil
}
func extractBaseDomain(inputURL string) (string, error) {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", err
	}
	return parsedURL.Host, nil
}
