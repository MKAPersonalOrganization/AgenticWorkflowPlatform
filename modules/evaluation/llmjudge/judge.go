package llmjudge

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// Judge calls the Gemini API to evaluate agent responses semantically.
// Pattern follows adapters/gemini/search_client.go.
type Judge struct {
	apiKey string
	model  string
}

// New creates a new Judge with the given API key and model name.
func New(apiKey, model string) *Judge {
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return &Judge{apiKey: apiKey, model: model}
}

// Score sends a prompt to the LLM and returns the raw text response.
func (j *Judge) Score(ctx context.Context, prompt string) (string, error) {
	if j.apiKey == "" {
		return "", fmt.Errorf("judge API key not configured")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: j.apiKey,
	})
	if err != nil {
		return "", fmt.Errorf("creating Gemini client: %w", err)
	}

	resp, err := client.Models.GenerateContent(ctx, j.model, []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{{Text: prompt}},
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("calling Gemini API: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 {
		return "", fmt.Errorf("empty response from Gemini API")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return "", fmt.Errorf("no content in Gemini response")
	}

	var result string
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			result += part.Text
		}
	}

	return result, nil
}
