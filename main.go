package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go"
)

const ASSISTANT_NAME = "Scholar"
const VECTOR_STORE_NAME = "ScholarVectorStore"
const OPENAI_MODEL = openai.ChatModelGPT4oMini

func main() {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		panic("OPENAI_API_KEY is not set")
	}

	backend := NewBackend(key)
	log := backend.log

	ctx := context.Background()

	if err := backend.init(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize backend")
	}

	log.Info().Msg("Backend initialized")

	// backend.uploadDocument(ctx, "testdata/FastPay.pdf")

	thread, err := backend.client.Beta.Threads.New(ctx, openai.BetaThreadNewParams{})
	if err != nil {
		panic(err)
	}

	// Create a message in the thread
	_, err = backend.client.Beta.Threads.Messages.New(ctx, thread.ID, openai.BetaThreadMessageNewParams{
		Role: openai.F(openai.BetaThreadMessageNewParamsRoleUser),
		Content: openai.F([]openai.MessageContentPartParamUnion{
			openai.TextContentBlockParam{
				Type: openai.F(openai.TextContentBlockParamTypeText),
				Text: openai.String("Can you summarize the FastPay paper? What is special about it?"),
			},
		}),
	})
	if err != nil {
		panic(err)
	}

	// Create a run
	run, err := backend.client.Beta.Threads.Runs.NewAndPoll(ctx, thread.ID, openai.BetaThreadRunNewParams{
		AssistantID:  openai.String(backend.assistant.ID),
		Instructions: openai.String("You are a scholarly assistant helping in summarizing articles, papers, and other documents. Use your vector store to respond to questions. Be concise to reduce token usage."),
	}, 100)

	if err != nil {
		panic(err)
	}

	fileCache := make(map[string]string)

	if run.Status == openai.RunStatusCompleted {
		messages, err := backend.client.Beta.Threads.Messages.List(ctx, thread.ID, openai.BetaThreadMessageListParams{})

		if err != nil {
			panic(err.Error())
		}

		// The first message is the response
		message := messages.Data[0]

		content := message.Content[0]

		citations := make([]string, len(content.Text.Annotations))

		for i, annotation := range content.Text.Annotations {
			index := i + 1
			content.Text.Value = strings.Replace(content.Text.Value, annotation.Text, fmt.Sprintf(" [%d]", index), 1)
			citation := annotation.FileCitation.(openai.FileCitationAnnotationFileCitation)
			if file, ok := fileCache[citation.FileID]; ok {
				citations[i] = fmt.Sprintf("[%d] %s", index, file)
				continue
			}

			file, err := backend.getFileName(ctx, citation.FileID)
			if err != nil {
				log.Err(err).Int("index", index).Msg("Invalid file citation, file doesn't exist in vector store")
				continue
			}

			fileCache[citation.FileID] = file
			citations[i] = fmt.Sprintf("[%d] %s", index, file)
		}

		println(content.Text.Value)
		println()
		println(strings.Join(citations, "\n"))
	} else {
		println("Run not completed", run.Status)
		println(run.JSON.RawJSON())
	}

}
