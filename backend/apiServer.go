package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nilsherzig/LLocalSearch/utils"
)

func streamHandler(w http.ResponseWriter, r *http.Request) {
	setCorsHeaders(w) // TODO not needed while proxied through frontend?

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// parse request settings
	clientSettings, err := parseClientSettings(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("%v", err), http.StatusBadRequest)
		return
	}

	// create the go channel which is used to push
	// logs from deeper inside the backend to
	// the http stream
	outputChan := make(chan utils.HttpJsonStreamElement)
	defer close(outputChan)

	// start the actual agentchain
	go startAgentChain(r.Context(), outputChan, clientSettings)

	w.Header().Set("Content-Type", "text/event-stream")
	for {
		select {
		case output, ok := <-outputChan:
			if !ok {
				return
			}
			jsonString, err := json.Marshal(output)
			if err != nil {
				slog.Info("Error marshalling output", "error", err)
			}
			sse := fmt.Sprintf("data: %s\n\n", jsonString)
			_, writeErr := fmt.Fprintf(w, sse)
			if writeErr != nil {
				slog.Info("Error writing to response writer", "error", writeErr)
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			slog.Info("Client disconnected")
			return
		}
	}
}

func modelsHandler(w http.ResponseWriter, r *http.Request) {
	setCorsHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	models, err := utils.GetOllamaModelList()
	if err != nil {
		http.Error(w, "Error getting model list", http.StatusInternalServerError)
		slog.Error("Error getting models")
		return
	}

	// remove the currently used embeddings model
	// from the modellist
	// TODO: find a way to remove all embeddings models
	for i, model := range models {
		if model == utils.EmbeddingsModel {
			models = append(models[:i], models[i+1:]...)
		}
	}

	jsonModels, err := json.Marshal(models)
	if err != nil {
		http.Error(w, "Error marshalling model list", http.StatusInternalServerError)
		slog.Error("Error marshalling models")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonModels)
}

func StartApiServer() {
	http.HandleFunc("/stream", streamHandler)
	http.HandleFunc("/models", modelsHandler)

	slog.Info("Starting server at http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("Error starting server", "error", err)
	}
}

func setCorsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func parseClientSettings(r *http.Request) (utils.ClientSettings, error) {
	body := r.URL.Query().Get("settings")
	if body == "" {
		slog.Error("no settings json provided")
		return utils.ClientSettings{}, fmt.Errorf("no settings json provided")
	}

	clientSettings := utils.ClientSettings{}
	err := json.Unmarshal([]byte(body), &clientSettings)
	if err != nil {
		slog.Error("error parsing request body json", "error", err)
		return utils.ClientSettings{}, err
	}
	slog.Info("Client settings", "settings", clientSettings)
	return clientSettings, nil
}
