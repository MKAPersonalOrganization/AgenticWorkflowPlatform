package llm

import (
	"context"
	"iter"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// OpenAIClient implements the ChatClient interface for OpenAI.
// It provides streaming chat completions using the OpenAI API.
type OpenAIClient struct {
	client *openai.Client
	model  string
}

// NewOpenAIClient creates a new OpenAI client with the specified model.
// Requires OPENAI_API_KEY environment variable to be set.
func NewOpenAIClient(model string) *OpenAIClient {
	client := openai.NewClient()
	return &OpenAIClient{
		client: &client,
		model:  model,
	}
}

// NewOpenAIClientWithAPIKey creates a new OpenAI client with an explicit API key and model.
func NewOpenAIClientWithAPIKey(apiKey string, model string) *OpenAIClient {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)
	return &OpenAIClient{
		client: &client,
		model:  model,
	}
}

// GetStreamingResponse implements the ChatClient interface.
// It sends messages to OpenAI and streams back response updates.
func (c *OpenAIClient) GetStreamingResponse(
	ctx context.Context,
	messages []ChatMessage,
	options *ChatOptions,
) iter.Seq2[ChatResponseUpdate, error] {
	return func(yield func(ChatResponseUpdate, error) bool) {
		// Build OpenAI message parameters
		params := c.buildMessages(messages, options)

		// Build stream parameters
		streamParams := openai.ChatCompletionNewParams{
			Messages: params,
			Model:    c.getModel(options),
		}

		// Apply options
		c.applyOptions(&streamParams, options)

		// Create the stream
		stream := c.client.Chat.Completions.NewStreaming(ctx, streamParams)

		var responseID string
		var finishReason FinishReason

		// Iterate over stream chunks
		for stream.Next() {
			chunk := stream.Current()

			// Capture response ID from first chunk
			if responseID == "" && chunk.ID != "" {
				responseID = chunk.ID
			}

			if len(chunk.Choices) > 0 {
				choice := chunk.Choices[0]
				delta := choice.Delta

				// Check for finish reason
				if choice.FinishReason != "" {
					finishReason = mapFinishReason(string(choice.FinishReason))
				}

				// Handle text content
				if delta.Content != "" {
					update := ChatResponseUpdate{
						Role:       ChatRoleAssistant,
						ResponseID: responseID,
						Contents:   []AIContent{TextContent{Text: delta.Content}},
					}
					if !yield(update, nil) {
						return
					}
				}

				// Handle tool calls
				for _, tc := range delta.ToolCalls {
					update := c.buildToolCallUpdate(tc, responseID)
					if !yield(update, nil) {
						return
					}
				}
			}
		}

		// Check for stream errors
		if err := stream.Err(); err != nil {
			yield(ChatResponseUpdate{}, err)
			return
		}

		// Yield final update with finish reason
		if finishReason != "" {
			finalUpdate := ChatResponseUpdate{
				Role:         ChatRoleAssistant,
				ResponseID:   responseID,
				FinishReason: finishReason,
			}
			yield(finalUpdate, nil)
		}
	}
}

// buildMessages converts ChatMessage slice to OpenAI message format.
func (c *OpenAIClient) buildMessages(messages []ChatMessage, options *ChatOptions) []openai.ChatCompletionMessageParamUnion {
	params := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages)+1)

	// Prepend system message if instructions are provided
	if options != nil && options.Instructions != "" {
		params = append(params, openai.SystemMessage(options.Instructions))
	}

	// Convert each message
	for _, msg := range messages {
		switch msg.Role {
		case ChatRoleUser:
			params = append(params, openai.UserMessage(msg.Text()))

		case ChatRoleAssistant:
			params = append(params, c.buildAssistantMessage(msg))

		case ChatRoleTool:
			for _, fr := range msg.FunctionResults() {
				content := fr.Result
				if fr.IsError() {
					content = fr.Error
				}
				params = append(params, openai.ToolMessage(content, fr.CallID))
			}

		case ChatRoleSystem:
			params = append(params, openai.SystemMessage(msg.Text()))
		}
	}

	return params
}

// buildAssistantMessage converts an assistant ChatMessage to OpenAI format.
func (c *OpenAIClient) buildAssistantMessage(msg ChatMessage) openai.ChatCompletionMessageParamUnion {
	functionCalls := msg.FunctionCalls()

	if len(functionCalls) > 0 {
		// Build tool calls
		toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(functionCalls))
		for _, fc := range functionCalls {
			toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
					ID: fc.CallID,
					Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Arguments: fc.ArgumentsJSON(),
						Name:      fc.Name,
					},
				},
			})
		}

		// Build assistant message with tool calls
		var asst openai.ChatCompletionAssistantMessageParam
		text := msg.Text()
		if text != "" {
			asst.Content.OfString = openai.String(text)
		}
		asst.Role = "assistant"
		asst.ToolCalls = toolCalls

		return openai.ChatCompletionMessageParamUnion{
			OfAssistant: &asst,
		}
	}

	return openai.AssistantMessage(msg.Text())
}

// buildToolCallUpdate creates a ChatResponseUpdate from an OpenAI tool call delta.
func (c *OpenAIClient) buildToolCallUpdate(tc openai.ChatCompletionChunkChoiceDeltaToolCall, responseID string) ChatResponseUpdate {
	update := ChatResponseUpdate{
		Role:          ChatRoleAssistant,
		ResponseID:    responseID,
		ToolCallIndex: int(tc.Index),
		ToolCallID:    tc.ID,
	}

	// Add function call content if we have name or arguments
	if tc.ID != "" || tc.Function.Name != "" || tc.Function.Arguments != "" {
		fc := FunctionCallContent{
			CallID:       tc.ID,
			Name:         tc.Function.Name,
			RawArguments: tc.Function.Arguments,
		}
		update.Contents = []AIContent{fc}
	}

	return update
}

// applyOptions applies ChatOptions to OpenAI stream parameters.
func (c *OpenAIClient) applyOptions(params *openai.ChatCompletionNewParams, options *ChatOptions) {
	if options == nil {
		return
	}

	// Apply temperature
	if options.Temperature != nil {
		params.Temperature = openai.Float(*options.Temperature)
	}

	// Apply max tokens
	if options.MaxOutputTokens != nil {
		params.MaxTokens = openai.Int(int64(*options.MaxOutputTokens))
	}

	// Apply top_p
	if options.TopP != nil {
		params.TopP = openai.Float(*options.TopP)
	}

	// Apply frequency penalty
	if options.FrequencyPenalty != nil {
		params.FrequencyPenalty = openai.Float(*options.FrequencyPenalty)
	}

	// Apply presence penalty
	if options.PresencePenalty != nil {
		params.PresencePenalty = openai.Float(*options.PresencePenalty)
	}

	// Apply stop sequences
	if len(options.StopSequences) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{
			OfStringArray: options.StopSequences,
		}
	}

	// Apply seed
	if options.Seed != nil {
		params.Seed = openai.Int(*options.Seed)
	}

	// Apply tools
	if len(options.Tools) > 0 {
		openaiTools := make([]openai.ChatCompletionToolUnionParam, 0, len(options.Tools))
		for _, tool := range options.Tools {
			funcDef := openai.FunctionDefinitionParam{
				Name:        tool.Name(),
				Description: openai.String(tool.Description()),
			}

			if params := tool.Parameters(); params != nil {
				funcDef.Parameters = openai.FunctionParameters(params)
			}

			openaiTools = append(openaiTools, openai.ChatCompletionFunctionTool(funcDef))
		}
		params.Tools = openaiTools

		// Apply tool mode
		switch options.ToolMode {
		case ToolModeNone:
			params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
				OfAuto: openai.String("none"),
			}
		case ToolModeRequired:
			params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
				OfAuto: openai.String("required"),
			}
			// ToolModeAuto is the default, no need to set
		}
	}
}

// getModel returns the model to use, preferring options.ModelID if set.
func (c *OpenAIClient) getModel(options *ChatOptions) openai.ChatModel {
	if options != nil && options.ModelID != "" {
		return openai.ChatModel(options.ModelID)
	}
	return openai.ChatModel(c.model)
}

// mapFinishReason converts OpenAI finish reason to our FinishReason type.
func mapFinishReason(reason string) FinishReason {
	switch reason {
	case "stop":
		return FinishReasonStop
	case "length":
		return FinishReasonLength
	case "tool_calls":
		return FinishReasonToolCalls
	case "content_filter":
		return FinishReasonContentFilter
	default:
		return FinishReason(reason)
	}
}
