package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nilsherzig/LLocalSearch/llm_tools"
	"github.com/nilsherzig/LLocalSearch/utils"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/tools"
)

func startAgentChain(ctx context.Context, outputChan chan<- utils.HttpJsonStreamElement, clientSettings utils.ClientSettings) error {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered from panic", "error", r)
		}
	}()

	if clientSettings.Session == "new" {
		clientSettings.Session = utils.GetSessionString()
		slog.Info("Created new session", "session", clientSettings.Session)
	}

	session := clientSettings.Session

	if sessions[session].Buffer == nil {
		sessions[session] = Session{
			Title:  clientSettings.Prompt,
			Buffer: memory.NewConversationWindowBuffer(clientSettings.ContextSize),
		}
		memory.NewChatMessageHistory()

		sessions[session].Buffer.ChatHistory.AddMessage(ctx, llms.SystemChatMessage{
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
		initVectorDBNamespaceVar := llm_tools.SearchVectorDB{
			CallbacksHandler: utils.CustomHandler{OutputChan: outputChan},
			SessionString:    session,
			Settings:         clientSettings,
		}
		initVectorDBNamespaceVar.Call(ctx, "")
		slog.Info("Initialized vector db namespace", "session", session)
	}
	mem := sessions[session].Buffer

	outputChan <- utils.HttpJsonStreamElement{
		StepType: utils.StepHandleUserMessage,
		Stream:   false,
		Message:  clientSettings.Prompt,
	}

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

	llm, err := utils.NewOllama(clientSettings.ModelName, clientSettings.ContextSize)
	if err != nil {
		slog.Error("Error creating new LLM", "error", err)
		return err
	}

	slog.Info("Starting agent chain", "session", session, "userQuery", clientSettings)
	startTime := time.Now()

	agentTools := []tools.Tool{
		llm_tools.WebScrape{
			CallbacksHandler: utils.CustomHandler{
				OutputChan: outputChan,
			},
			SessionString: session,
			Settings:      clientSettings,
		},

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

	mainExecutor := agents.NewExecutor(
		agents.NewConversationalAgent(llm, agentTools, agents.WithCallbacksHandler(utils.CustomHandler{OutputChan: outputChan})),
		agentTools,
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
		agents.WithMemory(mem),
	)

	// TODO: replace this with something smarter
	// currently used to tell the frotend, that everything worked so far
	// and the request is now going to be send to ollama
	outputChan <- utils.HttpJsonStreamElement{
		StepType: utils.StepHandleOllamaStart,
	}

	temp := clientSettings.Temperature

	originalAnswer, err := chains.Run(ctx, mainExecutor, clientSettings.Prompt, chains.WithTemperature(temp))

	if err != nil {
		return err
	}
	slog.Info("GotFirstAnswer", "session", session, "userQuery", clientSettings, "answer", originalAnswer, "time", time.Since(startTime))

	// answerFormat := []outputparser.ResponseSchema{
	// 	{
	// 		Name:        "quote",
	// 		Description: "The quote from the last answer",
	// 	},
	// 	{
	// 		Name:        "link",
	// 		Description: "The link to the source of the quote",
	// 	},
	// }
	//
	// parser := outputparser.NewStructured(answerFormat)
	// slog.Info("new parser", "format", parser.GetFormatInstructions())
	//
	// ans, err = chains.Run(ctx, mainExecutor, "Please provide citations for your last answer. Do not use tools. Include the codeblock quotes. "+parser.GetFormatInstructions(), chains.WithTemperature(temp))
	// if err != nil {
	// 	return err
	// }

	//   template1 := `
	// You are a playwright. Given the title of play, it is your job to write a synopsis for that title.
	// Title: {{.title}}
	// Playwright: This is a synopsis for the above play:
	// `
	//
	//   llmFormatChain := chains.LLMChain{
	//   	Prompt:           prompts.NewPromptTemplate(template1, []string{"title"}),
	//   	LLM:              llm,
	//   	Memory:           mem,
	//   	CallbacksHandler: nil,
	//   	OutputParser:     parser,
	//   	OutputKey:        "",
	//   }
	//   ans, err = llmFormatChain.
	//   if err != nil {
	//   	return err
	//   }

	// slog.Info("Ended source chain", "session", session, "userQuery", clientSettings, "answer", ans)
	// parsed, err := parser.Parse(ans)
	// if err != nil {
	// 	slog.Error("Error parsing answer", "error", err)
	// 	return err
	// }
	// slog.Info("parsed sources", "parsed", parsed)

	// _, err = llm.Call(ctx, "Please rewrite your last answer VERBATIM but replace parts of the text with markdown links to the relevant source. Do not use tools.", llms.WithTemperature(temp),
	// 	llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
	// 		outputChan <- utils.HttpJsonStreamElement{
	// 			Message:  string(chunk),
	// 			StepType: utils.StepHandleFormat,
	// 			Stream:   true,
	// 		}
	// 		return nil
	// 	}))

	// ansF, err := lschains.RunSourceChain(llm, llm_tools.UsedSources[session], originalAnswer)
	// if err != nil {
	// 	return err
	// }
	// sourcesJson, err := json.Marshal(llm_tools.UsedSources[session])
	// if err != nil {
	// 	slog.Error("Error getting sources", "error", err)
	// }
	// formatPrompt := fmt.Sprintf("Please repeat this the following old answer word for word. But replace parts of the old answer with markdown links to the relevant source. Old Answer: ```%s``` Sources: ```%s```.", originalAnswer, sourcesJson)
	// _, err = llm.Call(ctx, formatPrompt, llms.WithTemperature(temp),
	// 	llms.WithTemperature(0),
	// 	llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
	// 		outputChan <- utils.HttpJsonStreamElement{
	// 			Message:  string(chunk),
	// 			StepType: utils.StepHandleFinalAnswer,
	// 			Stream:   true,
	// 		}
	// 		return nil
	// 	}))
	//
	outputChan <- utils.HttpJsonStreamElement{
		Close: true,
	}
	slog.Info("SourcesAddedToAnswer", "session", session, "userQuery", clientSettings, "time", time.Since(startTime))
	return nil
}
