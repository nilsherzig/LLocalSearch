package utils

import "fmt"

func ParsingErrorPrompt() string {
	return "Parsing Error: Check your output and make sure it conforms, use the Action/Action Input syntax. Use `Final Answer: [Your last answer] if youre finished"
}

func FormatTextAsMArkdownPrompt(text string) string {
	// return fmt.Sprintf("Format the following text in fancy markdown: '%s'. Prefer lists or tables over normal text. Only write the formatted text, do not write something like 'Here is the fancy text:' just write the text. Dont surround your answer with a codeblock. Use all information provided. Dont write like a human, thats scarry.", text)
	return fmt.Sprintf("Format the following text in fancy markdown: '%s'. Only write the formatted text, do not write something like 'Here is the fancy text:' just write the text. Dont surround your answer with a codeblock. Use all information provided. Dont write like a human, thats scarry.", text)
}
