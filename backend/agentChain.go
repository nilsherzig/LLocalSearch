package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nilsherzig/LLocalSearch/llm_tools"
	"github.com/nilsherzig/LLocalSearch/utils"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/tools"
)

func startAgentChain(ctx context.Context, outputChan chan<- utils.HttpJsonStreamElement, clientSettings utils.ClientSettings) error {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered from panic", "error", r)
		}
	}()

	neededModels := []string{utils.EmbeddingsModel, clientSettings.ModelName}
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

	if clientSettings.Session == "default" {
		clientSettings.Session = utils.GetSessionString()
	}

	session := clientSettings.Session

	if sessions[session] == nil {
		slog.Info("Creating new session", "session", session)
		sessions[session] = memory.NewConversationBuffer()
		memory.NewChatMessageHistory()

		sessions[session].ChatHistory.AddMessage(ctx, schema.SystemChatMessage{
			Content: clientSettings.SystemMessage,
		})

		outputChan <- utils.HttpJsonStreamElement{
			StepType: utils.StepHandleNewSession,
			Session:  session,
			Stream:   false,
		}
		// TODO HACK remove this ugly workaround
		// initializes the vector db namespace
		// otherwise the go routine spam in the download func will
		// race the intialization
		sad := llm_tools.SearchVectorDB{
			CallbacksHandler: utils.CustomHandler{OutputChan: outputChan},
			SessionString:    session,
			Settings:         clientSettings,
		}
		sad.Call(ctx, "")
	}
	mem := sessions[session]

	slog.Info("Starting agent chain", "session", session, "userQuery", clientSettings)

	llm, err := utils.NewOllama(clientSettings.ModelName, clientSettings.ContextSize)
	if err != nil {
		slog.Error("Error creating new LLM", "error", err)
		return err
	}

	agentTools := []tools.Tool{
		tools.Calculator{},
		llm_tools.WebSearch{
			CallbacksHandler: utils.CustomHandler{
				OutputChan: outputChan,
			},
			SessionString: session,
			Settings:      clientSettings,
		},
		llm_tools.SearchVectorDB{
			CallbacksHandler: utils.CustomHandler{OutputChan: outputChan},
			SessionString:    session,
			Settings:         clientSettings,
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
		agents.WithMaxIterations(clientSettings.MaxIterations),
		agents.WithCallbacksHandler(utils.CustomHandler{
			OutputChan: outputChan,
		}),
		agents.WithMemory(mem),
	)
	if err != nil {
		return err
	}

	// TODO: replace this with something smarter
	// currently used to tell the frotend, that everything worked so far
	// and the request is now going to be send to ollama
	outputChan <- utils.HttpJsonStreamElement{
		StepType: utils.StepHandleOllamaStart,
	}

	temp := clientSettings.Temperature

	ans, err := chains.Run(ctx, executor, clientSettings.Prompt, chains.WithTemperature(temp))
	if err != nil {
		return err
	}
	slog.Info("Ended agent chain", "session", session, "userQuery", clientSettings, "answer", ans)

	outputChan <- utils.HttpJsonStreamElement{
		Close: true,
	}
	return nil
}
