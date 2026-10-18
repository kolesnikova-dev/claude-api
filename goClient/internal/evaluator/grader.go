// Package evaluator generates test datasets for a prompt, runs the prompt on
// them, grades the outputs with a model and reports the results.
package evaluator

import (
	"context"
	"errors"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// maxTokens is the response limit for every completion.
const maxTokens = 1000

// ErrNoText is returned when the first content block of a response is not text.
var ErrNoText = errors.New("evaluator: response did not start with a text block")

// Request is a single stateless model call.
type Request struct {
	System string // omitted when empty
	Prompt string // sent as the user message
	// Prefill, when non-empty, is sent as a trailing assistant message so the
	// model continues from it. The returned text does not include the prefill.
	Prefill       string
	Temperature   float64 // always sent, so 0 means 0 and not the API default
	StopSequences []string
}

// Grader performs one-shot model calls.
type Grader interface {
	Grade(ctx context.Context, req Request) (string, error)
}

// messenger is the subset of the Anthropic SDK used by AnthropicGrader.
// *anthropic.MessageService satisfies it.
type messenger interface {
	New(ctx context.Context, params anthropic.MessageNewParams, opts ...option.RequestOption) (*anthropic.Message, error)
}

// AnthropicGrader is a Grader backed by the Anthropic Messages API.
type AnthropicGrader struct {
	m     messenger
	model anthropic.Model
}

// NewAnthropicGrader returns a Grader that calls m. An empty model
// selects Claude Haiku 4.5.
func NewAnthropicGrader(m messenger, model anthropic.Model) *AnthropicGrader {
	if model == "" {
		model = anthropic.ModelClaudeHaiku4_5
	}
	return &AnthropicGrader{m: m, model: model}
}

// Grade sends req and returns the text of the first content block.
func (c *AnthropicGrader) Grade(ctx context.Context, req Request) (string, error) {
	msgs := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt)),
	}
	if req.Prefill != "" {
		msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(req.Prefill)))
	}

	params := anthropic.MessageNewParams{
		Model:         c.model,
		MaxTokens:     maxTokens,
		Messages:      msgs,
		Temperature:   anthropic.Float(req.Temperature),
		StopSequences: req.StopSequences,
	}
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}

	msg, err := c.m.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("evaluator: send message: %w", err)
	}
	if len(msg.Content) == 0 {
		return "", ErrNoText
	}
	tb, ok := msg.Content[0].AsAny().(anthropic.TextBlock)
	if !ok {
		return "", ErrNoText
	}
	return tb.Text, nil
}
