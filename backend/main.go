package main

import (
	"log/slog"

	"github.com/tmc/langchaingo/memory"
)

type Sessions map[string]*memory.ConversationBuffer

var sessions Sessions = make(Sessions)

// TODO: remove used linkes list
// combine the search and vector db somehow, like some sort of caching
func main() {
	slog.Info("Starting the server")
	StartApiServer()
}
