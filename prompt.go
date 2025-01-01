package main

import "fmt"

const SUMMARY_PROMPT = "Please provide a summary of this file: %s. In case of a tweet, be careful about retrieving the correct file, with the exact same name. If the tweet has any references, include those in the summary if relevant. In case of a PDF, always mention the title of the paper instead of the name. Only use a single reference per unique file."

func createSummaryPrompt(filename string) string {
	return fmt.Sprintf(SUMMARY_PROMPT, filename)
}
