package utils

type HttpJsonStreamElement struct {
	Close    bool     `json:"close"`
	Message  string   `json:"message"`
	Stream   bool     `json:"stream"`
	StepType StepType `json:"stepType"`
	Source   Source   `json:"source"`
	Session  string   `json:"session"`
}

type ClientQuery struct {
	Prompt        string `json:"prompt"`
	MaxIterations int    `json:"maxIterations"`
	ModelName     string `json:"modelName"`
	Session       string `json:"session"`
}

type StepType string

const (
	StepHandleNewSession  StepType = "HandleNewSession"
	StepHandleAgentAction StepType = "HandleAgentAction"
	StepHandleAgentFinish StepType = "HandleAgentFinish"

	StepHandleToolStart StepType = "HandleToolStart"
	StepHandleToolEnd   StepType = "HandleToolEnd"
	StepHandleToolError StepType = "HandleToolError"

	StepHandleChainStart StepType = "HandleChainStart"
	StepHandleChainEnd   StepType = "HandleChainEnd"
	StepHandleChainError StepType = "HandleChainError"

	StepHandleLlmStart StepType = "HandleLlmStart"
	StepHandleLlmEnd   StepType = "HandleLlmEnd"
	StepHandleLlmError StepType = "HandleLlmError"

	StepHandleLLMGenerateContentStart StepType = "HandleLLMGenerateContentStart"
	StepHandleLLMGenerateContentEnd   StepType = "HandleLLMGenerateContentEnd"

	StepHandleSourceAdded StepType = "HandleSourceAdded"
	StepHandleVectorFound StepType = "HandleVectorFound"

	StepHandleFinalAnswer StepType = "HandleFinalAnswer"

	StepHandleRetriverStart StepType = "HandleRetriverStart"
	StepHandleRetriverEnd   StepType = "HandleRetriverEnd"

	StepHandleParseError StepType = "HandleParseError"
)

type Source struct {
	Name    string `json:"name"`
	Link    string `json:"link"`
	Summary string `json:"summary"`
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
