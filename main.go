package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	"github.com/mempirate/scholar/backend"
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

	// 1. Download all remote files into the local storage (RemoteStore -> LocalStore)

	backend := backend.NewBackend(key, OPENAI_MODEL, fileStore)

	ctx := context.Background()

	if err := backend.Init(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize backend")
	}

	log.Info().Msg("Backend initialized")

	slack := slack.NewSlackHandler(appToken, botToken, backend)

	slack.Start()
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("Failed to get user home directory")
	}

	return filepath.Join(home, ".local", "share", "scholar")
}
