from dotenv import load_dotenv
load_dotenv()

from anthropic import Anthropic

client = Anthropic()
model="claude-sonnet-5-5"

message = client.messages.create(
    model=model,
    max_tokens=1000,
    messages=[{"role": "user", "content": "What is quantum entanglement? Answer in 1 sentence"}],
)
print(message.content)