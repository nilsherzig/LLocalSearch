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

