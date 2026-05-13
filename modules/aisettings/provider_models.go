package aisettings

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ProviderModel is a single chat-capable model returned by a provider's model-listing API.
type ProviderModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

var providerHTTPClient = &http.Client{Timeout: 10 * time.Second}

// fetchGeminiModels calls the Generative Language API and returns only models
// that support generateContent (i.e. chat/text generation).
func fetchGeminiModels(ctx context.Context, apiKey string) ([]ProviderModel, error) {
	url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + apiKey + "&pageSize=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating Gemini request: %w", err)
	}

	resp, err := providerHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Gemini models API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini models API returned status %d — check your API key", resp.StatusCode)
	}

	var body struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding Gemini response: %w", err)
	}

	var models []ProviderModel
	for _, m := range body.Models {
		if !containsString(m.SupportedGenerationMethods, "generateContent") {
			continue
		}
		id := strings.TrimPrefix(m.Name, "models/")
		displayName := m.DisplayName
		if displayName == "" {
			displayName = id
		}
		models = append(models, ProviderModel{ID: id, DisplayName: displayName})
	}
	return models, nil
}

// fetchOpenAIModels calls the OpenAI models API and filters to chat-capable models.
func fetchOpenAIModels(ctx context.Context, apiKey string) ([]ProviderModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("creating OpenAI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := providerHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling OpenAI models API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI models API returned status %d — check your API key", resp.StatusCode)
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding OpenAI response: %w", err)
	}

	// Prefixes that identify non-chat models (embeddings, image gen, audio, etc.)
	nonChatPrefixes := []string{
		"dall-e", "tts-", "whisper-", "text-embedding-",
		"text-moderation-", "babbage-", "davinci-", "curie-", "ada-",
	}

	var models []ProviderModel
	for _, m := range body.Data {
		if hasAnyPrefix(m.ID, nonChatPrefixes) {
			continue
		}
		models = append(models, ProviderModel{ID: m.ID, DisplayName: m.ID})
	}
	return models, nil
}

// fetchAnthropicModels calls the Anthropic models API.
// All models returned are chat-capable — no filtering needed.
func fetchAnthropicModels(ctx context.Context, apiKey string) ([]ProviderModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/v1/models?limit=100", nil)
	if err != nil {
		return nil, fmt.Errorf("creating Anthropic request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := providerHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Anthropic models API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic models API returned status %d — check your API key", resp.StatusCode)
	}

	var body struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding Anthropic response: %w", err)
	}

	var models []ProviderModel
	for _, m := range body.Data {
		displayName := m.DisplayName
		if displayName == "" {
			displayName = m.ID
		}
		models = append(models, ProviderModel{ID: m.ID, DisplayName: displayName})
	}
	return models, nil
}

func containsString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
