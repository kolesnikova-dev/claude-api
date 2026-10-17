package chat

import "errors"

var (
	// ErrEmptyMessage is returned when a message has no non-whitespace text.
	ErrEmptyMessage = errors.New("chat: empty message")
	// ErrInvalidOrder is returned when a message would break user/assistant alternation.
	ErrInvalidOrder = errors.New("chat: message breaks user/assistant alternation")
	// ErrNoTextContent is returned when the model response contains no text blocks.
	ErrNoTextContent = errors.New("chat: response contained no text")
	// ErrBusy is returned when the Service is already handling a Chat call.
	ErrBusy = errors.New("chat: another request is in progress")
)
