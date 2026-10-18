package evaluator

import (
	"strings"
	"testing"
)

func TestScoreClass(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{10, "score-high"}, {8, "score-high"},
		{7.5, "score-medium"}, {7, "score-medium"}, {6, "score-medium"},
		{5, "score-low"}, {1, "score-low"},
	}
	for _, tt := range tests {
		if got := scoreClass(tt.score); got != tt.want {
			t.Errorf("scoreClass(%v) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestGenerateReport(t *testing.T) {
	results := []Result{
		{
			Output: "<b>raw</b>",
			TestCase: TestCase{
				Scenario:         "scenario one",
				PromptInputs:     map[string]string{"b": "2", "a": "1"},
				SolutionCriteria: []string{"c1", "c2"},
			},
			Score:     7,
			Reasoning: "fine",
		},
		{TestCase: TestCase{Scenario: "two"}, Score: 4, Reasoning: "weak"},
	}
	html := GenerateReport(results)

	for _, want := range []string{
		`<div class="stat-value">2</div>`,
		`<div class="stat-value">5.5 / 10</div>`,
		"<div>Pass Rate (≥7)</div>\n                    <div class=\"stat-value\">50.0%</div>",
		`<td>scenario one</td>`,
		`<strong>a:</strong> 1<br><strong>b:</strong> 2`,
		`• c1<br>• c2`,
		`<pre><b>raw</b></pre>`, // unescaped, as in the notebook
		`<span class="score score-medium">7</span>`,
		`<span class="score score-low">4</span>`,
		`<td class="reasoning">weak</td>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("report missing %q", want)
		}
	}
	if !strings.HasSuffix(html, "</html>\n    ") {
		t.Errorf("report tail = %q", html[len(html)-20:])
	}
}

func TestGenerateReportEmpty(t *testing.T) {
	html := GenerateReport(nil)
	if !strings.Contains(html, `<div class="stat-value">0.0 / 10</div>`) || !strings.Contains(html, `<div class="stat-value">0.0%</div>`) {
		t.Error("empty report stats wrong")
	}
}
