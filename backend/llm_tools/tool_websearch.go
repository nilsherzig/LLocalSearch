package llm_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/nilsherzig/localLLMSearch/utils"
	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/tools"
)

type WebSearch struct {
	CallbacksHandler callbacks.Handler
	SessionString    string
}

var usedLinks = make(map[string][]string)

var _ tools.Tool = WebSearch{}

func (c WebSearch) Description() string {
	return `Usefull for searching the internet. You have to use this tool if you're not 100% certain. The top 10 results will be added to the vector db. The top 3 results are also getting returned to you directly.`
}

func (c WebSearch) Name() string {
	return "WebSearch"
}

func (ws WebSearch) Call(ctx context.Context, input string) (string, error) {
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
		log.Println("Error making the request:", err)
		return "", err
	}
	defer resp.Body.Close()

	var apiResponse utils.SeaXngResult
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		fmt.Println("Error parsing the JSON:", err)
		return "", err
	}

	log.Printf("Search found %d Results\n", len(apiResponse.Results))

	wg := sync.WaitGroup{}
	counter := 0
	for i := range apiResponse.Results {
		for _, usedLink := range usedLinks[ws.SessionString] {
			if usedLink == apiResponse.Results[i].URL {
				continue
			}
		}

		if counter > 10 {
			break
		}

		// if result link ends in .pdf, skip
		if strings.HasSuffix(apiResponse.Results[i].URL, ".pdf") {
			continue
		}

		counter += 1
		wg.Add(1)
		go func(i int) {
			err := utils.DownloadWebsiteToVectorDB(ctx, apiResponse.Results[i].URL, ws.SessionString)
			if err != nil {
				log.Printf("error from evaluator: %s", err.Error())
				wg.Done()
				return
			}
			ch, ok := ws.CallbacksHandler.(utils.CustomHandler)
			if ok {
				newSource := utils.Source{
					Name: "WebSearch",
					Link: apiResponse.Results[i].URL,
				}

				ch.HandleSourceAdded(ctx, newSource)
				usedLinks[ws.SessionString] = append(usedLinks[ws.SessionString], apiResponse.Results[i].URL)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	result, err := SearchVectorDB.Call(
		SearchVectorDB{
			CallbacksHandler: nil,
			SessionString:    ws.SessionString,
		},
		context.Background(), input)
	if err != nil {
		return fmt.Sprintf("error from vector db search: %s", err.Error()), nil //nolint:nilerr
	}

	if ws.CallbacksHandler != nil {
		ws.CallbacksHandler.HandleToolEnd(ctx, result)
	}

	if len(apiResponse.Results) == 0 {
		return "No results found", fmt.Errorf("No results, we might be rate limited")
	}

	return result, nil
}
