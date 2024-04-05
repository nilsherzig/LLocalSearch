package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nilsherzig/localLLMSearch/llm_tools"
	"github.com/nilsherzig/localLLMSearch/utils"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/tools"
)

func startAgentChain(ctx context.Context, outputChan chan<- utils.HttpJsonStreamElement, userQuery utils.ClientQuery) error {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered from panic", "error", r)
		}
	}()

	neededModels := []string{"nomic-embed-text:v1.5", userQuery.ModelName}
	for _, modelName := range neededModels {
		if err := utils.CheckIfModelExistsOrPull(modelName); err != nil {
			slog.Error("Model does not exist and could not be pulled", "model", modelName, "error", err)
			outputChan <- utils.HttpJsonStreamElement{
				Message:  fmt.Sprintf("Model %s does not exist and could not be pulled: %s", modelName, err.Error()),
				StepType: utils.StepHandleLlmError,
				Stream:   false,
			}
			return err
		}
	}

	startTime := time.Now()

	if userQuery.Session == "default" {
		userQuery.Session = utils.GetSessionString()
	}
	session := userQuery.Session

	if sessions[session] == nil {
		slog.Info("Creating new session", "session", session)
		sessions[session] = memory.NewConversationBuffer()
		memory.NewChatMessageHistory()
		outputChan <- utils.HttpJsonStreamElement{
			StepType: utils.StepHandleNewSession,
			Session:  session,
			Stream:   false,
		}
	}
	mem := sessions[session]

	slog.Info("Starting agent chain", "session", session, "userQuery", userQuery, "startTime", startTime)

	llm, err := utils.NewOllama(userQuery.ModelName)
	if err != nil {
		slog.Warn("Error creating new LLM", "error", err)
		return err
	}

	agentTools := []tools.Tool{
		tools.Calculator{},
		llm_tools.WebSearch{
			CallbacksHandler: utils.CustomHandler{
				OutputChan: outputChan,
			},
			SessionString: session,
		},
		llm_tools.SearchVectorDB{
			CallbacksHandler: utils.CustomHandler{
				OutputChan: outputChan,
			},
			SessionString: session,
		},
	}

	executor, err := agents.Initialize(
		llm,
		agentTools,
		agents.ConversationalReactDescription,
		agents.WithParserErrorHandler(agents.NewParserErrorHandler(func(s string) string {
			outputChan <- utils.HttpJsonStreamElement{
				Message:  fmt.Sprintf("Parsing Error. %s", s),
				StepType: utils.StepHandleParseError,
				Stream:   false,
			}
			slog.Error("Parsing Error", "error", s)
			return utils.ParsingErrorPrompt()
		})),

		agents.WithMaxIterations(userQuery.MaxIterations),
		agents.WithCallbacksHandler(utils.CustomHandler{
			OutputChan: outputChan,
		}),
		agents.WithMemory(mem),
	)

	if err != nil {
		return err
	}

	temp := 0.0
	prompt := fmt.Sprintf(`
    1. Format your answer (after AI:) in markdown. 
    2. You have to use your tools to answer questions. 
    3. You have to provide the sources / links you've used to answer the quesion.
    4. You may use tools more than once.
    5. Create your reply in the same language as the search string.
    Question: %s`, userQuery.Prompt)
	_, err = chains.Run(ctx, executor, prompt, chains.WithTemperature(temp))
	if err != nil {
		return err
	}

	outputChan <- utils.HttpJsonStreamElement{
		Close: true,
	}

	return nil
}
