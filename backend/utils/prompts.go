package utils

import "strings"

func ParsingErrorPrompt() string {
	return "Parsing Error: Check your output and make sure it conforms to the format."
}

func GenerateSystemMessage() string {
	systemMessage := []string{}
	systemMessage = append(systemMessage, "Strictly structure your answers in markdown for enhanced readability and organization.")

	systemMessage = append(systemMessage, "Mandatory: Employ all available tools to formulate well-informed and accurate responses. Guesswork is not permitted.")

	// systemMessage = append(systemMessage, "First consult VectorDB for existing insights on ongoing topics before considering a new web search. This step is crucial for efficient and informed response generation.")

	systemMessage = append(systemMessage, "Mandatory: All answers must include cited sources to ensure the information provided is traceable and credible.")

	systemMessage = append(systemMessage, "It is imperative to reply in the same language as the query. This ensures clarity and relevance in communication.")

	systemMessage = append(systemMessage, "For optimal search results, formulate queries concisely and use specific keywords related to the inquiry. Avoid vague terms to ensure the precision and relevance of the results.")
	return strings.Join(systemMessage, " ")
}
