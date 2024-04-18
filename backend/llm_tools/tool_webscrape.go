package llm_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/nilsherzig/LLocalSearch/utils"
	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/tools"
)

type WebScrape struct {
	CallbacksHandler callbacks.Handler
	SessionString    string
	Settings         utils.ClientSettings
}

var usedLinks = make(map[string][]string)

var _ tools.Tool = WebScrape{}

func (c WebScrape) Description() string {
	return `Use this tool to search for websites that may answer your search query. The best websites (according to the search engine) are broken down into small parts and added to your vector database. 

The parts of these websites that are most similar to your search query will be returned to you directly. 

You can query the vector database later with other inputs to get other parts of these websites.`
}

func (c WebScrape) Name() string {
	return "webscrape"
}

func (ws WebScrape) Call(ctx context.Context, input string) (string, error) {
	if ws.CallbacksHandler != nil {
		ws.CallbacksHandler.HandleToolStart(ctx, input)
	}

	input = strings.TrimPrefix(input, "\"")
	input = strings.TrimSuffix(input, "\"")
	inputQuery := url.QueryEscape(input)
	searXNGDomain := os.Getenv("SEARXNG_DOMAIN")
	url := fmt.Sprintf("%s/?q=%s&format=json", searXNGDomain, inputQuery)
	resp, err := http.Get(url)

	if err != nil {
		slog.Warn("Error making the request", "error", err)
		return "", err
	}
	defer resp.Body.Close()

	var apiResponse utils.SeaXngResult
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		slog.Warn("Error decoding the response", "error", err)
		return "", err
	}

	wg := sync.WaitGroup{}
	counter := 0
	for i := range apiResponse.Results {
		skip := false
		for _, usedLink := range usedLinks[ws.SessionString] {
			if usedLink == apiResponse.Results[i].URL {
				skip = true
				break
			}
		}

		if skip {
			continue
		}

		if counter >= ws.Settings.AmountOfWebsites {
			break
		}

		// if result link ends in .pdf, skip
		if strings.HasSuffix(apiResponse.Results[i].URL, ".pdf") {
			continue
		}

		counter += 1
		wg.Add(1)
		go func(i int) {
			defer func() {
				wg.Done()
				if r := recover(); r != nil {
					slog.Error("Recovered from panic", "error", r)
				}
			}()

			err := utils.DownloadWebsiteToVectorDB(context.Background(), apiResponse.Results[i].URL, ws.SessionString, ws.Settings.ChunkSize, ws.Settings.ChunkOverlap)
			if err != nil {
				slog.Warn("Error downloading website", "error", err)
				return
			}
			ch, ok := ws.CallbacksHandler.(utils.CustomHandler)
			if ok {
				newSource := utils.Source{
					Name:    "WebSearch",
					Link:    apiResponse.Results[i].URL,
					Summary: apiResponse.Results[i].Content,
					Title:   apiResponse.Results[i].Title,
					Engine:  apiResponse.Results[i].Engine,
				}

				ch.HandleSourceAdded(ctx, newSource)
			}
		}(i)
		usedLinks[ws.SessionString] = append(usedLinks[ws.SessionString], apiResponse.Results[i].URL)
	}
	wg.Wait()
	svb := SearchVectorDB{
		CallbacksHandler: ws.CallbacksHandler,
		SessionString:    ws.SessionString,
		Settings:         ws.Settings,
	}
	result, err := svb.Call(context.Background(), input)
	if err != nil {
		return fmt.Sprintf("error from vector db search: %s", err.Error()), nil
	}

	if ws.CallbacksHandler != nil {
		ws.CallbacksHandler.HandleToolEnd(ctx, result)
	}

	if len(apiResponse.Results) == 0 {
		return "No results found", fmt.Errorf("No results, we might be rate limited")
	}

	return result, nil
}
