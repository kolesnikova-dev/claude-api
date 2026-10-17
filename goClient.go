import "github.com/anthropics/anthropic-sdk-go"

client := anthropic.NewClient()
msg, _ := client.Messages.New(
  context.TODO(),
  anthropic.MessageNewParams{
    Model:     anthropic.ModelClaudeOpus5_5,
    MaxTokens: 1024,
    Messages: []anthropic.MessageParam{
      anthropic.NewUserMessage(
        anthropic.NewTextBlock("Hello, Claude"),
      ),
    },
  },
)
for _, block := range msg.Content {
  if textBlock, ok := block.AsAny().(anthropic.TextBlock); ok {
    fmt.Println(textBlock.Text)
  }
}