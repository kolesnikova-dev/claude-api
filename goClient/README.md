# Go Client Quick Summary
- internal/chat/service.go has Service, the options (WithModel, WithMaxTokens, WithSystemPrompt, WithMaxHistory), AddUserMessage, AddAssistantMessage and Chat(ctx, text).
- internal/chat/messenger.go defines the small interface the service depends on. The SDK's client.Messages satisfies it, so tests can fake it.
- internal/chat/errors.go has the sentinel errors: ErrEmptyMessage, ErrInvalidOrder, ErrNoTextContent and ErrBusy.
- internal/evaluator contains a PromptEvaluator. Evaluator (New, WithMaxConcurrency, WithOutput) has GenerateDataset and RunEvaluation, plus the per-step methods. GenerateReport builds the HTML report. Grader is a small stateless model-call interface (system, prefill, temperature, stop sequences), and NewAnthropicGrader backs it with client.Messages.


*To Review*

- Lock during the API call: the service doesn't hold its lock while waiting on the API.
Concurrent calls: a second call that arrives while one is in flight returns ErrBusy.
- Alternation: adding a message that would break user/assistant order returns ErrInvalidOrder. This avoids a 400 from the API.
- Failed Chat: the history is restored to what it was before the call.
- Silent skip: if .env is missing, it logs a line and carries on. A missing ANTHROPIC_API_KEY only fails at the API call.