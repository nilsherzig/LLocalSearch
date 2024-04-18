package utils

type ChatListItem struct {
	SessionId string `json:"sessionid"`
	Title     string `json:"title"`
}

type ChatHistoryItem struct {
	Element HttpJsonStreamElement `json:"element"`
}

type HttpError struct {
	Error string `json:"error"`
}

type HttpJsonStreamElement struct {
	Message  string   `json:"message"`
	Close    bool     `json:"close"`
	Stream   bool     `json:"stream"`
	StepType StepType `json:"stepType"`
	Source   Source   `json:"source"`
	Session  string   `json:"session"`
}

type ClientSettings struct {
	ContextSize         int      `json:"contextSize"`
	MaxIterations       int      `json:"maxIterations"`
	ModelName           string   `json:"modelName"`
	Prompt              string   `json:"prompt"`
	Session             string   `json:"session"`
	Temperature         float64  `json:"temperature"`
	ToolNames           []string `json:"toolNames"`
	WebSearchCategories []string `json:"webSearchCategories"`
	AmountOfResults     int      `json:"amountOfResults"`
	MinResultScore      float64  `json:"minResultScore"`
	AmountOfWebsites    int      `json:"amountOfWebsites"`
	ChunkSize           int      `json:"chunkSize"`
	ChunkOverlap        int      `json:"chunkOverlap"`
	SystemMessage       string   `json:"systemMessage"`
}

type StepType string

const (
	StepHandleAgentAction             StepType = "HandleAgentAction"
	StepHandleAgentFinish             StepType = "HandleAgentFinish"
	StepHandleChainEnd                StepType = "HandleChainEnd"
	StepHandleChainError              StepType = "HandleChainError"
	StepHandleChainStart              StepType = "HandleChainStart"
	StepHandleFinalAnswer             StepType = "HandleFinalAnswer"
	StepHandleLLMGenerateContentEnd   StepType = "HandleLLMGenerateContentEnd"
	StepHandleLLMGenerateContentStart StepType = "HandleLLMGenerateContentStart"
	StepHandleLlmEnd                  StepType = "HandleLlmEnd"
	StepHandleLlmError                StepType = "HandleLlmError"
	StepHandleLlmStart                StepType = "HandleLlmStart"
	StepHandleNewSession              StepType = "HandleNewSession"
	StepHandleOllamaStart             StepType = "HandleOllamaStart"
	StepHandleParseError              StepType = "HandleParseError"
	StepHandleRetriverEnd             StepType = "HandleRetriverEnd"
	StepHandleRetriverStart           StepType = "HandleRetriverStart"
	StepHandleSourceAdded             StepType = "HandleSourceAdded"
	StepHandleToolEnd                 StepType = "HandleToolEnd"
	StepHandleToolError               StepType = "HandleToolError"
	StepHandleToolStart               StepType = "HandleToolStart"
	StepHandleVectorFound             StepType = "HandleVectorFound"
	StepHandleFormat                  StepType = "HandleFormat"
	StepHandleStreaming               StepType = "HandleStreaming"
	StepHandleUserMessage             StepType = "HandleUserMessage"
)

type Source struct {
	Name    string `json:"name"`
	Link    string `json:"link"`
	Summary string `json:"summary"`
	Engine  string `json:"engine"`
	Title   string `json:"title"`
}

type SeaXngResult struct {
	Query           string `json:"query"`
	NumberOfResults int    `json:"number_of_results"`
	Results         []struct {
		URL           string   `json:"url"`
		Title         string   `json:"title"`
		Content       string   `json:"content"`
		PublishedDate any      `json:"publishedDate,omitempty"`
		ImgSrc        any      `json:"img_src,omitempty"`
		Engine        string   `json:"engine"`
		ParsedURL     []string `json:"parsed_url"`
		Template      string   `json:"template"`
		Engines       []string `json:"engines"`
		Positions     []int    `json:"positions"`
		Score         float64  `json:"score"`
		Category      string   `json:"category"`
	} `json:"results"`
	Answers             []any    `json:"answers"`
	Corrections         []any    `json:"corrections"`
	Infoboxes           []any    `json:"infoboxes"`
	Suggestions         []string `json:"suggestions"`
	UnresponsiveEngines []any    `json:"unresponsive_engines"`
}
