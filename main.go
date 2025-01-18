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
	"github.com/mempirate/scholar/prompt"
	"github.com/mempirate/scholar/scrape"
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

	key, appToken, botToken, fcKey := os.Getenv("OPENAI_API_KEY"), os.Getenv("SLACK_APP_TOKEN"), os.Getenv("SLACK_BOT_TOKEN"), os.Getenv("FIRECRAWL_API_KEY")
	if key == "" || appToken == "" || botToken == "" || fcKey == "" {
		panic("OPENAI_API_KEY || SLACK_APP_TOKEN || SLACK_BOT_TOKEN | FIRECRAWL_API_KEY is not set")
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
	events := slackHandler.SubscribeEvents()

	go slackHandler.Start()

	fc, err := scrape.NewFirecrawlScraper(fcKey)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Firecrawl scraper")
	}

	contentHandler := content.NewContentHandler(fc)

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

			if content == nil {
				log.Info().Str("url", cmd.URL.String()).Msg("Nil content")
				slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, "Content type not supported")
				continue
			}

			// We only de-duplicate uploads. If someone wants to summarize a file that already exists, we'll allow it, but we won't upload it again.
			if cmd.CommandType == slack.UploadCommand {
				// Handle content de-duplication
				contains, err := fileStore.Contains(content.FileName())
				if err != nil {
					log.Error().Err(err).Msg("Failed to check if content exists")
					continue
				}

				if contains {
					log.Info().Str("name", content.FileName()).Msg("File already exists, skipping")
					slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, "This file already exists.")
					continue
				}
			}

			fileName, file, err := content.ToMarkdown()
			if err != nil {
				log.Error().Err(err).Msg("Failed to convert content to markdown")
				slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, fmt.Sprintf("Failed to convert content to markdown: %s", err))
				continue
			}

			local := bytes.NewReader(file)
			remote := bytes.NewReader(file)
			if err := fileStore.Store(fileName, local); err != nil {
				log.Error().Err(err).Msg("Failed to store file locally")
				slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, err.Error())
				continue
			}

			if err := backend.UploadFile(ctx, fileName, remote); err != nil {
				log.Error().Err(err).Msg("Failed to upload file")
				slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, err.Error())
				continue
			}

			threadID, err := slackHandler.StartUploadThread(cmd.ChannelID, cmd.UserID, fmt.Sprintf("%s [%s]", content.FindTitle(), content.Metadata.Source))
			if err != nil {
				log.Error().Err(err).Msg("Failed to start upload thread")
				slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, fmt.Sprintf("Failed to start upload thread: %s", err))
				continue
			}

			if err := backend.CreateThread(ctx, threadID); err != nil {
				log.Error().Err(err).Msg("Failed to create thread")
				slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, fmt.Sprintf("Failed to create thread: %s", err))
				continue
			}

			if cmd.CommandType == slack.SummarizeCommand {
				summary, err := backend.Prompt(ctx, threadID, prompt.SUMMARY_PROMPT_INSTRUCTIONS, prompt.CreateSummaryPrompt(fileName))
				if err != nil {
					log.Error().Err(err).Msg("Failed to prompt for summary")
					slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, fmt.Sprintf("Failed to prompt for summary: %s", err))
					continue
				}

				if err := slackHandler.PostMessage(cmd.ChannelID, &threadID, summary); err != nil {
					log.Error().Err(err).Msg("Failed to post summary")
					slackHandler.PostEphemeral(cmd.ChannelID, cmd.UserID, fmt.Sprintf("Failed to post summary: %s", err))
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
				//  "messageId": "1635732824.000100", // To refer to messages later
				// 	"channelId": "C01B2PZQX1Z",
				// 	"threadId": "1635732824.000100",
				//  "userId": "U01B2PZQX1Z", // To refer to users. Does this need username as well?
				//  "text": "Hello, world!"
				// }
			case slack.MentionEvent:
				err := backend.CreateThread(ctx, event.ThreadID)
				if err != nil {
					log.Error().Err(err).Msg("Failed to create thread")
					slackHandler.PostEphemeral(event.ChannelID, event.UserID, err.Error())
					continue
				}

				log.Info().Str("user_id", event.UserID).Str("channel_id", event.ChannelID).Str("thread_id", event.ThreadID).Msg("New mention")

				reply, err := backend.Prompt(ctx, event.ThreadID, prompt.MENTION_PROMPT_INSTRUCTIONS, prompt.CreateMentionPrompt(event.Text, event.ChannelID, event.ThreadID, event.UserID))
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

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("Failed to get user home directory")
	}

	return filepath.Join(home, ".local", "share", "scholar")
}
