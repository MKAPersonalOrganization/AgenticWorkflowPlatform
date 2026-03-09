package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/ollama/ollama/api"
)

// ErrStreamCancelled is returned when the stream is cancelled by the consumer.
var ErrStreamCancelled = errors.New("stream cancelled")

// OllamaClient implements the LLM interface for Ollama.
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
		return nil, &Error{
			Op:   "ollama.NewClient",
			Kind: KindInternal,
			Err:  err,
		}
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

// Generate implements the LLM interface.
// It sends messages to Ollama and streams back response updates.
func (c *OllamaClient) Generate(
	ctx context.Context,
	req *Request,
) iter.Seq2[*Response, error] {
	return func(yield func(*Response, error) bool) {
		// Build Ollama messages
		ollamaMessages := c.buildMessages(req.Messages, req.System)

		// Build chat request
		apiReq := &api.ChatRequest{
			Model:    c.getModel(req.Extensions),
			Messages: ollamaMessages,
			Stream:   boolPtr(true),
		}

		// Apply options
		c.applyOptions(apiReq, req)

		// Track tool calls by index for generating IDs
		var toolCallIndex int

		// Stream chat responses
		err := c.client.Chat(ctx, apiReq, func(resp api.ChatResponse) error {
			// Handle text content
			if resp.Message.Content != "" {
				res := &Response{
					Content: []Content{TextContent{Text: resp.Message.Content}},
					Partial: true,
					Extensions: Metadata{
						"model":    resp.Model,
						"provider": "ollama",
					},
				}
				if !yield(res, nil) {
					return ErrStreamCancelled
				}
			}

			// Handle tool calls
			for _, tc := range resp.Message.ToolCalls {
				callID := fmt.Sprintf("call_%d", toolCallIndex)

				// Marshal arguments to JSON
				var argsJSON []byte
				if tc.Function.Arguments != nil {
					data, err := json.Marshal(tc.Function.Arguments)
					if err != nil {
						return fmt.Errorf("failed to marshal tool call arguments: %w", err)
					}
					argsJSON = data
				}

				// Emit tool call update
				res := &Response{
					Partial: true,
					Content: []Content{FunctionCall{
						ID:        callID,
						Name:      tc.Function.Name,
						Arguments: json.RawMessage(argsJSON),
					}},
					Extensions: Metadata{
						"model":    resp.Model,
						"provider": "ollama",
					},
				}

				if !yield(res, nil) {
					return ErrStreamCancelled
				}

				toolCallIndex++
			}

			// Check for completion
			if resp.Done {
				finalResp := &Response{
					FinishReason: c.mapFinishReason(resp),
					Partial:      false,
					Extensions: Metadata{
						"model":    resp.Model,
						"provider": "ollama",
					},
				}
				if !yield(finalResp, nil) {
					return ErrStreamCancelled
				}
			}

			return nil
		})
		if err != nil {
			yield(nil, c.mapError("ollama.Generate", err))
		}
	}
}

// buildMessages converts Message slice to Ollama message format.
func (c *OllamaClient) buildMessages(messages []Message, system string) []api.Message {
	ollamaMessages := make([]api.Message, 0, len(messages)+1)

	// Prepend system message if instructions are provided
	if system != "" {
		ollamaMessages = append(ollamaMessages, api.Message{
			Role:    "system",
			Content: system,
		})
	}

	// Convert each message
	for _, msg := range messages {
		switch msg.Role {
		case RoleUser:
			userMsg := api.Message{
				Role:    "user",
				Content: msg.Text(),
			}
			// Add images for multimodal models (Ollama uses raw bytes, images only)
			for _, img := range msg.ImageURLs() {
				if strings.HasPrefix(img.URL, "data:") {
					if idx := strings.Index(img.URL, ","); idx > 0 {
						b64Data := img.URL[idx+1:]
						decoded, err := base64.StdEncoding.DecodeString(b64Data)
						if err == nil {
							userMsg.Images = append(userMsg.Images, api.ImageData(decoded))
						}
					}
				}
			}
			// Note: Ollama does not natively support audio or file attachments;
			// they are silently skipped here.
			ollamaMessages = append(ollamaMessages, userMsg)

		case RoleAssistant:
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
						_ = json.Unmarshal(fc.Arguments, &args)
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

		case RoleTool:
			for _, fr := range msg.FunctionResults() {
				ollamaMessages = append(ollamaMessages, api.Message{
					Role:    "tool",
					Content: string(fr.Result),
				})
			}

		case RoleSystem:
			ollamaMessages = append(ollamaMessages, api.Message{
				Role:    "system",
				Content: msg.Text(),
			})
		}
	}

	return ollamaMessages
}

// applyOptions applies Parameters and Tools to the Ollama chat request.
func (c *OllamaClient) applyOptions(apiReq *api.ChatRequest, req *Request) {
	// Apply options as a map if provided
	if req.Parameters != nil {
		opts := make(map[string]any)
		p := req.Parameters

		if p.Temperature != nil {
			opts["temperature"] = *p.Temperature
		}

		if p.MaxTokens > 0 {
			opts["num_predict"] = p.MaxTokens
		}

		if p.TopP != nil {
			opts["top_p"] = *p.TopP
		}

		if len(p.StopSequences) > 0 {
			opts["stop"] = p.StopSequences
		}

		if len(opts) > 0 {
			apiReq.Options = opts
		}
	}

	// Apply extensions
	if req.Extensions != nil {
		if seed, ok := req.Extensions["seed"].(int64); ok {
			if apiReq.Options == nil {
				apiReq.Options = make(map[string]any)
			}
			apiReq.Options["seed"] = seed
		}
	}

	// Apply tools if provided
	if len(req.Tools) > 0 {
		apiReq.Tools = c.buildTools(req.Tools)
	}
}

// buildTools converts ToolDeclaration slice to Ollama tool format.
func (c *OllamaClient) buildTools(tools []ToolDeclaration) []api.Tool {
	ollamaTools := make([]api.Tool, 0, len(tools))

	for _, tool := range tools {
		var paramsMap map[string]any
		if tool.Parameters != nil {
			data, _ := json.Marshal(tool.Parameters)
			_ = json.Unmarshal(data, &paramsMap)
		}

		ollamaTools = append(ollamaTools, api.Tool{
			Type: "function",
			Function: api.ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  convertToToolParameters(paramsMap),
			},
		})
	}

	return ollamaTools
}

// getModel returns the model to use, preferring extensions["model"] if set.
func (c *OllamaClient) getModel(extensions Metadata) string {
	if extensions != nil {
		if model, ok := extensions["model"].(string); ok {
			return model
		}
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
		return FinishReasonToolUse
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
	err := c.client.Heartbeat(ctx)
	if err != nil {
		return fmt.Errorf("ollama heartbeat failed: %w", err)
	}

	return nil
}

func (c *OllamaClient) mapError(op string, err error) error {
	if err == nil {
		return nil
	}

	kind := KindInternal
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit"):
		kind = KindRateLimit
	case strings.Contains(errStr, "400") || strings.Contains(errStr, "invalid"):
		kind = KindInvalidInput
	case strings.Contains(errStr, "404") || strings.Contains(errStr, "not found"):
		kind = KindNotFound
	}

	return &Error{
		Op:   op,
		Kind: kind,
		Err:  err,
		Context: map[string]any{
			"model": c.model,
		},
	}
}

// ListModels lists available models on the Ollama server.
func (c *OllamaClient) ListModels(ctx context.Context) (*api.ListResponse, error) {
	resp, err := c.client.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list ollama models: %w", err)
	}

	return resp, nil
}

// Version returns the Ollama server version.
func (c *OllamaClient) Version(ctx context.Context) (string, error) {
	version, err := c.client.Version(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get ollama version: %w", err)
	}

	return version, nil
}
