package evaluator

import "context"

// TestCase is one generated evaluation scenario.
type TestCase struct {
	PromptInputs     map[string]string `json:"prompt_inputs"`
	SolutionCriteria []string          `json:"solution_criteria"`
	TaskDescription  string            `json:"task_description"`
	Scenario         string            `json:"scenario"`
}

// Result is the graded outcome of running the prompt on a TestCase.
type Result struct {
	Output    string   `json:"output"`
	TestCase  TestCase `json:"test_case"`
	Score     float64  `json:"score"`
	Reasoning string   `json:"reasoning"`
}

// RunFunc runs the prompt under evaluation on inputs and returns the raw model
// output. It is called once per test case, possibly concurrently.
type RunFunc func(ctx context.Context, inputs map[string]string) (string, error)
