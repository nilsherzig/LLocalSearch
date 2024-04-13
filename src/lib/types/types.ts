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
};

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
    HandleUserPrompt = "HandleUserPrompt",
    HandleVectorFound = "HandleVectorFound",
    HandleOllamaModelLoadMessage = "HandleOllamaModelLoadMessage",
}

export type Source = {
    name: string;
    link: string;
    summary: string;
};

