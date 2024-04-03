package main

import (
	"context"
	"fmt"
	"log"
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
		// send a close message to the client when the function ends
		// the client can use this to close the connection gracefully
		// outputChan <- utils.HttpJsonStreamElement{
		// 	Close: true,
		// }

		// this defer acts as a try catch block around the current function
		// this prevents the whole server from crashing when an error occurs

		// TODO: figure out how to stop the "sending on closed channel" error
		// when the client disconnects
		if r := recover(); r != nil {
			log.Printf("Recovered from panic: %v", r)
		}
	}()
	// TODO: move this check into the agent chain, if switching model in interface becomes a thing

	neededModels := []string{"all-minilm:v2", userQuery.ModelName}
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

	// used to set the vector db namespace and chat memory
	if userQuery.Session == "" {
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

	// llm, err := utils.NewGPT35()
	// llm, err := utils.NewGPT4()
	llm, err := utils.NewOllama(userQuery.ModelName)
	if err != nil {
		log.Printf("Error creating new LLM: %v", err)
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
		// llm_tools.Feedback{
		// 	CallbacksHandler: utils.CustomHandler{},
		// 	Query:            userQuery,
		// 	Llm:              llm,
		// },
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
    1. Fromat your answer (after AI:) in markdown. 
    2. You have to use your tools to answer questions. 
    3. You have to provide the sources / links you've used to answer the quesion.
    Question: %s`, userQuery.Prompt)
	_, err = chains.Run(ctx, executor, prompt, chains.WithTemperature(temp))
	if err != nil {
		return err
	}

	messages, err := mem.ChatHistory.Messages(ctx)
	if err != nil {
		return err
	}
	log.Printf("mem messages %v", messages)

	// outputChan <- utils.HttpJsonStreamElement{
	// 	Message:  ans,
	// 	StepType: utils.StepHandleFinalAnswer,
	// 	Stream:   false,
	// }

	outputChan <- utils.HttpJsonStreamElement{
		Close: true,
	}
	return nil
}
