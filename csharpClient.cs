#:package Anthropic@12.58.0
#:package DotNetEnv@3.1.1
#:property JsonSerializerIsReflectionEnabledByDefault=true

using System;
using System.Threading.Tasks;
using Anthropic;
using Anthropic.Models.Messages;
using Anthropic.Exceptions;
using DotNetEnv;

// Load the .env file from the current directory
Env.Load();

var client = new AnthropicClient();

try
{
    var msg = await client.Messages
      .Create(new()
      {
          Model = Model.ClaudeSonnet5_5,
          MaxTokens = 1000,
          Messages = [new() {
              Role = Role.User,
              Content = "What is your programming language of preference?"
          }]
      });

    foreach (var block in msg.Content)
    {
        if (block.TryPickText(out var textBlock))
        {
            Console.WriteLine(textBlock.Text);
        }
    }
}
catch (AnthropicException ex)
{
    // Handle specific errors returned by the Anthropic API (e.g., invalid API key, overloaded server, rate limits)
    Console.Error.WriteLine($"Anthropic API Error: {ex.Message}");
    // You can inspect status codes or error details if exposed by the SDK's exception properties
}
catch (HttpRequestException ex)
{
    // Handle network-related issues (e.g., no internet connection, DNS failure)
    Console.Error.WriteLine($"Network error occurred: {ex.Message}");
}
catch (Exception ex)
{
    // Fallback for any other unexpected errors
    Console.Error.WriteLine($"An unexpected error occurred: {ex.Message}");
}