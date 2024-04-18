package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
	"github.com/nilsherzig/LLocalSearch/utils"
	"github.com/tmc/langchaingo/memory"
)

type Session struct {
	Title    string
	Buffer   *memory.ConversationWindowBuffer
	Elements []utils.HttpJsonStreamElement
}

type Sessions map[string]Session

var sessions Sessions = make(Sessions)

func main() {
	w := os.Stderr
	slog.SetDefault(slog.New(
		tint.NewHandler(w, &tint.Options{
			AddSource: true,
			Level:     slog.LevelDebug,
		}),
	))

	exampleUuid := "how-are-you"
	sessions[exampleUuid] = Session{
		Title:  "How are you?",
		Buffer: memory.NewConversationWindowBuffer(1024 * 8),
	}
	newFakeSession := Session{
		Title:  "How are you?",
		Buffer: memory.NewConversationWindowBuffer(1024 * 8),
	}
	newFakeSession.Buffer.ChatHistory.AddUserMessage(context.Background(), "How are you?")
	newFakeSession.Buffer.ChatHistory.AddAIMessage(context.Background(), "I'm fine, thank you.")
	newFakeSession.Elements = []utils.HttpJsonStreamElement{
		{
			Message: "How are you?",
			Close:   false,
			Stream:  false,
		},
		{
			Message:  "I'm fine, thank you.",
			Close:    false,
			Stream:   true,
			StepType: utils.StepHandleFinalAnswer,
		},
	}
	sessions["how-are-you"] = newFakeSession
	slog.Info("created example session")

	// lschains.RunSourceChainExample()
	slog.Info("Starting the server")
	StartApiServer()
}
