package llm_tools

import (
	"context"
	"fmt"

	"github.com/nilsherzig/localLLMSearch/utils"
	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/tools"
)

// Feedback is a tool that can do math.
type Feedback struct {
	CallbacksHandler callbacks.Handler
	sessionString    string
	Query            utils.ClientQuery
	Llm              *ollama.LLM
}

var _ tools.Tool = Feedback{}

func (dw Feedback) Description() string {
	return `Useful for self critique. You have to use this function before submitting a final answer. You have to provide your current attempt at answering the quesion.`
}

func (dw Feedback) Name() string {
	return "Feedback"
}

func (dw Feedback) Call(ctx context.Context, input string) (string, error) {
	if dw.CallbacksHandler != nil {
		dw.CallbacksHandler.HandleToolStart(ctx, input)
	}

	newPrompt := fmt.Sprintf("Critique if the quesion: `%s` is answered with: `%s`", dw.Query.Prompt, input)
	feedback, err := dw.Llm.Call(ctx, newPrompt) // llms.WithTemperature(0.8),

	if err != nil {
		return "", err
	}

	if dw.CallbacksHandler != nil {
		dw.CallbacksHandler.HandleToolEnd(ctx, feedback)
	}

	return feedback, nil
}
