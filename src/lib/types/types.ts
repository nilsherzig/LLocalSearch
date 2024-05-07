export type LogElement = {
    children: LogElement[];
    close: boolean;
    message: string;
    parent: LogElement;
    source: Source;
    stepLevel: number;
    stepType: StepType;
    stream: boolean;
    session: string;
    timeStamp: number;
};

export type ChatListItem = {
    sessionid: string;
    title: string;
}

export type Problem = {
    title: string
    msg: string
}

export type MetricsResponse = {
    problems: Problem[];
}

export type ClientSettings = {
    contextSize: number;
    maxIterations: number;
    modelName: string;
    prompt: string;
    temperature: number;
    toolNames: string[];
    webSearchCategories: string[];
    session: string;
    amountOfResults: number;
    minResultScore: number;
    amountOfWebsites: number;
    chunkSize: number;
    chunkOverlap: number;
    systemMessage: string;
}

export const enum StepType {
    HandleAgentAction = "HandleAgentAction",
    HandleAgentFinish = "HandleAgentFinish",
    HandleChainEnd = "HandleChainEnd",
    HandleChainError = "HandleChainError",
    HandleChainStart = "HandleChainStart",
    HandleFinalAnswer = "HandleFinalAnswer",
    HandleLLMGenerateContentEnd = "HandleLLMGenerateContentEnd",
    HandleLLMGenerateContentStart = "HandleLLMGenerateContentStart",
    HandleLlmEnd = "HandleLlmEnd",
    HandleLlmError = "HandleLlmError",
    HandleLlmStart = "HandleLlmStart",
    HandleNewSession = "HandleNewSession",
    HandleOllamaStart = "HandleOllamaStart",
    HandleParseError = "HandleParseError",
    HandleSourceAdded = "HandleSourceAdded",
    HandleToolEnd = "HandleToolEnd",
    HandleToolError = "HandleToolError",
    HandleToolStart = "HandleToolStart",
    HandleVectorFound = "HandleVectorFound",
    HandleOllamaModelLoadMessage = "HandleOllamaModelLoadMessage",
    HandleStreaming = "HandleStreaming",
    HandleUserMessage = "HandleUserMessage"
}

export type Source = {
    name: string;
    link: string;
    summary: string;
    title: string;
    engine: string;
};

