package main

import "fmt"

const SUMMARY_PROMPT = "Please provide a summary of this file: %s. In case of a tweet, be careful about retrieving the correct file, with the exact same name. If the tweet has any references, include those in the summary if relevant. In case of a PDF, always mention the title of the paper instead of the name. Only use a single reference per unique file."

func createSummaryPrompt(filename string) string {
	return fmt.Sprintf(SUMMARY_PROMPT, filename)
}

const MENTION_PROMPT = `Your context is a series of chat messages. Each message is a JSON object with a 'userId' property, indicating
the user who sent the message and their user ID. The 'text' property contains the message text. The 'timestamp' property contains the time the message was sent.
The 'threadId' property contains the ID of the thread the message belongs to. Threads are conversations that are started by a user and can be replied to by other users,
with specific context. The 'channelId' property contains the ID of the channel the message was sent in. Take into account the context of the messages and the thread they belong to.
Messages are inserted into your vector store in the order they are received. It's important you give recent messages more precedence and importance than older messages.
You can mention a user in a response by using the following schema: <@userId> (including the smaller / greater than signs). Always do this if it is relevant, or if you
have to refer to messages sent by specific users.
With all this in mind, please answer the following question:

%s

channelId: %s
threadId: %s
userId: %s`

func createMentionPrompt(question, channel, thread, userID string) string {
	return fmt.Sprintf(MENTION_PROMPT, question, channel, thread, userID)
}
