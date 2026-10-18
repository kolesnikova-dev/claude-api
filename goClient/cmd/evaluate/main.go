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

	client := anthropic.NewClient()                         // reads ANTHROPIC_API_KEY
	c := evaluator.NewAnthropicGrader(&client.Messages, "") // "" = Haiku 4.5

	// Notebook cell 4. Keep concurrency low if you hit rate limits.
	ev := evaluator.New(c, evaluator.WithMaxConcurrency(1))

	// mealPlanTask := "Write a compact, concise 1 day meal plan for a single athlete"

	// // Cell 5: generate the dataset (a call to the model, so it costs tokens).
	// _, err := ev.GenerateDataset(ctx,
	// 	mealPlanTask, // what the prompt should do
	// 	map[string]string{"height": "Athlete's height in cm", // prompt input specs
	// 		"weight":       "Athlete's weight in kg",
	// 		"goal":         "Athlete's goal",
	// 		"restrictions": "Athlete's dietary restrictions"},
	// 	3,                            // number of test cases
	// 	evaluator.DefaultDatasetFile, // dataset.json
	// )
	// if err != nil {
	// 	return err
	// }

	// fmt.Println("generated", len(cases), "cases")
	// return nil
	// Cell 6: the prompt under test. Called once per test case.
	runPrompt := func(ctx context.Context, in map[string]string) (string, error) {
		return c.Grade(ctx, evaluator.Request{
			Prompt: "What should this person eat? :\n\n" +
				"<athlete_information>" +
				"- Height " + in["height"] + "\n" +
				"- Weight " + in["weight"] + "\n" +
				"- Goal " + in["goal"] + "\n" +
				"- Restrictions " + in["restrictions"] +
				"<athlete_information/>" +
				"<guidelines>" +
				"1. Include accurate daily calorie amount, " +
				"2. Show protein, fat, and carbohydrate amounts, " +
				"3. Specify when to eat each meal, " +
				"4. Use only foods that fit restrictions, " +
				"5. List all portion sizes in grams, " +
				"6. Keep budget-friendly if mentioned." +
				"<guidelines/>",
			Temperature: 1.0, // the notebook's chat() default
		})
	}

	// Cell 7: grade it. extraCriteria is optional ("" for none).
	extraCriteria := "The output should include: 1) Daily caloric total, 2) Macronutrient breakdown, 3) Meals with exact foods, portions and timing."
	results, err := ev.RunEvaluation(ctx, runPrompt,
		evaluator.DefaultDatasetFile, extraCriteria,
		evaluator.DefaultJSONOutput, evaluator.DefaultHTMLOutput,
	)
	if err != nil {
		return err
	}
	fmt.Println("cases graded:", len(results))
	return nil
}
