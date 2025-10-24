package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/util/exzerolog"
)

type Logger = zerolog.Logger

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
	log               zerolog.Logger
	matrixClient      *MatrixClient
}

func NewBot(config BotConfiguration) (*Bot, error) {
	log := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		//w.Out = io.Writer(buf)
		w.TimeFormat = time.Stamp
	})).With().Timestamp().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	exzerolog.SetupDefaults(&log)

	aiRequestChannel := make(chan AIRequest)
	aiResponseChannel := make(chan AIResponse)

	matrixClient, err := NewMatrixClient(log, config.MatrixHomeserver, config.MatrixUser, config.MatrixAccessToken, aiRequestChannel, aiResponseChannel)
	if err != nil {
		return nil, fmt.Errorf("Failed to create matrix client: %v", err)
	}

	openAIClient, err := NewOpenAIClient(config.OpenAIAPIKey, aiRequestChannel, aiResponseChannel)
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
	log.Printf("Received signal: %v\n", sig)

	log.Println("Canceling workers")
	cancel()
	log.Println("Waiting for workers to finish...")
	waitGroup.Wait()
	log.Println("Workers complete, exiting.")
}
