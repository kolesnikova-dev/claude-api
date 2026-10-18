package evaluator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// fakeGrader answers by calling fn and records every request.
type fakeGrader struct {
	mu   sync.Mutex
	reqs []Request
	fn   func(Request) (string, error)
}

func (f *fakeGrader) Grade(_ context.Context, req Request) (string, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()
	return f.fn(req)
}

func TestRender(t *testing.T) {
	tests := []struct {
		name, tmpl string
		vars       map[string]string
		want       string
	}{
		{"substitutes", "hi {name}", map[string]string{"name": "bob"}, "hi bob"},
		{"unknown placeholder kept", "hi {other}", map[string]string{"name": "bob"}, "hi {other}"},
		{"unescapes braces", "{{ {x} }}", map[string]string{"x": "1"}, "{ 1 }"},
		{"unescapes substituted values", "{x}", map[string]string{"x": "{{y}}"}, "{y}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := render(tt.tmpl, tt.vars); got != tt.want {
				t.Errorf("render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrompts(t *testing.T) {
	spec := map[string]string{"b": "second\nline", "a": "first"}

	got := testCasePrompt("task", "idea", spec)
	for _, want := range []string{
		"\"a\": \"EXAMPLE_VALUE\", // first\n\"b\": \"EXAMPLE_VALUE\", // second\\nline\n",
		`"a", "b"`,
		"<specific_idea>\nidea\n</specific_idea>",
		"```json\n{\n    \"prompt_inputs\": {\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("test case prompt missing %q", want)
		}
	}
	if strings.Contains(got, "{{") || strings.Contains(got, "~~~") {
		t.Error("test case prompt has unrendered escapes")
	}

	ideas := ideasPrompt("task", 3)
	if !strings.Contains(ideas, "Generate 3 unique") || !strings.Contains(ideas, "{prompt_inputs_spec}") {
		t.Errorf("unexpected ideas prompt:\n%s", ideas)
	}
}

func TestGradePrompt(t *testing.T) {
	tc := TestCase{
		PromptInputs:     map[string]string{"text": "a\nb"},
		SolutionCriteria: []string{"one", "two"},
		TaskDescription:  "task",
	}
	got := gradePrompt(tc, "the output", "")
	for _, want := range []string{
		"<task_inputs>\n{ \"text\":\"a\\nb\",\n }\n</task_inputs>",
		"<solution>\nthe output\n</solution>",
		"<criteria>\none\ntwo\n</criteria>",
		"\"score\": number\n}",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("grade prompt missing %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "Mandatory Requirements") {
		t.Error("extra criteria section present without extra criteria")
	}
	if got := gradePrompt(tc, "o", "be nice"); !strings.Contains(got, "<extra_important_criteria>\nbe nice\n</extra_important_criteria>") {
		t.Error("extra criteria section missing")
	}
}

func TestGenerateUniqueIdeas(t *testing.T) {
	f := &fakeGrader{fn: func(Request) (string, error) { return `["a", "b"]`, nil }}
	got, err := New(f).GenerateUniqueIdeas(context.Background(), "task", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("ideas = %v", got)
	}
	r := f.reqs[0]
	if r.Prefill != "```json" || r.Temperature != 1.0 || len(r.StopSequences) != 1 || r.StopSequences[0] != "```" || r.System != ideasSystemPrompt {
		t.Errorf("unexpected request %+v", r)
	}
}

func TestGenerateTestCase(t *testing.T) {
	f := &fakeGrader{fn: func(Request) (string, error) {
		return `{"prompt_inputs": {"x": "1"}, "solution_criteria": ["c"]}`, nil
	}}
	tc, err := New(f).GenerateTestCase(context.Background(), "task", "idea", map[string]string{"x": "desc"})
	if err != nil {
		t.Fatal(err)
	}
	if tc.TaskDescription != "task" || tc.Scenario != "idea" || tc.PromptInputs["x"] != "1" || tc.SolutionCriteria[0] != "c" {
		t.Errorf("tc = %+v", tc)
	}
	if f.reqs[0].Temperature != 0.7 {
		t.Errorf("temperature = %v", f.reqs[0].Temperature)
	}
}

func TestGenerateDatasetSkipsFailures(t *testing.T) {
	f := &fakeGrader{fn: func(r Request) (string, error) {
		switch {
		case r.System == ideasSystemPrompt:
			return `["good", "bad", "good2"]`, nil
		case strings.Contains(r.Prompt, "<specific_idea>\nbad\n"):
			return "", errors.New("boom")
		default:
			return `{"prompt_inputs": {"x": "1"}, "solution_criteria": ["c"]}`, nil
		}
	}}
	var out bytes.Buffer
	file := filepath.Join(t.TempDir(), "dataset.json")
	ev := New(f, WithMaxConcurrency(1), WithOutput(&out))

	got, err := ev.GenerateDataset(context.Background(), "task", map[string]string{"x": "d"}, 3, file)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d cases, want 2", len(got))
	}
	if !strings.Contains(out.String(), "Error generating test case: boom") {
		t.Errorf("output = %q", out.String())
	}

	var saved []TestCase
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &saved); err != nil || len(saved) != 2 {
		t.Errorf("saved dataset: %v, %v", saved, err)
	}
	if !strings.Contains(string(raw), "\n  {\n    \"prompt_inputs\"") {
		t.Errorf("dataset not indented by two spaces:\n%s", raw)
	}
}

func gradeJSON(score int) string {
	return `{"strengths": ["s"], "weaknesses": ["w"], "reasoning": "because", "score": ` + string(rune('0'+score)) + `}`
}

func writeDataset(t *testing.T, n int) string {
	t.Helper()
	cases := make([]TestCase, n)
	for i := range cases {
		cases[i] = TestCase{
			PromptInputs:     map[string]string{"x": string(rune('a' + i))},
			SolutionCriteria: []string{"c"},
			TaskDescription:  "task",
			Scenario:         "s",
		}
	}
	file := filepath.Join(t.TempDir(), "dataset.json")
	if err := writeJSON(file, cases); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestRunEvaluation(t *testing.T) {
	f := &fakeGrader{fn: func(Request) (string, error) { return gradeJSON(8), nil }}
	var out bytes.Buffer
	ev := New(f, WithOutput(&out))
	dir := t.TempDir()
	jsonOut, htmlOut := filepath.Join(dir, "o.json"), filepath.Join(dir, "o.html")

	run := func(_ context.Context, in map[string]string) (string, error) { return "out-" + in["x"], nil }
	results, err := ev.RunEvaluation(context.Background(), run, writeDataset(t, 5), "", jsonOut, htmlOut)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Fatalf("got %d results", len(results))
	}
	for _, r := range results {
		if r.Score != 8 || r.Reasoning != "because" || r.Output != "out-"+r.TestCase.PromptInputs["x"] {
			t.Errorf("bad result %+v", r)
		}
	}
	for _, r := range f.reqs {
		if r.Temperature != 0 || r.System != "" || r.Prefill != "```json" {
			t.Errorf("unexpected grader request %+v", r)
		}
	}
	if !strings.Contains(out.String(), "Graded 1/5 test cases") || !strings.Contains(out.String(), "Average score: 8") {
		t.Errorf("output = %q", out.String())
	}

	var saved []Result
	raw, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &saved); err != nil || len(saved) != 5 {
		t.Errorf("saved results: %v, %v", len(saved), err)
	}
	html, err := os.ReadFile(htmlOut)
	if err != nil || !strings.Contains(string(html), "Prompt Evaluation Report") {
		t.Errorf("html report: %v", err)
	}
}

func TestRunEvaluationAbortsOnError(t *testing.T) {
	f := &fakeGrader{fn: func(Request) (string, error) { return gradeJSON(8), nil }}
	ev := New(f, WithMaxConcurrency(1), WithOutput(&bytes.Buffer{}))
	boom := errors.New("boom")
	calls := 0
	run := func(context.Context, map[string]string) (string, error) {
		calls++
		return "", boom
	}
	dir := t.TempDir()
	_, err := ev.RunEvaluation(context.Background(), run, writeDataset(t, 4), "", filepath.Join(dir, "o.json"), filepath.Join(dir, "o.html"))
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if calls != 1 {
		t.Errorf("run called %d times, want 1", calls)
	}
	if _, err := os.Stat(filepath.Join(dir, "o.json")); !os.IsNotExist(err) {
		t.Error("results written despite error")
	}
}

func TestGradeOutputMissingScore(t *testing.T) {
	f := &fakeGrader{fn: func(Request) (string, error) { return `{"reasoning": "x"}`, nil }}
	_, _, err := New(f).GradeOutput(context.Background(), TestCase{}, "o", "")
	if !errors.Is(err, ErrNoScore) {
		t.Errorf("err = %v", err)
	}
}

func TestRunEvaluationEmptyDataset(t *testing.T) {
	ev := New(&fakeGrader{fn: func(Request) (string, error) { return "", nil }}, WithOutput(&bytes.Buffer{}))
	dir := t.TempDir()
	file := filepath.Join(dir, "d.json")
	if err := os.WriteFile(file, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := ev.RunEvaluation(context.Background(), nil, file, "", filepath.Join(dir, "o.json"), filepath.Join(dir, "o.html"))
	if err != nil || len(results) != 0 {
		t.Fatalf("results = %v, err = %v", results, err)
	}
}

func TestProgressMilestones(t *testing.T) {
	var out bytes.Buffer
	p := newProgress(&out, "Graded", "test cases", 10)
	for range 10 {
		p.step()
	}
	want := "Graded 2/10 test cases\nGraded 4/10 test cases\nGraded 6/10 test cases\nGraded 8/10 test cases\nGraded 10/10 test cases\n"
	if out.String() != want {
		t.Errorf("progress = %q", out.String())
	}
}

type fakeMessenger struct {
	params anthropic.MessageNewParams
	resp   string
}

func (m *fakeMessenger) New(_ context.Context, p anthropic.MessageNewParams, _ ...option.RequestOption) (*anthropic.Message, error) {
	m.params = p
	var msg anthropic.Message
	if err := msg.UnmarshalJSON([]byte(`{"content":[{"type":"text","text":"` + m.resp + `"}]}`)); err != nil {
		return nil, err
	}
	return &msg, nil
}

func TestAnthropicGrader(t *testing.T) {
	fm := &fakeMessenger{resp: "[]"}
	c := NewAnthropicGrader(fm, "")

	got, err := c.Grade(context.Background(), Request{
		System:        "sys",
		Prompt:        "hello",
		Prefill:       "```json",
		Temperature:   0,
		StopSequences: []string{"```"},
	})
	if err != nil || got != "[]" {
		t.Fatalf("got %q, %v", got, err)
	}

	p := fm.params
	if p.Model != anthropic.ModelClaudeHaiku4_5 || p.MaxTokens != 1000 {
		t.Errorf("model/max tokens = %v/%v", p.Model, p.MaxTokens)
	}
	if !p.Temperature.Valid() || p.Temperature.Value != 0 {
		t.Errorf("temperature 0 must be sent, got %+v", p.Temperature)
	}
	if len(p.Messages) != 2 || p.Messages[0].Role != anthropic.MessageParamRoleUser || p.Messages[1].Role != anthropic.MessageParamRoleAssistant {
		t.Errorf("messages = %+v", p.Messages)
	}
	if len(p.System) != 1 || p.System[0].Text != "sys" || len(p.StopSequences) != 1 {
		t.Errorf("system/stop = %+v / %v", p.System, p.StopSequences)
	}

	if _, err := c.Grade(context.Background(), Request{Prompt: "x"}); err != nil {
		t.Fatal(err)
	}
	if len(fm.params.Messages) != 1 || len(fm.params.System) != 0 {
		t.Errorf("no prefill/system expected: %+v", fm.params)
	}
}
