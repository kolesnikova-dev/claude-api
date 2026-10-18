package evaluator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"golang.org/x/sync/errgroup"
)

// Default file names, as in the original notebook.
const (
	DefaultDatasetFile = "dataset.json"
	DefaultJSONOutput  = "output.json"
	DefaultHTMLOutput  = "output.html"
)

const (
	defaultConcurrency = 3
	progressStep       = 20 // percent between progress lines
)

// Temperatures for each kind of model call.
const (
	ideaTemperature  = 1.0
	caseTemperature  = 0.7
	gradeTemperature = 0.0
)

// ErrNoScore is returned when the grader's JSON has no "score" field.
var ErrNoScore = errors.New("evaluator: grade has no score")

// Evaluator generates datasets and grades prompt outputs using a Completer.
type Evaluator struct {
	c             Completer
	maxConcurrent int
	out           io.Writer
}

// Option configures an Evaluator.
type Option func(*Evaluator)

// WithMaxConcurrency sets how many model calls run at once. Values <= 0 are
// ignored. Raise it for speed, but beware of rate limits.
func WithMaxConcurrency(n int) Option {
	return func(e *Evaluator) {
		if n > 0 {
			e.maxConcurrent = n
		}
	}
}

// WithOutput sets where progress lines are written. The default is os.Stdout.
func WithOutput(w io.Writer) Option {
	return func(e *Evaluator) {
		if w != nil {
			e.out = w
		}
	}
}

// New returns an Evaluator that calls c.
func New(c Completer, opts ...Option) *Evaluator {
	e := &Evaluator{c: c, maxConcurrent: defaultConcurrency, out: os.Stdout}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// GenerateUniqueIdeas asks the model for numCases distinct test scenarios.
func (e *Evaluator) GenerateUniqueIdeas(ctx context.Context, task string, numCases int) ([]string, error) {
	text, err := e.c.Complete(ctx, Request{
		System:        ideasSystemPrompt,
		Prompt:        ideasPrompt(task, numCases),
		Prefill:       jsonPrefill,
		Temperature:   ideaTemperature,
		StopSequences: []string{fence},
	})
	if err != nil {
		return nil, err
	}
	var ideas []string
	if err := json.Unmarshal([]byte(text), &ideas); err != nil {
		return nil, fmt.Errorf("evaluator: parse ideas: %w", err)
	}
	return ideas, nil
}

// GenerateTestCase expands a single idea into a full test case.
func (e *Evaluator) GenerateTestCase(ctx context.Context, task, idea string, spec map[string]string) (TestCase, error) {
	text, err := e.c.Complete(ctx, Request{
		System:        testCaseSystemPrompt,
		Prompt:        testCasePrompt(task, idea, spec),
		Prefill:       jsonPrefill,
		Temperature:   caseTemperature,
		StopSequences: []string{fence},
	})
	if err != nil {
		return TestCase{}, err
	}
	var tc TestCase
	if err := json.Unmarshal([]byte(text), &tc); err != nil {
		return TestCase{}, fmt.Errorf("evaluator: parse test case: %w", err)
	}
	tc.TaskDescription = task
	tc.Scenario = idea
	return tc, nil
}

// GenerateDataset creates test cases for task and saves them to outputFile.
// spec maps each prompt input name to a description of it. Cases that fail to
// generate are reported and skipped, so fewer than numCases may be returned.
func (e *Evaluator) GenerateDataset(ctx context.Context, task string, spec map[string]string, numCases int, outputFile string) ([]TestCase, error) {
	ideas, err := e.GenerateUniqueIdeas(ctx, task, numCases)
	if err != nil {
		return nil, err
	}

	dataset := make([]TestCase, 0, len(ideas))
	var mu sync.Mutex
	prog := newProgress(e.out, "Generated", "test cases", len(ideas))

	var g errgroup.Group
	g.SetLimit(e.maxConcurrent)
	for _, idea := range ideas {
		g.Go(func() error {
			tc, err := e.GenerateTestCase(ctx, task, idea, spec)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fmt.Fprintf(e.out, "Error generating test case: %v\n", err)
				return nil
			}
			prog.step()
			dataset = append(dataset, tc)
			return nil
		})
	}
	_ = g.Wait() // workers never return an error
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := writeJSON(outputFile, dataset); err != nil {
		return nil, err
	}
	return dataset, nil
}

type grade struct {
	Reasoning string   `json:"reasoning"`
	Score     *float64 `json:"score"`
}

// GradeOutput has the model score output against tc's criteria, plus any
// mandatory extraCriteria (violating one caps the score at 3).
func (e *Evaluator) GradeOutput(ctx context.Context, tc TestCase, output, extraCriteria string) (score float64, reasoning string, err error) {
	text, err := e.c.Complete(ctx, Request{
		Prompt:        gradePrompt(tc, output, extraCriteria),
		Prefill:       jsonPrefill,
		Temperature:   gradeTemperature,
		StopSequences: []string{fence},
	})
	if err != nil {
		return 0, "", err
	}
	var g grade
	if err := json.Unmarshal([]byte(text), &g); err != nil {
		return 0, "", fmt.Errorf("evaluator: parse grade: %w", err)
	}
	if g.Score == nil {
		return 0, "", ErrNoScore
	}
	return *g.Score, g.Reasoning, nil
}

// RunTestCase runs the prompt on tc and grades its output.
func (e *Evaluator) RunTestCase(ctx context.Context, tc TestCase, run RunFunc, extraCriteria string) (Result, error) {
	output, err := run(ctx, tc.PromptInputs)
	if err != nil {
		return Result{}, fmt.Errorf("evaluator: run prompt: %w", err)
	}
	score, reasoning, err := e.GradeOutput(ctx, tc, output, extraCriteria)
	if err != nil {
		return Result{}, err
	}
	return Result{Output: output, TestCase: tc, Score: score, Reasoning: reasoning}, nil
}

// RunEvaluation runs and grades every test case in datasetFile, then writes the
// results to jsonOutput and an HTML report to htmlOutput. The first error
// aborts the evaluation. Results are in completion order.
func (e *Evaluator) RunEvaluation(ctx context.Context, run RunFunc, datasetFile, extraCriteria, jsonOutput, htmlOutput string) ([]Result, error) {
	raw, err := os.ReadFile(datasetFile)
	if err != nil {
		return nil, fmt.Errorf("evaluator: read dataset: %w", err)
	}
	var dataset []TestCase
	if err := json.Unmarshal(raw, &dataset); err != nil {
		return nil, fmt.Errorf("evaluator: parse dataset: %w", err)
	}

	results := make([]Result, 0, len(dataset))
	var mu sync.Mutex
	prog := newProgress(e.out, "Graded", "test cases", len(dataset))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(e.maxConcurrent)
	for _, tc := range dataset {
		if gctx.Err() != nil {
			break
		}
		g.Go(func() error {
			if gctx.Err() != nil { // another case failed while this one waited for a slot
				return nil
			}
			res, err := e.RunTestCase(gctx, tc, run, extraCriteria)
			if err != nil {
				return err
			}
			mu.Lock()
			defer mu.Unlock()
			prog.step()
			results = append(results, res)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	if len(results) > 0 {
		var sum float64
		for _, r := range results {
			sum += r.Score
		}
		fmt.Fprintf(e.out, "Average score: %g\n", sum/float64(len(results)))
	}

	if err := writeJSON(jsonOutput, results); err != nil {
		return nil, err
	}
	if err := os.WriteFile(htmlOutput, []byte(GenerateReport(results)), 0o644); err != nil {
		return nil, fmt.Errorf("evaluator: write report: %w", err)
	}
	return results, nil
}

// progress prints a line each time completion crosses another 20% milestone.
// It is not safe for concurrent use; callers hold their own lock.
type progress struct {
	out         io.Writer
	verb, noun  string
	total, done int
	last        int
}

func newProgress(out io.Writer, verb, noun string, total int) *progress {
	return &progress{out: out, verb: verb, noun: noun, total: total}
}

func (p *progress) step() {
	p.done++
	milestone := p.done * 100 / p.total / progressStep * progressStep
	if milestone > p.last {
		fmt.Fprintf(p.out, "%s %d/%d %s\n", p.verb, p.done, p.total, p.noun)
		p.last = milestone
	}
}

// writeJSON writes v to path, indented by two spaces.
func writeJSON(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("evaluator: encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, bytes.TrimRight(buf.Bytes(), "\n"), 0o644); err != nil {
		return fmt.Errorf("evaluator: write %s: %w", path, err)
	}
	return nil
}
