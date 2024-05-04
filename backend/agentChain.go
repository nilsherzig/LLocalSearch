package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/nilsherzig/LLocalSearch/llm_tools"
	"github.com/nilsherzig/LLocalSearch/utils"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/tools"
	"github.com/tmc/langchaingo/vectorstores/chroma"
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
		llm, err := utils.NewOllamaEmbeddingLLM()
		if err != nil {
			return err
		}
		embeder, err := embeddings.NewEmbedder(llm)
		if err != nil {
			return err
		}
		_, errNs := chroma.New(
			chroma.WithChromaURL(os.Getenv("CHROMA_DB_URL")),
			chroma.WithEmbedder(embeder),
			chroma.WithDistanceFunction("cosine"),
			chroma.WithNameSpace(session),
		)
		if errNs != nil {
			slog.Error("Error creating new vector db namespace", "error", errNs)
			return errNs
		}
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

	messages, err := mem.ChatHistory.Messages(ctx)
	if err != nil {
		return err
	}

	ans, err := llm.Call(ctx, fmt.Sprintf("Please create a three (3) word title for the following conversation. Dont write anything else. Respond in the following Fromat `title: [your 3 word title]`. Conversation: ```%v```", messages))
	if err != nil {
		return err
	}

	slog.Info("GotTitleAnswer", "session", session, "answer", ans, "time", time.Since(startTime))
	oldSession := sessions[session]
	oldSession.Title = ans
	sessions[session] = oldSession

	outputChan <- utils.HttpJsonStreamElement{
		Close:   true,
		Message: sessions[session].Title,
	}
	return nil
}
