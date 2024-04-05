package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/nilsherzig/localLLMSearch/utils"
)

func setCorsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// streamHandler handles HTTP requests and streams the output of longRunningFunction.
func streamHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	setCorsHeaders(w)

	// Handle pre-flight CORS request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	clientQuery := utils.ClientQuery{}

	// get request params
	prompt := r.URL.Query().Get("prompt")
	if prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}
	clientQuery.Prompt = prompt

	session := r.URL.Query().Get("session")
	if session == "" {
		http.Error(w, "session is required", http.StatusBadRequest)
		return
	}
	clientQuery.Session = session

	modelname := r.URL.Query().Get("modelname")
	if modelname == "" {
		http.Error(w, "modelname is required", http.StatusBadRequest)
		return
	}
	clientQuery.ModelName = modelname

	maxIterations := os.Getenv("MAX_ITERATIONS")

	asInt, err := strconv.Atoi(maxIterations)
	if err != nil {
		http.Error(w, "maxIterations must be a number", http.StatusBadRequest)
		return
	}
	clientQuery.MaxIterations = asInt

	// Set the header to indicate streaming content
	w.Header().Set("Content-Type", "text/event-stream")

	// Create a channel for communication with the llm agent chain
	outputChan := make(chan utils.HttpJsonStreamElement)
	defer close(outputChan)

	// Start the agent chain function in a goroutine
	ctx := r.Context() // using the request ctx
	go startAgentChain(ctx, outputChan, clientQuery)

	// Stream the output back to the client as it arrives
	for {
		select {
		case output, ok := <-outputChan:
			if !ok {
				// Channel was closed, end the response
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
		case <-ctx.Done():
			slog.Info("Client disconnected")
			return
		}
	}
}

func modellistHandler(w http.ResponseWriter, r *http.Request) {
	utils.GetOllamaModelList()
	// Set CORS headers
	setCorsHeaders(w)

	// Handle pre-flight CORS request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	models, err := utils.GetOllamaModelList()
	if err != nil {
		http.Error(w, "Error getting model list", http.StatusInternalServerError)
		return
	}

	jsonModels, err := json.Marshal(models)
	if err != nil {
		http.Error(w, "Error marshalling model list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonModels)
}

func StartApiServer() {
	// Register the handler function
	http.HandleFunc("/stream", streamHandler)
	http.HandleFunc("/modellist", modellistHandler)

	// Start the HTTP server
	fmt.Println("Server started at http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
