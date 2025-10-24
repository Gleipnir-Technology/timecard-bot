package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type BotConfiguration struct {
	MatrixAccessToken string
	MatrixHomeserver  string
	MatrixUser        string
	OpenAIAPIKey      string
	PostgresURI       string
}

type Bot struct {
	aiClient          *OpenAIClient
	aiRequestChannel  chan AIRequest
	aiResponseChannel chan AIResponse
	config            BotConfiguration
	log               *Logger
	matrixClient      *MatrixClient
}

func NewBot(config BotConfiguration) (*Bot, error) {
	log := CreateLogger()
	aiRequestChannel := make(chan AIRequest)
	aiResponseChannel := make(chan AIResponse)

	matrixClient, err := NewMatrixClient(log.With().Str("component", "matrix").Logger(), config.MatrixHomeserver, config.MatrixUser, config.MatrixAccessToken, aiRequestChannel, aiResponseChannel)
	if err != nil {
		return nil, fmt.Errorf("Failed to create matrix client: %v", err)
	}

	openAIClient, err := NewOpenAIClient(log.With().Str("component", "openai").Logger(), config.OpenAIAPIKey, aiRequestChannel, aiResponseChannel)
	if err != nil {
		return nil, fmt.Errorf("Failed to create open AI client: %v", err)
	}

	bot := Bot{
		aiClient:     openAIClient,
		config:       config,
		log:          log,
		matrixClient: matrixClient,
	}
	return &bot, nil
}

func (bot *Bot) Run() {
	needsMigrations, err := needsMigrations(bot.config.PostgresURI)
	if err != nil || needsMigrations == nil {
		bot.log.Error().Msg("Failed to check if we need database migrations")
		os.Exit(1)
	}
	if *needsMigrations {
		doMigrations(bot.config.PostgresURI)
		bot.log.Info().Msg("Handled migrations")
	} else {
		bot.log.Info().Msg("No migrations necessary")
	}

	ctx, cancel := context.WithCancel(context.Background())
	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	go bot.matrixClient.Run(ctx, &waitGroup)
	waitGroup.Add(1)
	go bot.aiClient.Run(ctx, &waitGroup)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	bot.log.Info().Str("signal", sig.String()).Msg("Received signal")

	bot.log.Info().Msg("Canceling workers")
	cancel()
	bot.log.Info().Msg("Waiting for workers to finish...")
	waitGroup.Wait()
	bot.log.Info().Msg("Workers complete, exiting.")
}
