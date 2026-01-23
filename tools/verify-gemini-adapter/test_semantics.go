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
	msgs := []llm.ChatMessage{
		{
			Role: llm.ChatRoleUser,
			Contents: []llm.AIContent{
				llm.TextContent{Text: "Hi"},
			},
		},
		{
			Role: llm.ChatRoleAssistant,
			Contents: []llm.AIContent{
				llm.TextContent{Text: "Hello! How can I help?"},
			},
		},
	}

	opts := llm.ChatOptions{
		Temperature: nil,
	}

	fmt.Println("Streaming response (History ends in Model):")
	for update, err := range client.GetStreamingResponse(ctx, msgs, &opts) {
		if err != nil {
			fmt.Printf("Error stream: %v\n", err)
			break
		}
		if update.Contents != nil {
			for _, content := range update.Contents {
				switch c := content.(type) {
				case llm.TextContent:
					fmt.Print(c.Text)
				}
			}
		}
	}
	fmt.Println("\nDone.")
}
