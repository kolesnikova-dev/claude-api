package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/joho/godotenv"
)

func init() {
	// Look one level up for the .env file
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("Warning: No .env file found in the parent directory or error loading it. Relying on system environment variables.")
	}
}

func main() {
	client := anthropic.NewClient()

	msg, err := client.Messages.New(
		context.TODO(),
		anthropic.MessageNewParams{
			Model:     anthropic.ModelClaudeSonnet5_5,
			MaxTokens: 1000,
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(
					anthropic.NewTextBlock("Hello, Claude"),
				),
			},
		},
	)
	// Graceful error handling
	if err != nil {
		// 1. Print a clean, user-friendly message to standard error
		fmt.Fprintln(os.Stderr, "Oops! We encountered an issue while communicating with Claude.")

		// 2. Log the raw error details for technical troubleshooting/logging systems
		fmt.Fprintf(os.Stderr, "Details: %v\n", err)

		// 3. Exit
		os.Exit(1)
	}

	for _, block := range msg.Content {
		if textBlock, ok := block.AsAny().(anthropic.TextBlock); ok {
			fmt.Println(textBlock.Text)
		}
	}
}
