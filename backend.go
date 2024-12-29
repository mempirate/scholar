package main

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/mempirate/scholar/log"
)

// Backend manages interactions with the OpenAI API and is responsible for
// managing the assistant, vector store, and document uploads.
type Backend struct {
	log zerolog.Logger

	client *openai.Client

	assistant *openai.Assistant
	store     *openai.VectorStore
}

func NewBackend(apiKey string) *Backend {
	log := log.NewLogger("scholar")

	log.Info().Msg("Initializing OpenAI client")
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithHeader("OpenAI-Beta", "assistants=v2"),
	)

	return &Backend{
		log:    log,
		client: client,
	}
}

// init initializes the backend by getting or creating the assistant and vector store.
func (b *Backend) init(ctx context.Context) error {
	assistant, err := b.getOrCreateAssistant(ctx)
	if err != nil {
		return err
	}

	b.assistant = assistant

	vectorStore, err := b.getOrCreateVectorStore(ctx, VECTOR_STORE_NAME)
	if err != nil {
		return err
	}

	b.store = vectorStore

	b.log.Debug().Str("assistant_id", b.assistant.ID).Str("store_id", b.store.ID).Msg("Updating assistant with vector store")
	_, err = b.client.Beta.Assistants.Update(ctx, b.assistant.ID, openai.BetaAssistantUpdateParams{
		ToolResources: openai.F(openai.BetaAssistantUpdateParamsToolResources{
			FileSearch: openai.F(openai.BetaAssistantUpdateParamsToolResourcesFileSearch{
				VectorStoreIDs: openai.F([]string{b.store.ID}),
			}),
		}),
	})

	if err != nil {
		return err
	}

	return nil
}

func (b *Backend) getOrCreateAssistant(ctx context.Context) (*openai.Assistant, error) {
	b.log.Info().Str("name", ASSISTANT_NAME).Msg("Getting or creating assistant")
	assistants, err := b.client.Beta.Assistants.List(ctx, openai.BetaAssistantListParams{})
	if err != nil {
		return nil, err
	}

	for _, assistant := range assistants.Data {
		if assistant.Name == ASSISTANT_NAME && len(assistant.Tools) > 0 && assistant.Tools[0].Type == openai.AssistantToolTypeFileSearch {
			b.log.Debug().Msg("Existing assistant found with file_search tool")
			return &assistant, nil
		}
	}

	b.log.Debug().Msg("Creating new assistant with file_search tool")

	assistant, err := b.client.Beta.Assistants.New(ctx, openai.BetaAssistantNewParams{
		Name:         openai.String(ASSISTANT_NAME),
		Instructions: openai.String("You are a scholarly assistant helping in summarizing articles, papers, and other documents."),
		Model:        openai.String(OPENAI_MODEL),
		// Description:   param.Field{},
		// Metadata:      param.Field{},
		// Temperature:   param.Field{},
		Tools: openai.F([]openai.AssistantToolUnionParam{
			openai.FileSearchToolParam{Type: openai.F(openai.FileSearchToolTypeFileSearch)},
		}),
		// TopP:          param.Field{},
	})

	if err != nil {
		return nil, err
	}

	b.log.Info().Str("id", assistant.ID).Msg("Assistant created")

	return assistant, nil
}

// getOrCreateVectorStore gets or creates a vector store for the assistant.
// The vector store is used to store document embeddings for the file search tool of the assistant.
// It will expire after 30 days of inactivity.
// https://github.com/openai/openai-go/blob/main/examples/beta/vectorstorefilebatch/main.go
func (b *Backend) getOrCreateVectorStore(ctx context.Context, name string) (*openai.VectorStore, error) {
	stores, err := b.client.Beta.VectorStores.List(ctx, openai.BetaVectorStoreListParams{})
	if err != nil {
		return nil, err
	}

	for _, store := range stores.Data {
		if store.Name == name {
			b.log.Debug().Msg("Existing vector store found")
			return &store, nil
		}
	}

	vectorStore, err := b.client.Beta.VectorStores.New(
		ctx,
		openai.BetaVectorStoreNewParams{
			ExpiresAfter: openai.F(openai.BetaVectorStoreNewParamsExpiresAfter{
				Anchor: openai.F(openai.BetaVectorStoreNewParamsExpiresAfterAnchorLastActiveAt),
				// Expires after 30 days of inactivity
				Days: openai.Int(30),
			}),
			Name: openai.String(name),
		},
	)

	if err != nil {
		return nil, err
	}

	b.log.Info().Str("id", vectorStore.ID).Msg("Vector store created")

	return vectorStore, nil
}

func (b *Backend) uploadDocument(ctx context.Context, path string) error {
	f, err := os.Open(path)
	if err != nil {
		errors.Wrap(err, "failed to open file")
	}

	b.log.Debug().Str("path", path).Msg("Uploading document")

	vsFile, err := b.client.Beta.VectorStores.Files.UploadAndPoll(ctx, b.store.ID, openai.FileNewParams{
		File: openai.F[io.Reader](f),
		// Purpose of the file.
		Purpose: openai.F(openai.FilePurposeAssistants),
	}, 100)

	if err != nil {
		return errors.Wrap(err, "failed to upload document to vector store")
	}

	b.log.Info().Str("path", path).Str("size", formatBytes(vsFile.UsageBytes)).Str("status", string(vsFile.Status)).Msg("Document uploaded")

	return nil
}

func (b *Backend) getFileName(ctx context.Context, id string) (string, error) {
	f, err := b.client.Files.Get(ctx, id)
	if err != nil {
		return "", errors.Wrap(err, "failed to get file")
	}

	return f.Filename, nil
}

// getOrCreateThread gets or creates a thread with the given ID.
func (b *Backend) getOrCreateThread(ctx context.Context, id string) (*openai.Thread, error) {
	thread, err := b.client.Beta.Threads.Get(ctx, id)
	// If the thread does not exist, don't return the error
	if err != nil && !isNotFoundError(err) {
		return nil, err
	}

	// Double check if the thread exists
	if thread != nil {
		b.log.Debug().Str("id", thread.ID).Msg("Existing thread found")
		return thread, nil
	}

	thread, err = b.client.Beta.Threads.New(ctx, openai.BetaThreadNewParams{})
	if err != nil {
		return nil, err
	}

	b.log.Debug().Str("id", thread.ID).Msg("Thread created")

	return thread, nil
}

func isNotFoundError(err error) bool {
	return strings.Contains(err.Error(), "404")
}
