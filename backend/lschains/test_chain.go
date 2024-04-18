package lschains

import (
	"context"
	"log"
	"log/slog"
	"strings"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/tools"
)

func RunTestChain() {
	slog.Info("Running test chain")
	llm, err := ollama.New(
		ollama.WithModel("llama3:8b-instruct-q6_K"),
		// ollama.WithModel("adrienbrault/nous-hermes2pro:Q6_K"),
		ollama.WithServerURL("http://gpu-ubuntu:11434"),
		ollama.WithRunnerNumCtx(8*1024),
	)
	if err != nil {
		return
	}

	llmTools := []tools.Tool{tools.Calculator{}}

	executor := agents.NewExecutor(
		agents.NewConversationalAgent(llm, llmTools),
		llmTools,
		agents.WithParserErrorHandler(agents.NewParserErrorHandler(
			func(s string) string {
				return s
			},
		)),
		agents.WithCallbacksHandler(callbacks.StreamLogHandler{}),
		agents.WithMemory(memory.NewConversationBuffer()),
	)
	ans1, err := chains.Run(context.Background(), executor, "Hi! my name is Bob and the year I was born is 1987.",
		chains.WithTemperature(0.0))
	if err != nil {
		log.Fatal(err)
	}
	slog.Info("Answer 1", "Answer", ans1)

	ans2, err := chains.Run(context.Background(), executor, "What is the year I was born times 34. Use tools. Only answer with the number, nothing else.")
	if err != nil {
		slog.Error("Answer 2", "Error", err)
		return
	}
	expectedRe := "67558"
	if !strings.Contains(ans2, expectedRe) && !strings.Contains(ans2, "67,558") {
		slog.Error("Answer 2", "Answer", ans2, "Expected", expectedRe)
		return
	}
	slog.Info("Answer 2", "Answer", ans2, "Expected", expectedRe)
}
