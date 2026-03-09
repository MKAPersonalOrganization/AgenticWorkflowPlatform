package main

import (
	"context"
	"fmt"
	"os"
	"school-with-ai/foundation/agentsdk/llm"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("GEMINI_API_KEY not set")
		os.Exit(1)
	}

	client, err := llm.NewGeminiClient(ctx, apiKey, "gemini-2.0-flash")
	if err != nil {
		os.Exit(1)
	}

	// Define a tool
	tool := llm.ToolDeclaration{
		Name:        "get_weather",
		Description: "Get the weather for a location",
		Parameters: &llm.Schema{
			Type: llm.TypeObject,
			Properties: map[string]*llm.Schema{
				"location": {
					Type:        llm.TypeString,
					Description: "The city and state, e.g. San Francisco, CA",
				},
			},
			Required: []string{"location"},
		},
	}

	req := &llm.Request{
		Messages: []llm.Message{
			llm.NewUserMessage("What is the weather in New York?"),
		},
		Tools: []llm.ToolDeclaration{tool},
	}

	fmt.Println("Streaming response:")
	for resp, err := range client.Generate(ctx, req) {
		if err != nil {
			break
		}
		if resp.Content != nil {
			for _, content := range resp.Content {
				switch c := content.(type) {
				case llm.TextContent:
					fmt.Print(c.Text)
				case llm.FunctionCall:
				}
			}
		}
		if resp.FinishReason != "" {
		}
		if resp.Usage != nil {
		}
	}
	fmt.Println("\nDone.")
}
