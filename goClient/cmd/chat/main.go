package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/joho/godotenv"

	"go-api-claude/internal/chat"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Encountered an error:", err)
		os.Exit(1)
	}
}

func run() error {
	// Best-effort: fall back to the process environment.
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("no .env loaded, relying on system environment variables")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := anthropic.NewClient()
	svc := chat.New(&client.Messages, chat.WithMaxTokens(1000))

	reply, err := svc.Chat(ctx, "Hello, Claude")
	if err != nil {
		return err
	}
	fmt.Println(reply)
	return nil
}
