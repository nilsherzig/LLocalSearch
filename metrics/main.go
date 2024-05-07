package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

type Message struct {
	Title string `json:"title"`
	Msg   string `json:"msg"`
}

type metricsResponse struct {
	Problems []Message `json:"problems"`
}

type metricsReqBody struct {
	Version string `json:"version"`
	Model   string `json:"model"`
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	setCorsHeaders(w) // TODO not needed while proxied through frontend?
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	slog.Info("Request", "method", r.Method, "url", r.URL, "remote", strings.Split(r.RemoteAddr, ":")[0])

	// read body and log it
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Error reading body", "error", err)
	}
	rb := metricsReqBody{}
	err = json.Unmarshal(body, &rb)

	ipHash := hash(strings.Split(r.RemoteAddr, ":")[0])
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	resp := metricsResponse{}

	problems := []Message{}
	if rb.Version < latestVersion {
		message := fmt.Sprintf("Your version is outdated. Please update your docker containers. Your version: %s, latest version: %s", rb.Version, latestVersion)
		problems = append(problems, Message{"Version outdated", message})
	}

	resp.Problems = problems

	slog.Info("Client", "iphash", ipHash, "version", rb.Version, "model", rb.Model, "problems", problems)
	json.NewEncoder(w).Encode(resp)
}

func StartApiServer() {
	http.HandleFunc("/v1", versionHandler)

	slog.Info("Starting server at http://localhost:9999")
	if err := http.ListenAndServe(":9999", nil); err != nil {
		slog.Error("Error starting server", "error", err)
	}
}

var (
	latestVersion string
	ok            bool
)

func main() {
	latestVersion, ok = os.LookupEnv("VERSION")
	if !ok {
		slog.Error("VERSION env var not set")
		return
	}
	StartApiServer()
}

func setCorsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
