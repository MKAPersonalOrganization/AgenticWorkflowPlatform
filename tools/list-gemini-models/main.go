package main

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/genai"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("GEMINI_API_KEY not set")
		os.Exit(1)
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		// Backend: genai.BackendGoogleAI,
	})
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	// client.Models.List returns (Page[Model], error)
	pager, err := client.Models.List(ctx, nil)
	if err != nil {
		fmt.Printf("Error listing models: %v\n", err)
		os.Exit(1)
	}

	for {
		for _, model := range pager.Items {
			fmt.Printf("Model: %s\n", model.Name)
			fmt.Printf("  DisplayName: %s\n", model.DisplayName)
			fmt.Printf("  SupportedActions: %v\n", model.SupportedActions)
			fmt.Println("---")
		}

		if pager.NextPageToken == "" {
			break
		}

		pager, err = pager.Next(ctx)
		if err != nil {
			fmt.Printf("Error getting next page: %v\n", err)
			break
		}
	}
}
