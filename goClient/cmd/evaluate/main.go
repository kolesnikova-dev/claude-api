package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/joho/godotenv"

	"go-api-claude/internal/evaluator"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	_ = godotenv.Load("../.env") // best-effort, as in cmd/chat

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := anthropic.NewClient()                            // reads ANTHROPIC_API_KEY
	c := evaluator.NewAnthropicCompleter(&client.Messages, "") // "" = Haiku 4.5

	// Notebook cell 4. Keep concurrency low if you hit rate limits.
	ev := evaluator.New(c, evaluator.WithMaxConcurrency(1))

	// Cell 5: generate the dataset (a call to the model, so it costs tokens).
	_, err := ev.GenerateDataset(ctx,
		"Summarize a news article in one sentence",            // what the prompt should do
		map[string]string{"article": "The full article text"}, // input name -> description
		3,                            // number of test cases
		evaluator.DefaultDatasetFile, // dataset.json
	)
	if err != nil {
		return err
	}

	// Cell 6: the prompt under test. Called once per test case.
	runPrompt := func(ctx context.Context, in map[string]string) (string, error) {
		return c.Complete(ctx, evaluator.Request{
			Prompt:      "Summarize this in one sentence:\n\n" + in["article"],
			Temperature: 1.0, // the notebook's chat() default
		})
	}

	// Cell 7: grade it. extraCriteria is optional ("" for none).
	results, err := ev.RunEvaluation(ctx, runPrompt,
		evaluator.DefaultDatasetFile, "",
		evaluator.DefaultJSONOutput, evaluator.DefaultHTMLOutput,
	)
	if err != nil {
		return err
	}
	fmt.Println("cases graded:", len(results))
	return nil
}
