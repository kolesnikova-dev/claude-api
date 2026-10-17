package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type fake struct{ err error }

func (f fake) New(context.Context, anthropic.MessageNewParams, ...option.RequestOption) (*anthropic.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	var m anthropic.Message
	_ = m.UnmarshalJSON([]byte(`{"content":[{"type":"text","text":"hi"}]}`))
	return &m, nil
}

func TestTmp(t *testing.T) {
	s := New(fake{}, WithMaxHistory(1))
	for i := 0; i < 3; i++ {
		if r, err := s.Chat(context.Background(), "x"); err != nil || r != "hi" {
			t.Fatal(r, err)
		}
	}
	if len(s.history) != 2 {
		t.Fatal(len(s.history))
	}
	e := errors.New("boom")
	s.messenger = fake{err: e}
	if _, err := s.Chat(context.Background(), "y"); !errors.Is(err, e) || len(s.history) != 2 {
		t.Fatal(err, len(s.history))
	}
}
