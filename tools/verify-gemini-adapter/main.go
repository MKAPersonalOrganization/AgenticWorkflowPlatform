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
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	// Define a tool
	tool := llm.NewAIFunctionTool(
		"get_weather",
		"Get the weather for a location",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city and state, e.g. San Francisco, CA",
				},
			},
			"required": []interface{}{"location"},
		},
	)

	msgs := []llm.ChatMessage{
		{
			Role: llm.ChatRoleUser,
			Contents: []llm.AIContent{
				llm.TextContent{Text: "What is the weather in New York?"},
			},
		},
	}

	temp := 0.7
	opts := llm.ChatOptions{
		Temperature: &temp,
		Tools: []llm.AITool{tool},
		ToolMode: llm.ToolModeAuto,
	}

	fmt.Println("Streaming response:")
	for update, err := range client.GetStreamingResponse(ctx, msgs, &opts) {
		if err != nil {
			fmt.Printf("Error stream: %v\n", err)
			break
		}
		if err != nil {
			fmt.Printf("Error stream: %v\n", err)
			break
		}
		if update.Contents != nil {
			for _, content := range update.Contents {
				switch c := content.(type) {
				case llm.TextContent:
					fmt.Print(c.Text)
				case llm.FunctionCallContent:
					fmt.Printf("\n[ToolCall] Name: %s, ID: %s, Args: %s\n", c.Name, c.CallID, c.ArgumentsJSON())
				case llm.UsageContent:
					fmt.Printf("\n[Usage] Input: %d, Output: %d\n", c.InputTokens, c.OutputTokens)
				}
			}
		}
		if update.FinishReason != "" {
			fmt.Printf("\n[FinishReason] %s\n", update.FinishReason)
		}
		if update.Usage != nil {
			fmt.Printf("\n[Usage] Input: %d, Output: %d\n", update.Usage.InputTokens, update.Usage.OutputTokens)
		}
	}
	fmt.Println("\nDone.")
}
