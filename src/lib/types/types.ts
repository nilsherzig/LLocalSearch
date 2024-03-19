export type LogElement = {
    children: LogElement[];
    close: boolean;
    message: string;
    parent: LogElement;
    source: Source;
    stepLevel: number;
    stepType: StepType;
    stream: boolean;
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
    HandleSourceAdded = "HandleSourceAdded",
    HandleToolEnd = "HandleToolEnd",
    HandleToolError = "HandleToolError",
    HandleToolStart = "HandleToolStart",
    HandleVectorFound = "HandleVectorFound",
}

export type Source = {
    name: string;
    link: string;
    summary: string;
};

