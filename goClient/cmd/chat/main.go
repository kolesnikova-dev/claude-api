package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/joho/godotenv"

	"go-api-claude/internal/chat"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Encountered an error:", err)
		os.Exit(1)
	}
}

const (
	orange = "\033[38;5;208m" // ANSI 256-color orange
	reset  = "\033[0m"
)

func printBanner() {
	fmt.Println(orange + `    \ | /` + reset)
	fmt.Println(orange + `   -- * --` + reset)
	fmt.Println(orange + `    / | \` + reset)
	fmt.Println(orange + `  Claude says hi` + reset)
	fmt.Println(orange + "Type 'exit' or press Ctrl+Z to quit." + reset)
}

// logRequest prints each outgoing request's method, URL and JSON body.
// Headers are skipped so the API key is never printed.
func logRequest(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("log request: read body: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
		fmt.Fprintf(os.Stderr, "--> %s %s\n%s\n", req.Method, req.URL, body)
	}
	return next(req)
}

func run() error {
	// Best-effort: fall back to the process environment.
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("no .env loaded, relying on system environment variables")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	system := "You are patient and understanding of a developer trying to build with you. The dev configured a continuous message loop."
	// temp := 0.9
	// client := anthropic.NewClient(option.WithMiddleware(logRequest))
	client := anthropic.NewClient()
	svc := chat.New(&client.Messages, chat.WithMaxTokens(1000), chat.WithSystemPrompt(system))

	scanner := bufio.NewScanner(os.Stdin)
	printBanner()

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
		fmt.Println("Booping...")

		reply, err := svc.Chat(ctx, line)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			fmt.Fprintln(os.Stderr, "error:", err)
			continue // history is rolled back on failure, so you can retry
		}
		fmt.Println("claude> ", reply)

	}
}
