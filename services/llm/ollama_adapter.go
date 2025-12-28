package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"

	"school-with-ai/domain"

	"github.com/ollama/ollama/api"
)

// OllamaProvider implements the Provider interface for Ollama
type OllamaProvider struct {
	client *api.Client
	model  string
}

// NewOllamaProvider creates a new Ollama provider with a model
// Connects to Ollama server at OLLAMA_HOST (defaults to http://localhost:11434)
func NewOllamaProvider(model string) (*OllamaProvider, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("ollama client: %w", err)
	}

	return &OllamaProvider{
		client: client,
		model:  model,
	}, nil
}

// NewOllamaProviderWithClient creates a new Ollama provider with a custom client
func NewOllamaProviderWithClient(client *api.Client, model string) *OllamaProvider {
	return &OllamaProvider{
		client: client,
		model:  model,
	}
}

// CreateStreamingCompletion implements the Provider interface
func (p *OllamaProvider) CreateStreamingCompletion(
	ctx context.Context,
	config CompletionConfig,
) iter.Seq2[StreamChunk, error] {
	return func(yield func(StreamChunk, error) bool) {
		// Convert messages to Ollama format
		messages := buildOllamaMessages(config.Messages)

		// Build chat request
		req := &api.ChatRequest{
			Model:    p.model,
			Messages: messages,
			Stream:   boolPtr(true),
		}

		// Add tools if provided
		if len(config.Tools) > 0 {
			req.Tools = buildOllamaTools(config.Tools)
		}

		// Track active tool calls by index
		toolCallsByIndex := make(map[int]*ToolCallDelta)
		var toolCallIndex int

		// Stream chat responses
		err := p.client.Chat(ctx, req, func(resp api.ChatResponse) error {
			streamChunk := StreamChunk{}

			// Handle text content
			if resp.Message.Content != "" {
				streamChunk.Content = resp.Message.Content
			}

			// Handle tool calls
			if len(resp.Message.ToolCalls) > 0 {
				for _, tc := range resp.Message.ToolCalls {
					// Check if this is a new tool call or continuation
					toolCall, exists := toolCallsByIndex[toolCallIndex]
					if !exists {
						// New tool call
						toolCall = &ToolCallDelta{
							Index: toolCallIndex,
							ID:    fmt.Sprintf("call_%d", toolCallIndex),
							Name:  tc.Function.Name,
						}
						toolCallsByIndex[toolCallIndex] = toolCall

						// Emit tool call start
						streamChunk.ToolCalls = append(streamChunk.ToolCalls, ToolCallDelta{
							Index: toolCall.Index,
							ID:    toolCall.ID,
							Name:  tc.Function.Name,
						})
					}

					// Add arguments
					if tc.Function.Arguments != nil {
						argsJSON, err := json.Marshal(tc.Function.Arguments)
						if err == nil {
							argsDelta := string(argsJSON)
							toolCall.Arguments = argsDelta

							// Emit arguments
							streamChunk.ToolCalls = append(streamChunk.ToolCalls, ToolCallDelta{
								Index:     toolCall.Index,
								Arguments: argsDelta,
							})
						}
					}

					toolCallIndex++
				}
			}

			// Yield the chunk
			if !yield(streamChunk, nil) {
				return fmt.Errorf("stream cancelled")
			}

			return nil
		})
		if err != nil {
			yield(StreamChunk{}, err)
		}
	}
}

// buildOllamaMessages converts domain messages to Ollama format
func buildOllamaMessages(messages []domain.Message) []api.Message {
	ollamaMessages := make([]api.Message, 0, len(messages))

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			ollamaMessages = append(ollamaMessages, api.Message{
				Role:    "user",
				Content: msg.Content,
			})

		case "assistant":
			ollamaMsg := api.Message{
				Role:    "assistant",
				Content: msg.Content,
			}

			// Check if assistant message has tool calls
			if len(msg.ToolCalls) > 0 {
				toolCalls := make([]api.ToolCall, 0, len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					// Parse arguments as JSON
					var args map[string]any
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
						toolCalls = append(toolCalls, api.ToolCall{
							Function: api.ToolCallFunction{
								Name:      tc.Function.Name,
								Arguments: args,
							},
						})
					}
				}
				ollamaMsg.ToolCalls = toolCalls
			}

			ollamaMessages = append(ollamaMessages, ollamaMsg)

		case "tool":
			// Tool result message
			ollamaMessages = append(ollamaMessages, api.Message{
				Role:    "tool",
				Content: msg.Content,
			})

		case "system":
			ollamaMessages = append(ollamaMessages, api.Message{
				Role:    "system",
				Content: msg.Content,
			})
		}
	}

	return ollamaMessages
}

// buildOllamaTools converts domain tools to Ollama format
func buildOllamaTools(tools []Tool) []api.Tool {
	ollamaTools := make([]api.Tool, 0, len(tools))

	for _, tool := range tools {
		ollamaTools = append(ollamaTools, api.Tool{
			Type: tool.Type,
			Function: api.ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  convertToToolParameters(tool.Parameters),
			},
		})
	}

	return ollamaTools
}

// convertToToolParameters converts a generic parameters map to Ollama's ToolFunctionParameters
func convertToToolParameters(params any) api.ToolFunctionParameters {
	if params == nil {
		return api.ToolFunctionParameters{}
	}

	paramsMap, ok := params.(map[string]any)
	if !ok {
		return api.ToolFunctionParameters{}
	}

	result := api.ToolFunctionParameters{}

	// Extract type
	if typeVal, ok := paramsMap["type"].(string); ok {
		result.Type = typeVal
	}

	// Extract required fields
	if requiredVal, ok := paramsMap["required"].([]any); ok {
		required := make([]string, 0, len(requiredVal))
		for _, r := range requiredVal {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
		result.Required = required
	}

	// Extract properties
	if propsVal, ok := paramsMap["properties"].(map[string]any); ok {
		properties := make(map[string]api.ToolProperty)
		for key, val := range propsVal {
			if propMap, ok := val.(map[string]any); ok {
				prop := api.ToolProperty{}
				if typeVal, ok := propMap["type"].(string); ok {
					prop.Type = api.PropertyType{typeVal}
				}
				if descVal, ok := propMap["description"].(string); ok {
					prop.Description = descVal
				}
				if enumVal, ok := propMap["enum"].([]any); ok {
					prop.Enum = enumVal
				}
				properties[key] = prop
			}
		}
		result.Properties = properties
	}

	// Extract other optional fields
	if defsVal, ok := paramsMap["$defs"]; ok {
		result.Defs = defsVal
	}
	if itemsVal, ok := paramsMap["items"]; ok {
		result.Items = itemsVal
	}

	return result
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// Heartbeat checks if Ollama server is running
func (p *OllamaProvider) Heartbeat(ctx context.Context) error {
	return p.client.Heartbeat(ctx)
}

// ListModels lists available models on the Ollama server
func (p *OllamaProvider) ListModels(ctx context.Context) (*api.ListResponse, error) {
	return p.client.List(ctx)
}

// Version returns the Ollama server version
func (p *OllamaProvider) Version(ctx context.Context) (string, error) {
	return p.client.Version(ctx)
}
