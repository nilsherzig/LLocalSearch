package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/nilsherzig/LLocalSearch/utils"
)

func sendError(w http.ResponseWriter, message string, errorcode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(errorcode)
	json.NewEncoder(w).Encode(utils.HttpError{Error: message})
}

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
	outputChan := make(chan utils.HttpJsonStreamElement, 100)
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
			output.TimeStamp = time.Now().Unix()

			if output.StepType == utils.StepHandleNewSession {
				clientSettings.Session = output.Session
			}

			session, ok := sessions[clientSettings.Session]
			if ok {
				session.Elements = append(session.Elements, output)
				sessions[clientSettings.Session] = session
			} else {
				slog.Error("Session not found", "session", clientSettings.Session)
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

// TODO improve the amount of data that is sent
// currently 99% of the data is empty json keys haha
func loadChatHistory(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Millisecond * 350)
	setCorsHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	requestChatId := r.PathValue("chatid")
	if requestChatId == "" {
		message := "No chatId provided"
		sendError(w, message, http.StatusBadRequest)
		slog.Error(message)
		return
	}

	chat, ok := sessions[requestChatId]
	if !ok {
		message := fmt.Sprintf("Chat with id %s not found", requestChatId)
		sendError(w, message, http.StatusInternalServerError)
		slog.Error(message)
		return
	}

	response := chat.Elements

	jsonChat, err := json.Marshal(response)
	if err != nil {
		message := "Error marshalling chat"
		sendError(w, message, http.StatusInternalServerError)
		slog.Error(message)
		return
	}

	slog.Info("Loaded Chat", "id", requestChatId, "message count", len(chat.Elements))
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonChat)
}

func chatListHandler(w http.ResponseWriter, r *http.Request) {
	// time.Sleep(time.Millisecond * 120)
	setCorsHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	chatIds := []utils.ChatListItem{}
	for sessionid := range sessions {
		chatIds = append(chatIds, utils.ChatListItem{
			SessionId: sessionid,
			Title:     sessions[sessionid].Title,
		})
	}
	// sort chatIds by timestamp
	// TODO HACK this is wildly inefficient
	sort.Slice(chatIds, func(i, j int) bool {
		iLen := len(sessions[chatIds[i].SessionId].Elements)
		jLen := len(sessions[chatIds[j].SessionId].Elements)
		return sessions[chatIds[i].SessionId].Elements[iLen-1].TimeStamp > sessions[chatIds[j].SessionId].Elements[jLen-1].TimeStamp
	})

	jsonChatIds, err := json.Marshal(chatIds)
	if err != nil {
		message := "Error marshalling chatIds"
		sendError(w, message, http.StatusInternalServerError)
		slog.Error(message)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonChatIds)
	slog.Info("Chat list sent")
}

func StartApiServer() {
	http.HandleFunc("/stream", streamHandler)
	http.HandleFunc("/models", modelsHandler)
	http.HandleFunc("/chat/{chatid}", loadChatHistory)
	http.HandleFunc("/chats/", chatListHandler)

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
