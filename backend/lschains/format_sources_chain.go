package lschains

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/prompts"
	"github.com/tmc/langchaingo/schema"
)

const (
	PartKey   = "Quote"
	SourceKey = "ID"
)

const outputKey = "text"
const _formatPromptTemplate = `Its your task to enhance the "old answer" with sources. To do this, you pick a quote from the old answer and find a source that supports it.

The old Answer without sources (only quote from this): 
-----------------
{{.oldAnswer}}
-----------------

Sources:
-----------------
{{.sources}}
-----------------

{{.format}}
Provide at least three quotes from the sources and the source ID.

Your answer:
`

func RunSourceChain(llm *ollama.LLM, sources []schema.Document, textWithoutSources string) (string, error) {
	startTime := time.Now()

	// reducing the amount of tokens in the context window
	sourceContentIdMap := make(map[string]string)
	for i, source := range sources {
		sourceContentIdMap[fmt.Sprintf("s%d", i)] = source.PageContent
	}

	// converting the go map to json, since llms have seen a lot more json
	// during training - resulting in better responses
	sourcesJson, err := json.Marshal(sourceContentIdMap)
	if err != nil {
		slog.Error("Error marshalling source map", "error", err)
		return "", err
	}

	rs := []ResponseSchema{
		{
			Name:        PartKey,
			Description: "A very short (3 words) verbatim quote from the old answer.",
		},
		{
			Name:        SourceKey,
			Description: "The related source ID.",
		},
	}

	parser := NewStructured(rs)

	cps := chains.ConditionalPromptSelector{
		DefaultPrompt: prompts.NewPromptTemplate(_formatPromptTemplate, []string{"format", "sources", "oldAnswer"}),
	}

	formatChain := chains.LLMChain{
		Prompt:           cps.DefaultPrompt,
		LLM:              llm,
		Memory:           memory.NewConversationBuffer(),
		CallbacksHandler: callbacks.LogHandler{},
		OutputParser:     parser,
		OutputKey:        outputKey,
	}

	valueMap, err := chains.Call(context.Background(), formatChain, map[string]any{
		"format":    parser.GetFormatInstructions(),
		"sources":   string(sourcesJson),
		"oldAnswer": textWithoutSources,
	},
		chains.WithTemperature(0.0),
		chains.WithTopK(1),
	)

	out, ok := valueMap[outputKey].([]ResponseSchema)
	if !ok {
		slog.Error("type assertion failed", "value", valueMap[outputKey])
		return "", fmt.Errorf("type assertion failed")
	}
	matches := 0
	failed := 0
	textWithSources := textWithoutSources
	for _, o := range out {
		sourceURL := ""
		sourceContent := sourceContentIdMap[o.Description]
		for _, source := range sources {
			if source.PageContent == sourceContent {
				sourceURL = source.Metadata["URL"].(string)
				break
			}
		}

		if !strings.Contains(textWithSources, o.Name) {
			slog.Warn("quote not found in text", "quote", o.Name)
			failed++
			continue
		}
		matches++

		textWithSources = strings.Replace(textWithSources,
			o.Name,
			fmt.Sprintf("%s [source](%s)", o.Name, sourceURL),
			1,
		)

	}
	slog.Info("Sources mapped", "amount", matches, "failed", failed)
	slog.Info("added sources", "text", textWithSources)
	slog.Info("Time taken to run the chain", "time", time.Since(startTime))
	return textWithSources, nil
}
