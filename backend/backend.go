package backend

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/pagination"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/mempirate/scholar/cache"
	"github.com/mempirate/scholar/log"
	"github.com/mempirate/scholar/prompt"
	"github.com/mempirate/scholar/store"
	"github.com/mempirate/scholar/util"
)

const ASSISTANT_NAME = "Scholar"
const VECTOR_STORE_NAME = "ScholarVectorStore"

// ScholarBackend is an interface for the LLM backend used by applications.
type ScholarBackend interface {
	// CreateThread creates a new thread.
	CreateThread(ctx context.Context, threadID string) error
	// UploadFile uploads a file to the vector store. threadID is only used to associate prompts with the
	// file upload. The file is uploaded to the vector store associated with the assistant.
	UploadFile(ctx context.Context, threadID, path string) error
	// Post adds a message to the thread with no response (adds more context).
	Post(ctx context.Context, threadID, text string) error
	// Prompt prompts the assistant with a message and returns the response.
	Prompt(ctx context.Context, threadID, text string) (string, error)
}

// Backend manages interactions with the OpenAI API and is responsible for
// managing the assistant, vector store, and document uploads.
type Backend struct {
	log zerolog.Logger

	client *openai.Client
	model  openai.ChatModel

	assistant *openai.Assistant
	store     *openai.VectorStore

	localStore store.LocalStore

	// threadCache is a cache that maps local IDs to openAI thread IDs.
	threadCache *cache.BoltCache
}

func NewBackend(apiKey string, model openai.ChatModel, localStore store.LocalStore) *Backend {
	log := log.NewLogger("scholar")

	log.Info().Msg("Initializing OpenAI client")
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithHeader("OpenAI-Beta", "assistants=v2"),
	)

	cache, err := cache.NewBoltCache(path.Join(localStore.Path(), "threads.db"))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create thread cache")
		return nil
	}

	return &Backend{
		log:         log,
		client:      client,
		model:       model,
		threadCache: cache,
		localStore:  localStore,
	}
}

// Init initializes the backend by getting or creating the assistant and vector store.
func (b *Backend) Init(ctx context.Context) error {
	assistant, err := b.GetOrCreateAssistant(ctx)
	if err != nil {
		return err
	}

	b.assistant = assistant

	vectorStore, err := b.GetOrCreateVectorStore(ctx, VECTOR_STORE_NAME)
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

	if err := b.syncFiles(ctx); err != nil {
		return err
	}

	return nil
}

// SyncFiles syncs the local store with the remote store.
func (b *Backend) syncFiles(ctx context.Context) error {
	fileNames, err := b.localStore.List()
	if err != nil {
		return err
	}

	remoteFiles, err := b.client.Files.List(ctx, openai.FileListParams{})
	if err != nil {
		return errors.Wrap(err, "failed to list remote files")
	}

	remoteNames := make(map[string]struct{})

	for _, file := range remoteFiles.Data {
		remoteNames[file.Filename] = struct{}{}
	}

	eg := errgroup.Group{}
	eg.SetLimit(4)

	// Sync local files to remote store
	for _, fileName := range fileNames {
		if _, ok := remoteNames[fileName]; ok {
			continue
		}

		func(fileName string) {
			eg.Go(func() error {
				b.log.Debug().Str("filename", fileName).Msg("Uploading local file")
				f, err := b.localStore.Get(fileName)
				if err != nil {
					return err

				}

				defer f.Close()

				vsFile, err := b.client.Beta.VectorStores.Files.UploadAndPoll(ctx, b.store.ID, openai.FileNewParams{
					File: openai.F[io.Reader](f),
					// Purpose of the file.
					Purpose: openai.F(openai.FilePurposeAssistants),
				}, 100)

				if err != nil {
					return errors.Wrap(err, "failed to upload document to vector store")
				}

				b.log.Info().Str("file", fileName).Str("size", util.FormatBytes(vsFile.UsageBytes)).Str("status", string(vsFile.Status)).Msg("Document uploaded")

				return nil
			})
		}(fileName)
	}

	return nil
}

func (b *Backend) GetOrCreateAssistant(ctx context.Context) (*openai.Assistant, error) {
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
		Instructions: openai.String(prompt.ASSISTANT_PROMPT_INSTRUCTIONS),
		Model:        openai.String(b.model),
		// Description:   param.Field{},
		// Metadata:      param.Field{},
		// Temperature:   param.Field{},
		Temperature: openai.Float(1),
		Tools: openai.F([]openai.AssistantToolUnionParam{
			openai.FileSearchToolParam{Type: openai.F(openai.FileSearchToolTypeFileSearch), FileSearch: openai.F(openai.FileSearchToolFileSearchParam{
				// NOTE: set max num results to 50 for now (maximum)
				// Ref. <https://platform.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings>
				MaxNumResults: openai.Int(50),
			})},
		}),
		// TopP:          param.Field{},
	})

	if err != nil {
		return nil, err
	}

	b.log.Info().Str("id", assistant.ID).Msg("Assistant created")

	return assistant, nil
}

// GetOrCreateVectorStore gets or creates a vector store for the assistant.
// The vector store is used to store document embeddings for the file search tool of the assistant.
// It will expire after 30 days of inactivity.
// https://github.com/openai/openai-go/blob/main/examples/beta/vectorstorefilebatch/main.go
func (b *Backend) GetOrCreateVectorStore(ctx context.Context, name string) (*openai.VectorStore, error) {
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

func (b *Backend) UploadFile(ctx context.Context, name string, content io.Reader) error {
	b.log.Debug().Str("name", name).Msg("Uploading document")

	vsFile, err := b.client.Beta.VectorStores.Files.UploadAndPoll(ctx, b.store.ID, openai.FileNewParams{
		File: openai.F(content),
		// Purpose of the file.
		Purpose: openai.F(openai.FilePurposeAssistants),
	}, 100)

	if err != nil {
		return errors.Wrap(err, "failed to upload document to vector store")
	}

	b.log.Info().Str("name", name).Str("size", util.FormatBytes(vsFile.UsageBytes)).Str("status", string(vsFile.Status)).Msg("Document uploaded")

	return nil
}

func (b *Backend) getFileName(ctx context.Context, id string) (string, error) {
	f, err := b.client.Files.Get(ctx, id)
	if err != nil {
		return "", errors.Wrap(err, "failed to get file")
	}

	return f.Filename, nil
}

// CreateThread creates a new thread with the given ID if it doesn't exist yet.
func (b *Backend) CreateThread(ctx context.Context, threadID string) error {
	if b.ContainsThread(threadID) {
		return nil
	}

	thread, err := b.client.Beta.Threads.New(ctx, openai.BetaThreadNewParams{})
	if err != nil {
		return errors.Wrap(err, "failed to create thread")
	}

	b.log.Debug().Str("thread_id", threadID).Str("openai_id", thread.ID).Msg("Thread created")

	b.threadCache.Put(threadID, thread.ID)
	return nil
}

func (b *Backend) ContainsThread(threadID string) bool {
	return b.threadCache.Contains(threadID)
}

func (b *Backend) Post(ctx context.Context, threadID, text string) error {
	// TODO
	return nil
}

func (b *Backend) Prompt(ctx context.Context, threadID, instructions, text string) (string, error) {
	start := time.Now()
	defer func() {
		b.log.Debug().Dur("duration", time.Since(start)).Msg("Message posted")
	}()

	thread, ok := b.threadCache.Get(threadID)
	if !ok {
		return "", errors.New("local thread not found")
	}

	_, err := b.client.Beta.Threads.Messages.New(ctx, thread, openai.BetaThreadMessageNewParams{
		Role: openai.F(openai.BetaThreadMessageNewParamsRoleUser),
		Content: openai.F([]openai.MessageContentPartParamUnion{
			openai.TextContentBlockParam{
				Type: openai.F(openai.TextContentBlockParamTypeText),
				Text: openai.String(text),
			},
		}),
	})

	if err != nil {
		return "", errors.Wrap(err, "failed to create new message")
	}

	run, err := b.client.Beta.Threads.Runs.NewAndPoll(ctx, thread, openai.BetaThreadRunNewParams{
		AssistantID: openai.String(b.assistant.ID),
		// TODO: add in config
		Instructions: openai.String(instructions),
		// NOTE: with file search, we should increase the max prompt tokens for better responses.
		// Ref: <https://platform.openai.com/docs/assistants/deep-dive#max-completion-and-max-prompt-tokens>
		MaxPromptTokens:     openai.Int(100_000),
		MaxCompletionTokens: openai.Int(30_000),
		Include:             openai.F([]openai.RunStepInclude{openai.RunStepIncludeStepDetailsToolCallsFileSearchResultsContent}),
	}, 100)

	if err != nil {
		return "", errors.Wrap(err, "failed to create new run")
	}

	fileCache := make(map[string]string)

	response := strings.Builder{}

	if run.Status == openai.RunStatusCompleted {
		eg := errgroup.Group{}

		var messages *pagination.CursorPage[openai.Message]

		eg.Go(func() error {
			messages, err = b.client.Beta.Threads.Messages.List(ctx, thread, openai.BetaThreadMessageListParams{})
			if err != nil {
				return err
			}

			// TODO: include steps and files consulted
			// steps, err = b.client.Beta.Threads.Runs.Steps.List(ctx, thread, run.ID, openai.BetaThreadRunStepListParams{
			// 	Include: openai.F([]openai.RunStepInclude{openai.RunStepIncludeStepDetailsToolCallsFileSearchResultsContent}),
			// })
			if err != nil {
				return err
			}

			return nil
		})

		if err := eg.Wait(); err != nil {
			return "", err
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

			file, err := b.getFileName(ctx, citation.FileID)
			if err != nil {
				b.log.Err(err).Int("index", index).Msg("Invalid file citation, file doesn't exist in vector store")
				continue
			}

			fileCache[citation.FileID] = file
			citations[i] = fmt.Sprintf("[%d] %s", index, file)
		}

		response.WriteString(content.Text.Value)
		if len(citations) > 0 {
			response.WriteByte('\n')
			response.WriteByte('\n')
			response.WriteString("---")
			response.WriteByte('\n')
			response.WriteString(strings.Join(citations, "\n"))
		}

		return response.String(), nil
	} else {
		err = errors.New("run not completed")
		b.log.Error().Str("status", string(run.Status)).Str("data", run.JSON.RawJSON()).Msg("Run not completed")
		return "", err
	}
}
