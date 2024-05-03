package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
)

type CustomHandler struct {
	OutputChan chan<- HttpJsonStreamElement
}

func (l CustomHandler) HandleLLMGenerateContentStart(_ context.Context, ms []llms.MessageContent) {
	l.LogDebug("Entering LLM with messages:")
	for _, m := range ms {
		var buf strings.Builder
		for _, t := range m.Parts {
			if t, ok := t.(llms.TextContent); ok {
				buf.WriteString(t.Text)
			}
		}
		l.LogDebug(fmt.Sprintf("Role: %s", m.Role))
		l.LogDebug(fmt.Sprintf("Text: %s", buf.String()))
	}
}

func (l CustomHandler) HandleLLMGenerateContentEnd(_ context.Context, res *llms.ContentResponse) {
	fmt.Println("Exiting LLM with response:")
	for _, c := range res.Choices {
		if c.Content != "" {
			l.LogDebug(fmt.Sprintf("Content: %s", c.Content))
		}
		if c.StopReason != "" {
			l.LogDebug(fmt.Sprintf("StopReason: %s", c.StopReason))
		}
		if len(c.GenerationInfo) > 0 {
			text := ""
			text += fmt.Sprintf("GenerationInfo: ")
			for k, v := range c.GenerationInfo {
				text += fmt.Sprintf("%20s: %v\n", k, v)
			}
			l.LogDebug(text)
		}
		if c.FuncCall != nil {
			l.LogDebug(fmt.Sprintf("FuncCall: %s %s", c.FuncCall.Name, c.FuncCall.Arguments))
		}
	}
}

func (l CustomHandler) LogDebug(text string) {
	// log.Println(text)
	l.OutputChan <- HttpJsonStreamElement{
		Message: text,
		Stream:  false,
	}
}

func (l CustomHandler) HandleStreamingFunc(_ context.Context, chunk []byte) {
	l.OutputChan <- HttpJsonStreamElement{
		Message:  string(chunk),
		Stream:   true,
		StepType: StepHandleStreaming,
	}
}

func (l CustomHandler) HandleText(_ context.Context, text string) {
	l.OutputChan <- HttpJsonStreamElement{
		Message: text,
		Stream:  false,
	}
}

func (l CustomHandler) HandleLLMStart(_ context.Context, prompts []string) {
	l.OutputChan <- HttpJsonStreamElement{
		Message:  fmt.Sprintf("Entering LLM with prompts: %s", prompts),
		Stream:   false,
		StepType: StepHandleLlmStart,
	}
}

func (l CustomHandler) HandleLLMError(_ context.Context, err error) {
	fmt.Println("Exiting LLM with error:", err)
	l.OutputChan <- HttpJsonStreamElement{
		Message: err.Error(),
		Stream:  false,
	}
}

func (l CustomHandler) HandleChainStart(_ context.Context, inputs map[string]any) {
	chainValuesJson, err := json.Marshal(inputs)
	if err != nil {
		fmt.Println("Error marshalling chain values:", err)
	}

	charCount := utf8.RuneCountInString(string(chainValuesJson))
	slog.Info("Entering chain", "tokens", (charCount / 4))

	l.OutputChan <- HttpJsonStreamElement{
		Message:  fmt.Sprintf("Entering chain with %d tokens: %s", (charCount / 4), chainValuesJson),
		Stream:   false,
		StepType: StepHandleChainStart,
	}
}

func (l CustomHandler) HandleChainEnd(_ context.Context, outputs map[string]any) {
	chainValuesJson, err := json.Marshal(outputs)
	if err != nil {
		fmt.Println("Error marshalling chain values:", err)
	}
	l.OutputChan <- HttpJsonStreamElement{
		Message:  fmt.Sprintf("Exiting chain with outputs: %s", chainValuesJson),
		Stream:   false,
		StepType: StepHandleChainEnd,
	}
}

func (l CustomHandler) HandleChainError(_ context.Context, err error) {
	message := fmt.Sprintf("Exiting chain with error: %v", err)
	fmt.Println(message)
	l.OutputChan <- HttpJsonStreamElement{
		Message:  message,
		Stream:   false,
		StepType: StepHandleChainError,
	}
}

func (l CustomHandler) HandleToolStart(_ context.Context, input string) {
	l.OutputChan <- HttpJsonStreamElement{
		Message:  fmt.Sprintf("Entering tool with input: %s", removeNewLines(input)),
		Stream:   false,
		StepType: StepHandleToolStart,
	}
}

func (l CustomHandler) HandleToolEnd(_ context.Context, output string) {
	l.OutputChan <- HttpJsonStreamElement{
		Message:  fmt.Sprintf("Exiting tool with output: %s", removeNewLines(output)),
		Stream:   false,
		StepType: StepHandleToolEnd,
	}
}

func (l CustomHandler) HandleToolError(_ context.Context, err error) {
	fmt.Println("Exiting tool with error:", err)
	l.OutputChan <- HttpJsonStreamElement{
		Message: err.Error(),
		Stream:  false,
	}
}

func (l CustomHandler) HandleAgentAction(_ context.Context, action schema.AgentAction) {
	actionJson, err := json.Marshal(action)
	if err != nil {
		fmt.Println("Error marshalling action:", err)
	}

	l.OutputChan <- HttpJsonStreamElement{
		Message:  string(actionJson),
		Stream:   false,
		StepType: StepHandleAgentAction,
	}
}

func (l CustomHandler) HandleAgentFinish(_ context.Context, finish schema.AgentFinish) {
	finishJson, err := json.Marshal(finish)
	if err != nil {
		fmt.Println("Error marshalling finish:", err)
	}
	l.OutputChan <- HttpJsonStreamElement{
		Message:  string(finishJson),
		Stream:   false,
		StepType: StepHandleAgentFinish,
	}
}

func (l CustomHandler) HandleRetrieverStart(_ context.Context, query string) {
	fmt.Println("Entering retriever with query:", removeNewLines(query))
}

func (l CustomHandler) HandleRetrieverEnd(_ context.Context, query string, documents []schema.Document) {
	// fmt.Println("Exiting retriever with documents for query:", documents, query)
	l.OutputChan <- HttpJsonStreamElement{
		Message:  fmt.Sprintf("Exiting retriever with documents for query: %s", query),
		Stream:   false,
		StepType: StepHandleRetriverEnd,
	}
}

func (l CustomHandler) HandleVectorFound(_ context.Context, vectorString string) {
	l.OutputChan <- HttpJsonStreamElement{
		Message:  fmt.Sprintf("Found vector %s", vectorString),
		Stream:   false,
		StepType: StepHandleVectorFound,
	}
}

func (l CustomHandler) HandleSourceAdded(_ context.Context, source Source) {
	l.OutputChan <- HttpJsonStreamElement{
		Message:  "Source added",
		Source:   source,
		Stream:   false,
		StepType: StepHandleSourceAdded,
	}
}

func formatChainValues(values map[string]any) string {
	output := ""
	for key, value := range values {
		output += fmt.Sprintf("\"%s\" : \"%s\", ", removeNewLines(key), removeNewLines(value))
	}

	return output
}

func formatAgentAction(action schema.AgentAction) string {
	return fmt.Sprintf("\"%s\" with input \"%s\"", removeNewLines(action.Tool), removeNewLines(action.ToolInput))
}

func removeNewLines(s any) string {
	return strings.ReplaceAll(fmt.Sprint(s), "\n", " ")
}
