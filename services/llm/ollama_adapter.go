package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"

	"github.com/ollama/ollama/api"
)

// OllamaClient implements the ChatClient interface for Ollama.
// It provides streaming chat completions using the Ollama API.
type OllamaClient struct {
	client *api.Client
	model  string
}

// NewOllamaClient creates a new Ollama client with the specified model.
// Connects to Ollama server at OLLAMA_HOST (defaults to http://localhost:11434).
func NewOllamaClient(model string) (*OllamaClient, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("ollama client: %w", err)
	}

	return &OllamaClient{
		client: client,
		model:  model,
	}, nil
}

// NewOllamaClientWithClient creates a new Ollama client with a custom api.Client.
func NewOllamaClientWithClient(client *api.Client, model string) *OllamaClient {
	return &OllamaClient{
		client: client,
		model:  model,
	}
}

// GetStreamingResponse implements the ChatClient interface.
// It sends messages to Ollama and streams back response updates.
func (c *OllamaClient) GetStreamingResponse(
	ctx context.Context,
	messages []ChatMessage,
	options *ChatOptions,
) iter.Seq2[ChatResponseUpdate, error] {
	return func(yield func(ChatResponseUpdate, error) bool) {
		// Build Ollama messages
		ollamaMessages := c.buildMessages(messages, options)

		// Build chat request
		req := &api.ChatRequest{
			Model:    c.getModel(options),
			Messages: ollamaMessages,
			Stream:   boolPtr(true),
		}

		// Apply options
		c.applyOptions(req, options)

		// Track tool calls by index for generating IDs
		var toolCallIndex int

		// Stream chat responses
		err := c.client.Chat(ctx, req, func(resp api.ChatResponse) error {
			// Handle text content
			if resp.Message.Content != "" {
				update := ChatResponseUpdate{
					Role:     ChatRoleAssistant,
					Contents: []AIContent{TextContent{Text: resp.Message.Content}},
				}
				if !yield(update, nil) {
					return fmt.Errorf("stream cancelled")
				}
			}

			// Handle tool calls
			for _, tc := range resp.Message.ToolCalls {
				callID := fmt.Sprintf("call_%d", toolCallIndex)

				// Marshal arguments to JSON
				var argsJSON string
				if tc.Function.Arguments != nil {
					if data, err := json.Marshal(tc.Function.Arguments); err == nil {
						argsJSON = string(data)
					}
				}

				// Emit tool call update
				update := ChatResponseUpdate{
					Role:          ChatRoleAssistant,
					ToolCallIndex: toolCallIndex,
					ToolCallID:    callID,
					Contents: []AIContent{FunctionCallContent{
						CallID:        callID,
						Name:          tc.Function.Name,
						ArgumentsJSON: argsJSON,
					}},
				}

				if !yield(update, nil) {
					return fmt.Errorf("stream cancelled")
				}

				toolCallIndex++
			}

			// Check for completion
			if resp.Done {
				finalUpdate := ChatResponseUpdate{
					Role:         ChatRoleAssistant,
					FinishReason: c.mapFinishReason(resp),
				}
				if !yield(finalUpdate, nil) {
					return fmt.Errorf("stream cancelled")
				}
			}

			return nil
		})
		if err != nil {
			yield(ChatResponseUpdate{}, err)
		}
	}
}

// buildMessages converts ChatMessage slice to Ollama message format.
func (c *OllamaClient) buildMessages(messages []ChatMessage, options *ChatOptions) []api.Message {
	ollamaMessages := make([]api.Message, 0, len(messages)+1)

	// Prepend system message if instructions are provided
	if options != nil && options.Instructions != "" {
		ollamaMessages = append(ollamaMessages, api.Message{
			Role:    "system",
			Content: options.Instructions,
		})
	}

	// Convert each message
	for _, msg := range messages {
		switch msg.Role {
		case ChatRoleUser:
			ollamaMessages = append(ollamaMessages, api.Message{
				Role:    "user",
				Content: msg.Text(),
			})

		case ChatRoleAssistant:
			ollamaMsg := api.Message{
				Role:    "assistant",
				Content: msg.Text(),
			}

			// Convert function calls to tool calls
			functionCalls := msg.FunctionCalls()
			if len(functionCalls) > 0 {
				toolCalls := make([]api.ToolCall, 0, len(functionCalls))
				for _, fc := range functionCalls {
					// Parse arguments
					var args map[string]any
					if fc.Arguments != nil {
						args = fc.Arguments
					} else if fc.ArgumentsJSON != "" {
						json.Unmarshal([]byte(fc.ArgumentsJSON), &args)
					}

					toolCalls = append(toolCalls, api.ToolCall{
						Function: api.ToolCallFunction{
							Name:      fc.Name,
							Arguments: args,
						},
					})
				}
				ollamaMsg.ToolCalls = toolCalls
			}

			ollamaMessages = append(ollamaMessages, ollamaMsg)

		case ChatRoleTool:
			for _, fr := range msg.FunctionResults() {
				content := fr.Result
				if fr.IsError() {
					content = fr.Error
				}
				ollamaMessages = append(ollamaMessages, api.Message{
					Role:    "tool",
					Content: content,
				})
			}

		case ChatRoleSystem:
			ollamaMessages = append(ollamaMessages, api.Message{
				Role:    "system",
				Content: msg.Text(),
			})
		}
	}

	return ollamaMessages
}

// applyOptions applies ChatOptions to the Ollama chat request.
func (c *OllamaClient) applyOptions(req *api.ChatRequest, options *ChatOptions) {
	if options == nil {
		return
	}

	// Apply options as a map
	opts := make(map[string]any)

	if options.Temperature != nil {
		opts["temperature"] = *options.Temperature
	}

	if options.MaxOutputTokens != nil {
		opts["num_predict"] = *options.MaxOutputTokens
	}

	if options.TopP != nil {
		opts["top_p"] = *options.TopP
	}

	if options.Seed != nil {
		opts["seed"] = *options.Seed
	}

	if len(options.StopSequences) > 0 {
		opts["stop"] = options.StopSequences
	}

	if len(opts) > 0 {
		req.Options = opts
	}

	// Apply tools
	if len(options.Tools) > 0 {
		req.Tools = c.buildTools(options.Tools)
	}
}

// buildTools converts AITool slice to Ollama tool format.
func (c *OllamaClient) buildTools(tools []AITool) []api.Tool {
	ollamaTools := make([]api.Tool, 0, len(tools))

	for _, tool := range tools {
		ollamaTools = append(ollamaTools, api.Tool{
			Type: "function",
			Function: api.ToolFunction{
				Name:        tool.GetName(),
				Description: tool.GetDescription(),
				Parameters:  convertToToolParameters(tool.GetParameters()),
			},
		})
	}

	return ollamaTools
}

// getModel returns the model to use, preferring options.ModelID if set.
func (c *OllamaClient) getModel(options *ChatOptions) string {
	if options != nil && options.ModelID != "" {
		return options.ModelID
	}
	return c.model
}

// mapFinishReason determines the finish reason from Ollama response.
func (c *OllamaClient) mapFinishReason(resp api.ChatResponse) FinishReason {
	if resp.DoneReason == "stop" {
		return FinishReasonStop
	}
	if resp.DoneReason == "length" {
		return FinishReasonLength
	}
	// Check if the last response had tool calls
	if len(resp.Message.ToolCalls) > 0 {
		return FinishReasonToolCalls
	}
	return FinishReasonStop
}

// convertToToolParameters converts a generic parameters map to Ollama's ToolFunctionParameters.
func convertToToolParameters(params map[string]any) api.ToolFunctionParameters {
	if params == nil {
		return api.ToolFunctionParameters{}
	}

	result := api.ToolFunctionParameters{}

	// Extract type
	if typeVal, ok := params["type"].(string); ok {
		result.Type = typeVal
	}

	// Extract required fields
	if requiredVal, ok := params["required"].([]any); ok {
		required := make([]string, 0, len(requiredVal))
		for _, r := range requiredVal {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
		result.Required = required
	}

	// Extract properties
	if propsVal, ok := params["properties"].(map[string]any); ok {
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
	if defsVal, ok := params["$defs"]; ok {
		result.Defs = defsVal
	}
	if itemsVal, ok := params["items"]; ok {
		result.Items = itemsVal
	}

	return result
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool {
	return &b
}

// Heartbeat checks if Ollama server is running.
func (c *OllamaClient) Heartbeat(ctx context.Context) error {
	return c.client.Heartbeat(ctx)
}

// ListModels lists available models on the Ollama server.
func (c *OllamaClient) ListModels(ctx context.Context) (*api.ListResponse, error) {
	return c.client.List(ctx)
}

// Version returns the Ollama server version.
func (c *OllamaClient) Version(ctx context.Context) (string, error) {
	return c.client.Version(ctx)
}
