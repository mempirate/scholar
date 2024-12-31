package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
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

	slackHandler := slack.NewSlackHandler(appToken, botToken)
	commands := slackHandler.SubscribeCommands()

	go slackHandler.Start()

	contentHandler := content.NewContentHandler()

	for {
		select {
		case cmd := <-commands:
			// Handle content extraction
			content, err := contentHandler.HandleURL(cmd.URL)
			if err != nil {
				log.Error().Err(err).Msg("Failed to handle URL")
				// TODO: Send error message to Slack
				// slack.PostEphemeral(channelID string, userID string, text string)
				continue
			}

			// Handle content de-duplication
			contains, err := fileStore.Contains(content.Name)
			if err != nil {
				log.Error().Err(err).Msg("Failed to check if content exists")
				continue
			}

			if contains {
				log.Info().Str("name", content.Name).Msg("File already exists, skipping")
				// TODO: Send error message to Slack
				// slack.PostEphemeral(channelID string, userID string, text string)
				continue
			}

			r := bytes.NewReader(content.Content)
			if err := fileStore.Store(content.Name, r); err != nil {
				log.Error().Err(err).Msg("Failed to store file locally")
				// TODO: Send error message to Slack
				// slack.PostEphemeral(channelID string, userID string, text string)
				continue
			}

			file, err := fileStore.Get(content.Name)

			if err := backend.UploadFile(ctx, content.Name, file); err != nil {
				log.Error().Err(err).Msg("Failed to upload file")
				// TODO: Send error message to Slack
				// slack.PostEphemeral(channelID string, userID string, text string)
				continue
			}

			file.Close()

			threadID, err := slackHandler.StartUploadThread(cmd.ChannelID, cmd.UserID, fmt.Sprintf("%s [%s]", content.Name, content.URL))
			if err := backend.CreateThread(ctx, threadID); err != nil {
				log.Error().Err(err).Msg("Failed to create thread")
				continue
			}

			if cmd.CommandType == slack.SummarizeCommand {
				summary, err := backend.Prompt(ctx, threadID, fmt.Sprintf("Please provide a summary of this file: %s. In case of a PDF, always mention the title of the paper instead of the name. Only use a single reference per unique file.", content.Name))
				if err != nil {
					log.Error().Err(err).Msg("Failed to prompt for summary")
					continue
				}

				if err := slackHandler.PostMessage(cmd.ChannelID, &threadID, summary); err != nil {
					log.Error().Err(err).Msg("Failed to post summary")
					continue
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
