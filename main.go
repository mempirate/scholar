package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/mempirate/scholar/backend"
	"github.com/mempirate/scholar/content"
	"github.com/mempirate/scholar/log"
	"github.com/mempirate/scholar/slack"
	"github.com/mempirate/scholar/store"
	"github.com/openai/openai-go"
)

const OPENAI_MODEL = openai.ChatModelGPT4oMini

var (
	dataDir = flag.String("data-dir", defaultDataDir(), "Directory to store learned file data. This directory will mirror what's in the vector store.")
)

func main() {
	flag.Parse()

	key, appToken, botToken := os.Getenv("OPENAI_API_KEY"), os.Getenv("SLACK_APP_TOKEN"), os.Getenv("SLACK_BOT_TOKEN")
	if key == "" || appToken == "" || botToken == "" {
		panic("OPENAI_API_KEY || SLACK_APP_TOKEN || SLACK_BOT_TOKEN is not set")
	}

	log := log.NewLogger("main")

	// Expand environment variables in datadir
	dataDir := os.ExpandEnv(*dataDir)
	fileStore := store.NewFileStore(dataDir)

	log.Info().Str("dataDir", dataDir).Msg("Using data directory")

	// Ensure datadir exists
	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		panic(err)
	}

	backend := backend.NewBackend(key, OPENAI_MODEL, fileStore)

	ctx := context.Background()

	if err := backend.Init(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize backend")
	}
	log.Info().Msg("Backend initialized")

	globalThreadFile := path.Join(dataDir, "global-thread.txt")

	b, err := os.ReadFile(globalThreadFile)
	globalThread := string(b)
	if err != nil {
		if os.IsNotExist(err) {
			globalThread, err = backend.CreateGlobalThread(ctx)
			if err := os.WriteFile(globalThreadFile, []byte(globalThread), 0644); err != nil {
				log.Fatal().Str("thread_id", globalThread).Err(err).Msg("Failed to write global thread file")
			}
		} else {
			log.Fatal().Err(err).Msg("Failed to read global thread file")
		}
	}

	log.Info().Str("thread_id", globalThread).Msg("Using global thread")

	slackHandler := slack.NewSlackHandler(appToken, botToken)
	commands := slackHandler.SubscribeCommands()
	events := slackHandler.SubscribeEvents()

	go slackHandler.Start()

	contentHandler := content.NewContentHandler()

	for {
		select {
		case cmd := <-commands:
			// Handle content extraction
			content, err := contentHandler.HandleURL(cmd.URL)
			if err != nil {
				log.Error().Err(err).Msg("Failed to handle URL")
				slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, fmt.Sprintf("Failed to handle URL: %s (%s)", cmd.URL, err))
				continue
			}

			// We only de-duplicate uploads. If someone wants to summarize a file that already exists, we'll allow it, but we won't upload it again.
			if cmd.CommandType == slack.UploadCommand {
				// Handle content de-duplication
				contains, err := fileStore.Contains(content.Name)
				if err != nil {
					log.Error().Err(err).Msg("Failed to check if content exists")
					continue
				}

				if contains {
					log.Info().Str("name", content.Name).Msg("File already exists, skipping")
					slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, "This file already exists.")
					continue
				}
			}

			r := bytes.NewReader(content.Content)
			if err := fileStore.Store(content.Name, r); err != nil {
				log.Error().Err(err).Msg("Failed to store file locally")
				slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, err.Error())
				continue
			}

			file, err := fileStore.Get(content.Name)

			if err := backend.UploadFile(ctx, content.Name, file); err != nil {
				log.Error().Err(err).Msg("Failed to upload file")
				slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, err.Error())
				continue
			}

			file.Close()

			threadID, err := slackHandler.StartUploadThread(cmd.ChannelID, cmd.UserID, fmt.Sprintf("%s [%s]", content.Name, content.URL))
			if err := backend.CreateThread(ctx, threadID); err != nil {
				log.Error().Err(err).Msg("Failed to create thread")
				continue
			}

			if cmd.CommandType == slack.SummarizeCommand {
				summary, err := backend.Prompt(ctx, threadID, createSummaryPrompt(content.Name))
				if err != nil {
					log.Error().Err(err).Msg("Failed to prompt for summary")
					continue
				}

				if err := slackHandler.PostMessage(cmd.ChannelID, &threadID, summary); err != nil {
					log.Error().Err(err).Msg("Failed to post summary")
					continue
				}
			}
		case event := <-events:
			switch event.Type {
			case slack.MessageEvent:
				// TODO:
				// Upload messages to the assistant context as JSON objects:
				// {
				//  "type": "message",
				// 	"channelId": "C01B2PZQX1Z",
				// 	"threadId": "1635732824.000100",
				//  "userId": "U01B2PZQX1Z",
				//  "text": "Hello, world!"
				// }
			case slack.MentionEvent:
				if event.ThreadID != "" {
					reply, err := backend.Prompt(ctx, event.ThreadID, createMentionPrompt(event.Text, event.ChannelID, event.ThreadID, event.UserID))
					if err != nil {
						log.Error().Err(err).Msg("Failed to prompt assistant")
						slackHandler.PostEphemeral(event.ChannelID, event.UserID, err.Error())
						continue
					}

					if err := slackHandler.PostMessage(event.ChannelID, &event.ThreadID, reply); err != nil {
						log.Error().Err(err).Msg("Failed to post message")
						continue
					}
				} else {
					// Work with the global thread
					reply, err := backend.Prompt(ctx, "global", event.Text)
					if err != nil {
						log.Error().Err(err).Msg("Failed to prompt assistant")
						slackHandler.PostEphemeral(event.ChannelID, event.UserID, err.Error())
						continue
					}

					if err := slackHandler.PostMessage(event.ChannelID, &event.ThreadID, reply); err != nil {
						log.Error().Err(err).Msg("Failed to post message")
						continue
					}
				}
			}
		}
	}

}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("Failed to get user home directory")
	}

	return filepath.Join(home, ".local", "share", "scholar")
}
