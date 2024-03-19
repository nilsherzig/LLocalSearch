package utils

import (
	"io"
	"net/http"
)

func DownloadWebsiteText(url string) (string, error) {
	// Make an HTTP GET request to the URL
	resp, err := http.Get(url)
	if err != nil {
		return "", err // Return the error if the request failed
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err // Return the error if reading the body failed
	}

	// Convert the body to a string and return it
	return string(body), nil
}
