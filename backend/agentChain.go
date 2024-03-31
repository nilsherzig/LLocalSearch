package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/nilsherzig/localLLMSearch/llm_tools"
	"github.com/nilsherzig/localLLMSearch/utils"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
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

	neededModels := []string{"all-minilm", os.Getenv("OLLAMA_MODEL_NAME")}
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

	// used to set the vector db namespace
	startTime := time.Now()
	session := utils.GetSessionString()
	slog.Info("Starting agent chain", "session", session, "userQuery", userQuery, "startTime", startTime)

	// llm, err := utils.NewGPT35()
	// llm, err := utils.NewGPT4()
	llm, err := utils.NewOllamaLLM()
	// llm, err :=
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
		agents.ZeroShotReactDescription,
		agents.WithParserErrorHandler(agents.NewParserErrorHandler(func(s string) string {
			outputChan <- utils.HttpJsonStreamElement{
				Message: fmt.Sprintf("Parsing Error. %s", s),
				Stream:  false,
			}
			return utils.ParsingErrorPrompt()
		})),

		agents.WithMaxIterations(userQuery.MaxIterations),
		agents.WithCallbacksHandler(utils.CustomHandler{
			OutputChan: outputChan,
		}),
	)

	if err != nil {
		return err
	}

	temp := 0.0

	finalAnswer, err := chains.Run(ctx, executor, userQuery.Prompt, chains.WithTemperature(temp))
	if err != nil {
		return err
	}

	newAns, err := llm.Call(ctx, utils.FormatTextAsMArkdownPrompt(finalAnswer),
		llms.WithTemperature(temp),
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			outputChan <- utils.HttpJsonStreamElement{
				StepType: utils.StepHandleFinalAnswer,
				Message:  string(chunk),
				Stream:   true,
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}
	_ = newAns
	slog.Info("finished chain", "session", session, "duration", time.Since(startTime), "finalAnswer", newAns)

	return nil
}
