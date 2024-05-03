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

	SetupTutorialChatHistory()
	slog.Info("created example session")

	// lschains.RunSourceChainExample()
	slog.Info("Starting the server")
	StartApiServer()
}

func SetupTutorialChatHistory() {
	newFakeSession := Session{
		Title:  "LLocalSearch Tutorial",
		Buffer: memory.NewConversationWindowBuffer(1024 * 8),
	}
	userQuestion := "How does LLocalSearch work?"
	newFakeSession.Buffer.ChatHistory.AddUserMessage(context.Background(), userQuestion)

	tutorialMessageOne := `## Welcome to the LLocalSearch tutorial.
### How it works

asdasd

### What you can do

### How to get started

### Customizing your experience

### Contributing / Reporting issues
`
	newFakeSession.Buffer.ChatHistory.AddAIMessage(context.Background(), tutorialMessageOne)

	newFakeSession.Elements = []utils.HttpJsonStreamElement{
		{
			Message:  "How does LLocalSearch work?",
			Close:    false,
			Stream:   false,
			StepType: utils.StepHandleUserMessage,
		},
		{
			Message:  tutorialMessageOne,
			Close:    false,
			Stream:   true,
			StepType: utils.StepHandleFinalAnswer,
		},
	}
	sessions["tutorial"] = newFakeSession
}
