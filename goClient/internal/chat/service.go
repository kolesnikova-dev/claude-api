// Package chat provides a conversation layer on top of the Anthropic Messages API.
package chat

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
)

const defaultMaxTokens = 1024

// Service holds the history of a single conversation.
//
// It is safe for concurrent use, but a conversation is inherently serial:
// while a Chat call is in flight, other calls return ErrBusy.
type Service struct {
	messenger   Messenger
	model       anthropic.Model
	maxTokens   int64
	system      string
	maxHistory  int // in user/assistant turns; 0 means unbounded
	temperature float64

	mu       sync.Mutex
	history  []anthropic.MessageParam
	inflight bool
}

// Option configures a Service.
type Option func(*Service)

// WithModel sets the model. Empty values are ignored.
func WithModel(m anthropic.Model) Option {
	return func(s *Service) {
		if m != "" {
			s.model = m
		}
	}
}

// WithMaxTokens sets the response token limit. Values <= 0 are ignored.
func WithMaxTokens(n int64) Option {
	return func(s *Service) {
		if n > 0 {
			s.maxTokens = n
		}
	}
}

// WithSystemPrompt sets the system prompt.
func WithTemperature(t float64) Option {
	return func(s *Service) { s.temperature = t }
}

// WithSystemPrompt sets the system prompt.
func WithSystemPrompt(p string) Option {
	return func(s *Service) { s.system = p }
}

// WithMaxHistory keeps at most n user/assistant turns, dropping the oldest.
// Values <= 0 leave history unbounded.
func WithMaxHistory(n int) Option {
	return func(s *Service) {
		if n > 0 {
			s.maxHistory = n
		}
	}
}

// New returns a Service that talks to m.
func New(m Messenger, opts ...Option) *Service {
	s := &Service{
		messenger: m,
		model:     anthropic.ModelClaudeHaiku4_5,
		maxTokens: defaultMaxTokens,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// AddUserMessage appends a user message, e.g. to seed history.
func (s *Service) AddUserMessage(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inflight {
		return ErrBusy
	}
	return s.addUserLocked(text)
}

// AddAssistantMessage appends an assistant message, e.g. to seed history.
func (s *Service) AddAssistantMessage(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inflight {
		return ErrBusy
	}
	return s.addAssistantLocked(text)
}

func (s *Service) ChatStream(ctx context.Context, text string, onDelta func(string)) (string, error) {
	s.mu.Lock()
	if s.inflight {
		s.mu.Unlock()
		return "", ErrBusy
	}
	before := len(s.history)
	if err := s.addUserLocked(text); err != nil {
		s.mu.Unlock()
		return "", err
	}
	s.inflight = true
	snapshot := append([]anthropic.MessageParam(nil), s.history...)
	s.mu.Unlock()

	params := s.buildParams(snapshot)

	stream := s.messenger.NewStreaming(ctx, params)
	var sb strings.Builder
	for stream.Next() {
		if ev, ok := stream.Current().AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if td, ok := ev.Delta.AsAny().(anthropic.TextDelta); ok {
				sb.WriteString(td.Text)
				onDelta(td.Text) // caller prints it
			}
		}
	}
	err := stream.Err()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.inflight = false

	if err != nil {
		s.history = s.history[:before]
		return "", fmt.Errorf("chat: stream: %w", err)
	}
	reply := sb.String()
	if strings.TrimSpace(sb.String()) == "" {
		s.history = s.history[:before]
		return "", ErrNoTextContent
	}

	// addAssistantLocked cannot fail: reply is non-empty and the last message is the user's.
	_ = s.addAssistantLocked(reply)
	s.trimLocked()
	return reply, nil
}

// Chat appends text as a user message, sends the conversation to the model,
// appends the reply as an assistant message and returns its text.
// On failure the history is left unchanged.
func (s *Service) Chat(ctx context.Context, text string) (string, error) {
	s.mu.Lock()
	if s.inflight {
		s.mu.Unlock()
		return "", ErrBusy
	}
	before := len(s.history)
	if err := s.addUserLocked(text); err != nil {
		s.mu.Unlock()
		return "", err
	}
	s.inflight = true
	snapshot := append([]anthropic.MessageParam(nil), s.history...)
	s.mu.Unlock()

	reply, err := s.send(ctx, snapshot)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.inflight = false
	if err != nil {
		s.history = s.history[:before]
		return "", err
	}
	// addAssistantLocked cannot fail: reply is non-empty and the last message is the user's.
	_ = s.addAssistantLocked(reply)
	s.trimLocked()
	return reply, nil
}

func (s *Service) buildParams(msgs []anthropic.MessageParam) anthropic.MessageNewParams {
	params := anthropic.MessageNewParams{
		Model:     s.model,
		MaxTokens: s.maxTokens,
		Messages:  msgs,
	}
	if s.system != "" {
		params.System = []anthropic.TextBlockParam{{Text: s.system}}
	}

	if s.temperature != 0 {
		params.Temperature = anthropic.Float(s.temperature)
	}
	return params
}

func (s *Service) send(ctx context.Context, msgs []anthropic.MessageParam) (string, error) {
	params := anthropic.MessageNewParams{
		Model:     s.model,
		MaxTokens: s.maxTokens,
		Messages:  msgs,
	}
	if s.system != "" {
		params.System = []anthropic.TextBlockParam{{Text: s.system}}
	}

	if s.temperature != 0 {
		params.Temperature = anthropic.Float(s.temperature)
	}

	msg, err := s.messenger.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("chat: send message: %w", err)
	}

	var sb strings.Builder
	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			sb.WriteString(tb.Text)
		}
	}
	if strings.TrimSpace(sb.String()) == "" {
		return "", ErrNoTextContent
	}
	return sb.String(), nil
}

func (s *Service) addUserLocked(text string) error {
	if strings.TrimSpace(text) == "" {
		return ErrEmptyMessage
	}
	if n := len(s.history); n > 0 && s.history[n-1].Role != anthropic.MessageParamRoleAssistant {
		return ErrInvalidOrder
	}
	s.history = append(s.history, anthropic.NewUserMessage(anthropic.NewTextBlock(text)))
	return nil
}

func (s *Service) addAssistantLocked(text string) error {
	if strings.TrimSpace(text) == "" {
		return ErrEmptyMessage
	}
	if n := len(s.history); n == 0 || s.history[n-1].Role != anthropic.MessageParamRoleUser {
		return ErrInvalidOrder
	}
	s.history = append(s.history, anthropic.NewAssistantMessage(anthropic.NewTextBlock(text)))
	return nil
}

// trimLocked drops the oldest turns so history starts with a user message
// and holds at most maxHistory user/assistant pairs.
func (s *Service) trimLocked() {
	if s.maxHistory <= 0 {
		return
	}
	if excess := len(s.history) - 2*s.maxHistory; excess > 0 {
		// Round up to an even count to remove whole pairs.
		excess += excess % 2
		s.history = append([]anthropic.MessageParam(nil), s.history[excess:]...)
	}
}
