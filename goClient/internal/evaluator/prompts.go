package evaluator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Raw strings cannot hold backticks, so templates spell a code fence as ~~~.
const (
	fence       = "```"
	fenceMarker = "~~~"
)

func fenced(s string) string { return strings.ReplaceAll(s, fenceMarker, fence) }

const (
	ideasSystemPrompt     = "You are a test scenario designer specialized in creating diverse, unique testing scenarios."
	testCaseSystemPrompt  = "You are a test case creator specializing in designing evaluation scenarios."
	jsonPrefill           = fence + "json"
	extraCriteriaTemplate = `
Mandatory Requirements - ANY VIOLATION MEANS AUTOMATIC FAILURE (score of 3 or lower):
<extra_important_criteria>
{extra_criteria}
</extra_important_criteria>
`
)

var ideasTemplate = fenced(`
Generate {num_cases} unique, diverse ideas for testing a prompt that accomplishes this task:

<task_description>
{task_description}
</task_description>

The prompt will receive the following inputs
<prompt_inputs>
{prompt_inputs_spec}
</prompt_inputs>

Each idea should represent a distinct scenario or example that tests different aspects of the task.

Output Format:
Provide your response as a structured JSON array where each item is a brief description of the idea.

Example:
~~~json
[
    "Testing with technical computer science terminology",
    "Testing with medical research findings",
    "Testing with complex mathematical concepts",
    ...
]
~~~

Ensure each idea is:
- Clearly distinct from the others
- Relevant to the task description
- Specific enough to guide generation of a full test case
- Quick to solve without requiring extensive computation or multi-step processing
- Solvable with no more than 400 tokens of output

Remember, only generate {num_cases} unique ideas
`)

var testCaseTemplate = fenced(`
Generate a single detailed test case for a prompt evaluation based on:

<task_description>
{task_description}
</task_description>

<specific_idea>
{idea}
</specific_idea>

<allowed_input_keys>
{allowed_keys}
</allowed_input_keys>

Output Format:
~~~json
{{
    "prompt_inputs": {{
    {example_prompt_inputs}
    }},
    "solution_criteria": ["criterion 1", "criterion 2", ...] // Concise list of criteria for evaluating the solution, 1 to 4 items
}}
~~~

IMPORTANT REQUIREMENTS:
- You MUST ONLY use these exact input keys in your prompt_inputs: {allowed_keys}
- Do NOT add any additional keys to prompt_inputs
- All keys listed in allowed_input_keys must be included in your response
- Make the test case realistic and practically useful
- Include measurable, concise solution criteria
- The solution criteria should ONLY address the direct requirements of the task description and the generated prompt_inputs
- Avoid over-specifying criteria with requirements that go beyond the core task
- Keep solution criteria simple, focused, and directly tied to the fundamental task
- The test case should be tailored to the specific idea provided
- Quick to solve without requiring extensive computation or multi-step processing
- Solvable with no more than 400 tokens of output
- DO NOT include any fields beyond those specified in the output format

Here's an example of a sample input with an ideal output:
<sample_input>
<sample_task_description>
Extract topics out of a passage of text
</sample_task_description>
<sample_specific_idea>
Testing with a text that contains multiple nested topics and subtopics (e.g., a passage about renewable energy that covers solar power economics, wind turbine technology, and policy implications simultaneously)
</sample_specific_idea>

<sample_allowed_input_keys>
"content"
</sample_allowed_input_keys>
</sample_input>
<ideal_output>
~~~json
{
    "prompt_inputs": {
        "content": "The transition to renewable energy encompasses numerous interdependent dimensions. Solar photovoltaic technology has seen dramatic cost reductions, with panel efficiency improving 24% since 2010 while manufacturing costs declined by 89%, making it economically competitive with fossil fuels in many markets. Concurrently, wind energy has evolved through innovative turbine designs featuring carbon-fiber composite blades and advanced control systems that increase energy capture by 35% in low-wind conditions."
    },
    "solution_criteria": [
        "Includes all topics mentioned"
    ]
}
~~~
</ideal_output>
This is ideal output because the solution criteria is concise and doesn't ask for anything outside of the scope of the task description.
`)

const evalTemplate = `
Your task is to evaluate the following AI-generated solution with EXTREME RIGOR.

Original task description:
<task_description>
{task_description}
</task_description>

Original task inputs:
<task_inputs>
{{ {prompt_inputs} }}
</task_inputs>

Solution to Evaluate:
<solution>
{output}
</solution>

Criteria you should use to evaluate the solution:
<criteria>
{solution_criteria}
</criteria>

{extra_criteria_section}

Scoring Guidelines:
* Score 1-3: Solution fails to meet one or more MANDATORY requirements
* Score 4-6: Solution meets all mandatory requirements but has significant deficiencies in secondary criteria
* Score 7-8: Solution meets all mandatory requirements and most secondary criteria, with minor issues
* Score 9-10: Solution meets all mandatory and secondary criteria

IMPORTANT SCORING INSTRUCTIONS:
* Grade the output based ONLY on the listed criteria. Do not add your own extra requirements.
* If a solution meets all of the mandatory and secondary criteria give it a 10
* Don't complain that the solution "only" meets the mandatory and secondary criteria. Solutions shouldn't go above and beyond - they should meet the exact listed criteria.
* ANY violation of a mandatory requirement MUST result in a score of 3 or lower
* The full 1-10 scale should be utilized - don't hesitate to give low scores when warranted

Output Format
Provide your evaluation as a structured JSON object with the following fields, in this specific order:
- "strengths": An array of 1-3 key strengths
- "weaknesses": An array of 1-3 key areas for improvement
- "reasoning": A concise explanation of your overall assessment
- "score": A number between 1-10

Respond with JSON. Keep your response concise and direct.
Example response shape:
{{
    "strengths": string[],
    "weaknesses": string[],
    "reasoning": string,
    "score": number
}}
`

var placeholderRE = regexp.MustCompile(`\{([^{}]+)\}`)

// render substitutes {name} placeholders found in tmpl with vars[name], leaving
// unknown placeholders untouched, then turns "{{" and "}}" into "{" and "}".
// The unescape also applies to substituted values, as in the original notebook.
func render(tmpl string, vars map[string]string) string {
	result := tmpl
	for _, m := range placeholderRE.FindAllStringSubmatch(tmpl, -1) {
		if v, ok := vars[m[1]]; ok {
			result = strings.ReplaceAll(result, "{"+m[1]+"}", v)
		}
	}
	return strings.NewReplacer("{{", "{", "}}", "}").Replace(result)
}

// sortedKeys returns the keys of m in order, so prompts are deterministic.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func escapeNewlines(s string) string { return strings.ReplaceAll(s, "\n", `\n`) }

func ideasPrompt(task string, numCases int) string {
	// The notebook passes the inputs spec under the key "prompt_inputs" while
	// the template asks for {prompt_inputs_spec}, so the placeholder is sent
	// to the model unfilled. Kept for functional equivalence.
	return render(ideasTemplate, map[string]string{
		"task_description": task,
		"num_cases":        fmt.Sprint(numCases),
	})
}

func testCasePrompt(task, idea string, spec map[string]string) string {
	var examples strings.Builder
	keys := make([]string, 0, len(spec))
	for _, k := range sortedKeys(spec) {
		fmt.Fprintf(&examples, "\"%s\": \"EXAMPLE_VALUE\", // %s\n", k, escapeNewlines(spec[k]))
		keys = append(keys, `"`+k+`"`)
	}
	return render(testCaseTemplate, map[string]string{
		"allowed_keys":          strings.Join(keys, ", "),
		"task_description":      task,
		"idea":                  idea,
		"example_prompt_inputs": examples.String(),
	})
}

func gradePrompt(tc TestCase, output, extraCriteria string) string {
	var inputs strings.Builder
	for _, k := range sortedKeys(tc.PromptInputs) {
		fmt.Fprintf(&inputs, "\"%s\":\"%s\",\n", k, escapeNewlines(tc.PromptInputs[k]))
	}

	var extra string
	if extraCriteria != "" {
		extra = render(extraCriteriaTemplate, map[string]string{"extra_criteria": extraCriteria})
	}

	return render(evalTemplate, map[string]string{
		"task_description":       tc.TaskDescription,
		"prompt_inputs":          inputs.String(),
		"output":                 output,
		"solution_criteria":      strings.Join(tc.SolutionCriteria, "\n"),
		"extra_criteria_section": extra,
	})
}
