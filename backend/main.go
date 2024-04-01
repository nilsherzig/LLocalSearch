package main

import (
	"log/slog"

	"github.com/tmc/langchaingo/memory"
)

type Sessions map[string]*memory.ConversationBuffer

var sessions Sessions = make(Sessions)

func main() {
	slog.Info("Starting the server")
	StartApiServer()
}
