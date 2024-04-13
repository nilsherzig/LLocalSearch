package utils

import "strings"

func ParsingErrorPrompt() string {
	return "Parsing Error: Check your output and make sure it conforms to the format."
}

func GenerateSystemMessage() string {
	systemMessage := []string{}
	systemMessage = append(systemMessage, "1. Format your answer in markdown.")
	systemMessage = append(systemMessage, "2. You have to use your tools to answer quesions.")
	systemMessage = append(systemMessage, "3. You have to provide sources / links you've used to answer the question.")
	systemMessage = append(systemMessage, "4. You may use tools more than once.")
	systemMessage = append(systemMessage, "5. Answer in the same language as the quesion.")

	return strings.Join(systemMessage, " ")
}
