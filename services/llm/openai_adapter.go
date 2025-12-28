package llm

import (
	"context"
	"iter"

	"school-with-ai/internal/session"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAIProvider creates a new OpenAI provider with a model
// Requires OPENAI_API_KEY environment variable to be set
func NewOpenAIProvider(model string) *OpenAIProvider {
	client := openai.NewClient()
	return &OpenAIProvider{
		client: &client,
		model:  model,
	}
}

// NewOpenAIProviderWithAPIKey creates a new OpenAI provider with an explicit API key and model
func NewOpenAIProviderWithAPIKey(apiKey string, model string) *OpenAIProvider {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)
	return &OpenAIProvider{
		client: &client,
		model:  model,
	}
}

// CreateStreamingCompletion implements the Provider interface
func (p *OpenAIProvider) CreateStreamingCompletion(
	ctx context.Context,
	config CompletionConfig,
) iter.Seq2[StreamChunk, error] {
	return func(yield func(StreamChunk, error) bool) {
		// Convert session messages to OpenAI format
		params := buildOpenAIMessages(config.Messages)

		// Build stream parameters
		streamParams := openai.ChatCompletionNewParams{
			Messages: params,
			Model:    openai.ChatModel(p.model),
		}

		// Convert and add tools if provided
		if len(config.Tools) > 0 {
			openaiTools := make([]openai.ChatCompletionToolUnionParam, 0, len(config.Tools))
			for _, tool := range config.Tools {
				funcDef := openai.FunctionDefinitionParam{
					Name:        tool.Name,
					Description: openai.String(tool.Description),
				}

				// Set parameters if provided
				// tool.Parameters is map[string]any which is compatible with openai.FunctionParameters
				if tool.Parameters != nil {
					funcDef.Parameters = openai.FunctionParameters(tool.Parameters.(map[string]any))
				}

				openaiTools = append(openaiTools, openai.ChatCompletionFunctionTool(funcDef))
			}
			streamParams.Tools = openaiTools
		}

		// Create the stream
		stream := p.client.Chat.Completions.NewStreaming(ctx, streamParams)

		// Track active tool calls by index
		toolCallsByIndex := make(map[int]*ToolCallDelta)

		// Iterate over stream chunks
		for stream.Next() {
			chunk := stream.Current()

			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				streamChunk := StreamChunk{}

				// Handle text content
				if delta.Content != "" {
					streamChunk.Content = delta.Content
				}

				// Handle tool calls
				if len(delta.ToolCalls) > 0 {
					for _, tc := range delta.ToolCalls {
						index := int(tc.Index)

						// Get or create tool call state
						toolCall, exists := toolCallsByIndex[index]
						if !exists {
							toolCall = &ToolCallDelta{
								Index: index,
							}
							toolCallsByIndex[index] = toolCall
						}

						// Update fields from delta
						if tc.ID != "" {
							toolCall.ID = tc.ID
						}
						if tc.Function.Name != "" {
							toolCall.Name = tc.Function.Name
						}
						if tc.Function.Arguments != "" {
							toolCall.Arguments += tc.Function.Arguments
						}

						// Add to chunk (only include fields from the delta, not accumulated state)
						streamChunk.ToolCalls = append(streamChunk.ToolCalls, ToolCallDelta{
							Index:     index,
							ID:        tc.ID,                   // From delta, not accumulated
							Name:      tc.Function.Name,        // From delta, not accumulated
							Arguments: tc.Function.Arguments,   // From delta, not accumulated
						})
					}
				}

				// Yield the chunk
				if !yield(streamChunk, nil) {
					return
				}
			}
		}

		// Check for stream errors
		if err := stream.Err(); err != nil {
			yield(StreamChunk{}, err)
		}
	}
}

// buildOpenAIMessages converts session messages to OpenAI message format
func buildOpenAIMessages(messages []session.Message) []openai.ChatCompletionMessageParamUnion {
	params := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			params = append(params, openai.UserMessage(msg.Content))
		case "assistant":
			// Check if assistant message has tool calls
			if len(msg.ToolCalls) > 0 {
				// Convert tool calls to OpenAI format
				toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.ID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Arguments: tc.Function.Arguments,
								Name:      tc.Function.Name,
							},
						},
					})
				}

				// Build assistant message with tool calls
				var asst openai.ChatCompletionAssistantMessageParam
				if msg.Content != "" {
					asst.Content.OfString = openai.String(msg.Content)
				}
				asst.Role = "assistant"
				asst.ToolCalls = toolCalls

				params = append(params, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &asst,
				})
			} else {
				params = append(params, openai.AssistantMessage(msg.Content))
			}
		case "tool":
			// Tool result message (content first, then toolCallID)
			params = append(params, openai.ToolMessage(msg.Content, msg.ToolCallID))
		case "system":
			params = append(params, openai.SystemMessage(msg.Content))
		}
	}

	return params
}
