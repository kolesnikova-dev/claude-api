package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

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

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Chat with Claude. Type 'exit' or press Ctrl+D to quit.")

	for {
		fmt.Print("you> ")
		if !scanner.Scan() {
			return scanner.Err() // nil on EOF (Ctrl+D / Ctrl+Z on Windows)
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" {
			return nil
		}

		reply, err := svc.Chat(ctx, line)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			fmt.Fprintln(os.Stderr, "error:", err)
			continue // history is rolled back on failure, so you can retry
		}
		fmt.Println("claude>", reply)
	}
}
