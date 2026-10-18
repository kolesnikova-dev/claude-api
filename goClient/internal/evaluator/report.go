package evaluator

import (
	"fmt"
	"strings"
)

const (
	maxScore      = 10
	passThreshold = 7
	highThreshold = 8
	lowThreshold  = 5
)

const reportHead = `
    <!DOCTYPE html>
    <html lang="en">
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <title>Prompt Evaluation Report</title>
        <style>
            body {
                font-family: Arial, sans-serif;
                line-height: 1.6;
                margin: 0;
                padding: 20px;
                color: #333;
            }
            .header {
                background-color: #f0f0f0;
                padding: 20px;
                border-radius: 5px;
                margin-bottom: 20px;
            }
            .summary-stats {
                display: flex;
                justify-content: space-between;
                flex-wrap: wrap;
                gap: 10px;
            }
            .stat-box {
                background-color: #fff;
                border-radius: 5px;
                padding: 15px;
                box-shadow: 0 2px 5px rgba(0,0,0,0.1);
                flex-basis: 30%;
                min-width: 200px;
            }
            .stat-value {
                font-size: 24px;
                font-weight: bold;
                margin-top: 5px;
            }
            table {
                width: 100%;
                border-collapse: collapse;
                margin-top: 20px;
            }
            th {
                background-color: #4a4a4a;
                color: white;
                text-align: left;
                padding: 12px;
            }
            td {
                padding: 10px;
                border-bottom: 1px solid #ddd;
                vertical-align: top;
            }
            tr:nth-child(even) {
                background-color: #f9f9f9;
            }
            .output-cell {
                white-space: pre-wrap;
            }
            .score {
                font-weight: bold;
                padding: 5px 10px;
                border-radius: 3px;
                display: inline-block;
            }
            .score-high {
                background-color: #c8e6c9;
                color: #2e7d32;
            }
            .score-medium {
                background-color: #fff9c4;
                color: #f57f17;
            }
            .score-low {
                background-color: #ffcdd2;
                color: #c62828;
            }
            .output {
                overflow: auto;
                white-space: pre-wrap;
            }

            .output pre {
                background-color: #f5f5f5;
                border: 1px solid #ddd;
                border-radius: 4px;
                padding: 10px;
                margin: 0;
                font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
                font-size: 14px;
                line-height: 1.4;
                color: #333;
                box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.1);
                overflow-x: auto;
                white-space: pre-wrap;
                word-wrap: break-word;
            }

            td {
                width: 20%;
            }
            .score-col {
                width: 80px;
            }
        </style>
    </head>
    <body>
        <div class="header">
            <h1>Prompt Evaluation Report</h1>
            <div class="summary-stats">
`

const reportStats = `                <div class="stat-box">
                    <div>Total Test Cases</div>
                    <div class="stat-value">%d</div>
                </div>
                <div class="stat-box">
                    <div>Average Score</div>
                    <div class="stat-value">%.1f / %d</div>
                </div>
                <div class="stat-box">
                    <div>Pass Rate (≥%d)</div>
                    <div class="stat-value">%.1f%%</div>
                </div>
`

const reportTableHead = `            </div>
        </div>

        <table>
            <thead>
                <tr>
                    <th>Scenario</th>
                    <th>Prompt Inputs</th>
                    <th>Solution Criteria</th>
                    <th>Output</th>
                    <th>Score</th>
                    <th>Reasoning</th>
                </tr>
            </thead>
            <tbody>
    `

const reportRow = `
            <tr>
                <td>%s</td>
                <td class="prompt-inputs">%s</td>
                <td class="criteria">• %s</td>
                <td class="output"><pre>%s</pre></td>
                <td class="score-col"><span class="score %s">%g</span></td>
                <td class="reasoning">%s</td>
            </tr>
        `

const reportTail = `
            </tbody>
        </table>
    </body>
    </html>
    `

// GenerateReport renders results as a standalone HTML page. Values are
// inserted verbatim, without HTML escaping, as in the original notebook.
func GenerateReport(results []Result) string {
	var avg, passRate float64
	if n := len(results); n > 0 {
		var sum float64
		var passed int
		for _, r := range results {
			sum += r.Score
			if r.Score >= passThreshold {
				passed++
			}
		}
		avg = sum / float64(n)
		passRate = 100 * float64(passed) / float64(n)
	}

	var b strings.Builder
	b.WriteString(reportHead)
	fmt.Fprintf(&b, reportStats, len(results), avg, maxScore, passThreshold, passRate)
	b.WriteString(reportTableHead)
	for _, r := range results {
		inputs := make([]string, 0, len(r.TestCase.PromptInputs))
		for _, k := range sortedKeys(r.TestCase.PromptInputs) {
			inputs = append(inputs, fmt.Sprintf("<strong>%s:</strong> %s", k, r.TestCase.PromptInputs[k]))
		}
		fmt.Fprintf(&b, reportRow,
			r.TestCase.Scenario,
			strings.Join(inputs, "<br>"),
			strings.Join(r.TestCase.SolutionCriteria, "<br>• "),
			r.Output,
			scoreClass(r.Score),
			r.Score,
			r.Reasoning,
		)
	}
	b.WriteString(reportTail)
	return b.String()
}

func scoreClass(score float64) string {
	switch {
	case score >= highThreshold:
		return "score-high"
	case score <= lowThreshold:
		return "score-low"
	default:
		return "score-medium"
	}
}
