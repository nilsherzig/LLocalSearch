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

// ReadWebsite is a tool that can do math.
type WebSearch struct {
	CallbacksHandler callbacks.Handler
	SessionString    string
}

var usedLinks = make(map[string]bool)

var _ tools.Tool = WebSearch{}

func (c WebSearch) Description() string {
	return `Usefull for searching the internet. The top results for your search query will be downloaded to your vector db.`
}

func (c WebSearch) Name() string {
	return "WebSearch"
}

func (c WebSearch) Call(ctx context.Context, input string) (string, error) {
	if c.CallbacksHandler != nil {
		c.CallbacksHandler.HandleToolStart(ctx, input)
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

	// for i, result := range apiResponse.Results {
	// 	if i < 3 {
	// 		err := DownloadWebsiteToVectorDB(ctx, result.URL, c.sessionString)
	// 		if err != nil {
	// 			return fmt.Sprintf("error from evaluator: %s", err.Error()), nil //nolint:nilerr
	// 		}
	// 	}
	// }

	wg := sync.WaitGroup{}
	counter := 0
	for i := range apiResponse.Results {
		if usedLinks[apiResponse.Results[i].URL] {
			continue
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
			err := utils.DownloadWebsiteToVectorDB(ctx, apiResponse.Results[i].URL, c.SessionString)
			if err != nil {
				log.Printf("error from evaluator: %s", err.Error())
				wg.Done()
				return
			}
			ch, ok := c.CallbacksHandler.(utils.CustomHandler)
			if ok {
				newSource := utils.Source{
					Name: "WebSearch",
					Link: apiResponse.Results[i].URL,
				}

				ch.HandleSourceAdded(ctx, newSource)
				usedLinks[apiResponse.Results[i].URL] = true
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	result := fmt.Sprintf("Downloaded websites to vector db. You dont know anything new from this tool, you have to search through the vector db to find anything about the downloaded websites.")

	if c.CallbacksHandler != nil {
		c.CallbacksHandler.HandleToolEnd(ctx, result)
	}

	if len(apiResponse.Results) == 0 {
		return "No results found", fmt.Errorf("No results, we might be rate limited")
	}

	return result, nil
}
