package main

import (
	"context"
	"os"

	"github.com/mempirate/scholar/backend"
	"github.com/mempirate/scholar/log"
	"github.com/mempirate/scholar/slack"
	"github.com/openai/openai-go"
)

const OPENAI_MODEL = openai.ChatModelGPT4oMini

func main() {
	key, appToken, botToken := os.Getenv("OPENAI_API_KEY"), os.Getenv("SLACK_APP_TOKEN"), os.Getenv("SLACK_BOT_TOKEN")
	if key == "" || appToken == "" || botToken == "" {
		panic("OPENAI_API_KEY || SLACK_APP_TOKEN || SLACK_BOT_TOKEN is not set")
	}

	backend := backend.NewBackend(key, OPENAI_MODEL)
	log := log.NewLogger("main")

	ctx := context.Background()

	if err := backend.Init(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize backend")
	}

	log.Info().Msg("Backend initialized")

	slack := slack.NewSlackHandler(appToken, botToken, backend)

	slack.Start()
}
