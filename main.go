package main

import (
	"log"
	"os"
)

func main() {
	accessToken := os.Getenv("MATRIX_ACCESS_TOKEN")
	databaseURI := os.Getenv("POSTGRES_URI")
	homeserver := os.Getenv("MATRIX_HOMESERVER")
	openaiApiKey := os.Getenv("OPENAI_API_KEY")
	user := os.Getenv("MATRIX_USER")

	if accessToken == "" {
		log.Println("You must provide a value for MATRIX_ACCESS_TOKEN")
		os.Exit(1)
	}
	if databaseURI == "" {
		log.Println("You must provide a value for POSTGRES_URI")
		os.Exit(1)
	}
	if homeserver == "" {
		log.Println("You must provide a value for MATRIX_HOMESERVER")
		os.Exit(1)
	}
	if openaiApiKey == "" {
		log.Println("You must provide a value for OPENAI_API_KEY")
		os.Exit(1)
	}
	if user == "" {
		log.Println("You must provide a value for MATRIX_USER")
		os.Exit(1)
	}

	config := BotConfiguration{
		MatrixAccessToken: accessToken,
		MatrixHomeserver:  homeserver,
		MatrixUser:        user,
		OpenAIAPIKey:      openaiApiKey,
		PostgresURI:       databaseURI,
	}
	bot, err := NewBot(config)
	if err != nil {
		log.Printf("Failed to create bot: %v", err)
		os.Exit(1)
	}
	bot.Run()
}
