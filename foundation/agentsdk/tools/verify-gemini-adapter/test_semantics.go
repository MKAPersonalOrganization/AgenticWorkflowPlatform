package main

import (
	"context"
	"fmt"
	"os"
	"school-with-ai/foundation/agentsdk/llm"
)

func runSemanticsTest() {
	ctx := context.Background()
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("GEMINI_API_KEY not set")
		os.Exit(1)
	}

	client, err := llm.NewGeminiClient(ctx, apiKey, "gemini-2.0-flash")
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	// Scenario: History ends with Model message. Adapter should append User continuation.
	req := &llm.Request{
		Messages: []llm.Message{
			llm.NewUserMessage("Hi"),
			{
				Role:    llm.RoleAssistant,
				Content: []llm.Content{llm.TextContent{Text: "Hello! How can I help?"}},
			},
		},
	}

	fmt.Println("Streaming response (History ends in Model):")
	for resp, err := range client.Generate(ctx, req) {
		if err != nil {
			fmt.Printf("Error stream: %v\n", err)
			break
		}
		if resp.Content != nil {
			for _, content := range resp.Content {
				switch c := content.(type) {
				case llm.TextContent:
					fmt.Print(c.Text)
				}
			}
		}
	}
	fmt.Println("\nDone.")
}
