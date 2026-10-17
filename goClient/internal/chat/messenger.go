package chat

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Messenger is the subset of the Anthropic SDK used by Service.
// *anthropic.MessageService satisfies it.
type Messenger interface {
	New(ctx context.Context, params anthropic.MessageNewParams, opts ...option.RequestOption) (*anthropic.Message, error)
}
