package llm

import (
	"context"
	"encoding/json"
	"iter"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
)

// OpenAIClient implements the LLM interface for OpenAI.
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

// Generate implements the LLM interface.
// It sends messages to OpenAI and streams back response updates.
func (c *OpenAIClient) Generate(
	ctx context.Context,
	req *Request,
) iter.Seq2[*Response, error] {
	return func(yield func(*Response, error) bool) {
		// Resolve model first so buildMessages can adapt content to model capabilities
		model := c.getModel(req.Extensions)

		// Build OpenAI message parameters
		params := c.buildMessages(req.Messages, req.System, model)

		// Build stream parameters
		streamParams := openai.ChatCompletionNewParams{
			Messages: params,
			Model:    model,
		}

		// Apply options
		c.applyOptions(&streamParams, req)

		// Create the stream
		stream := c.client.Chat.Completions.NewStreaming(ctx, streamParams)

		var responseID string
		var modelUsed string
		var finishReason FinishReason

		// Iterate over stream chunks
		for stream.Next() {
			chunk := stream.Current()

			// Capture response ID and model from first chunk
			if responseID == "" && chunk.ID != "" {
				responseID = chunk.ID
			}
			if modelUsed == "" && chunk.Model != "" {
				modelUsed = chunk.Model
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
					resp := &Response{
						Content: []Content{TextContent{Text: delta.Content}},
						Partial: true,
						Extensions: Metadata{
							"response_id": responseID,
							"model":       modelUsed,
							"provider":    "openai",
						},
					}
					if !yield(resp, nil) {
						return
					}
				}

				// Handle tool calls
				for _, tc := range delta.ToolCalls {
					resp := c.buildToolCallResponse(tc, responseID, modelUsed)
					if !yield(resp, nil) {
						return
					}
				}
			}

			// Capture usage information
			if chunk.Usage.TotalTokens > 0 {
				usage := &Usage{
					InputTokens:  int(chunk.Usage.PromptTokens),
					OutputTokens: int(chunk.Usage.CompletionTokens),
					TotalTokens:  int(chunk.Usage.TotalTokens),
				}

				// Capture detailed prompt token breakdown
				usage.CachedTokens = int(chunk.Usage.PromptTokensDetails.CachedTokens)
				usage.AudioInputTokens = int(chunk.Usage.PromptTokensDetails.AudioTokens)

				// Capture detailed completion token breakdown
				usage.ReasoningTokens = int(chunk.Usage.CompletionTokensDetails.ReasoningTokens)
				usage.AudioOutputTokens = int(chunk.Usage.CompletionTokensDetails.AudioTokens)
				usage.AcceptedPredictionTokens = int(chunk.Usage.CompletionTokensDetails.AcceptedPredictionTokens)
				usage.RejectedPredictionTokens = int(chunk.Usage.CompletionTokensDetails.RejectedPredictionTokens)

				resp := &Response{
					Usage:   usage,
					Partial: true,
					Extensions: Metadata{
						"response_id": responseID,
						"model":       modelUsed,
					},
				}
				if !yield(resp, nil) {
					return
				}
			}
		}

		// Check for stream errors
		err := stream.Err()
		if err != nil {
			yield(nil, c.mapError("openai.Generate", err))

			return
		}

		// Yield final update with finish reason
		if finishReason != "" {
			finalResp := &Response{
				FinishReason: finishReason,
				Partial:      false,
				Extensions: Metadata{
					"response_id": responseID,
					"model":       modelUsed,
				},
			}
			yield(finalResp, nil)
		}
	}
}

// buildMessages converts Message slice to OpenAI message format.
func (c *OpenAIClient) buildMessages(messages []Message, system string, model openai.ChatModel) []openai.ChatCompletionMessageParamUnion {
	params := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages)+1)

	supportsAudio := modelSupportsAudioInput(string(model))
	supportsFiles := modelSupportsFileInput(string(model))

	// Prepend system message if instructions are provided
	if system != "" {
		params = append(params, openai.SystemMessage(system))
	}

	// Convert each message
	for _, msg := range messages {
		switch msg.Role {
		case RoleUser:
			// Check if message has multimodal content (images, audio, files)
			if msg.HasMultimodal() {
				parts := []openai.ChatCompletionContentPartUnionParam{}
				// Add text content if present
				text := msg.Text()
				if text != "" {
					parts = append(parts, openai.TextContentPart(text))
				}
				// Add image content parts
				for _, img := range msg.ImageURLs() {
					detail := img.Detail
					if detail == "" {
						detail = "auto"
					}
					parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
						URL:    img.URL,
						Detail: detail,
					}))
				}
				// Add audio content parts
				for _, audio := range msg.AudioContents() {
					if supportsAudio {
						parts = append(parts, openai.InputAudioContentPart(openai.ChatCompletionContentPartInputAudioInputAudioParam{
							Data:   audio.Data,
							Format: audio.Format,
						}))
					} else {
						// Model doesn't support input_audio — add a text note so the LLM
						// is aware an audio attachment was provided.
						parts = append(parts, openai.TextContentPart("[Audio attachment: "+audio.Format+" format — this model does not support inline audio input]"))
					}
				}
				// Add file content parts
				for _, file := range msg.FileContents() {
					if supportsFiles {
						// OpenAI expects a full data URL: data:<mime>;base64,<data>
						fileDataURL := "data:" + file.MimeType + ";base64," + file.Data
						parts = append(parts, openai.FileContentPart(openai.ChatCompletionContentPartFileFileParam{
							FileData: param.NewOpt(fileDataURL),
							Filename: param.NewOpt(file.FileName),
						}))
					} else {
						// Model doesn't support inline file content — add a text note.
						parts = append(parts, openai.TextContentPart("[File attachment: "+file.FileName+" ("+file.MimeType+") — this model does not support inline file input]"))
					}
				}
				params = append(params, openai.UserMessage(parts))
			} else {
				params = append(params, openai.UserMessage(msg.Text()))
			}

		case RoleAssistant:
			params = append(params, c.buildAssistantMessage(msg))

		case RoleTool:
			for _, fr := range msg.FunctionResults() {
				params = append(params, openai.ToolMessage(string(fr.Result), fr.CallID))
			}

		case RoleSystem:
			params = append(params, openai.SystemMessage(msg.Text()))
		}
	}

	return params
}

// buildAssistantMessage converts an assistant Message to OpenAI format.
func (c *OpenAIClient) buildAssistantMessage(msg Message) openai.ChatCompletionMessageParamUnion {
	functionCalls := msg.FunctionCalls()

	if len(functionCalls) > 0 {
		// Build tool calls
		toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(functionCalls))
		for _, fc := range functionCalls {
			toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
					ID: fc.ID,
					Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Arguments: string(fc.Arguments),
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

// buildToolCallResponse creates a Response from an OpenAI tool call delta.
func (c *OpenAIClient) buildToolCallResponse(tc openai.ChatCompletionChunkChoiceDeltaToolCall, responseID string, model string) *Response {
	resp := &Response{
		Partial: true,
		Extensions: Metadata{
			"response_id": responseID,
			"model":       model,
			"index":       tc.Index,
			"provider":    "openai",
		},
	}

	// Add function call content if we have name or arguments
	if tc.ID != "" || tc.Function.Name != "" || tc.Function.Arguments != "" {
		fc := FunctionCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		}
		resp.Content = []Content{fc}
	}

	return resp
}

// applyOptions applies Parameters and Tools to OpenAI stream parameters.
func (c *OpenAIClient) applyOptions(params *openai.ChatCompletionNewParams, req *Request) {
	// Always include usage in streams
	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{
		IncludeUsage: openai.Bool(true),
	}

	if req.Parameters != nil {
		opts := req.Parameters
		// Apply temperature
		if opts.Temperature != nil {
			params.Temperature = openai.Float(*opts.Temperature)
		}

		// Apply max tokens
		if opts.MaxTokens > 0 {
			params.MaxTokens = openai.Int(int64(opts.MaxTokens))
		}

		// Apply top_p
		if opts.TopP != nil {
			params.TopP = openai.Float(*opts.TopP)
		}

		// Apply frequency penalty
		if opts.FrequencyPenalty != nil {
			params.FrequencyPenalty = openai.Float(*opts.FrequencyPenalty)
		}

		// Apply presence penalty
		if opts.PresencePenalty != nil {
			params.PresencePenalty = openai.Float(*opts.PresencePenalty)
		}

		// Apply stop sequences
		if len(opts.StopSequences) > 0 {
			params.Stop = openai.ChatCompletionNewParamsStopUnion{
				OfStringArray: opts.StopSequences,
			}
		}
	}

	// Apply extensions (higher priority)
	if req.Extensions != nil {
		if seed, ok := req.Extensions["seed"].(int64); ok {
			params.Seed = openai.Int(seed)
		}
	}

	// Apply tools
	if len(req.Tools) > 0 {
		parallel := false
		if req.Extensions != nil {
			if extParallel, ok := req.Extensions["parallel_tool_calls"].(bool); ok {
				parallel = extParallel
			}
		}
		params.ParallelToolCalls = openai.Bool(parallel)
		openaiTools := make([]openai.ChatCompletionToolUnionParam, 0, len(req.Tools))
		for _, tool := range req.Tools {
			funcDef := openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
			}

			if tool.Parameters != nil {
				// Convert Schema to map[string]any for OpenAI
				data, _ := json.Marshal(tool.Parameters)
				var paramsMap map[string]any
				_ = json.Unmarshal(data, &paramsMap)

				// Ensure 'properties' is present for object types, as OpenAI requires it.
				if typ, ok := paramsMap["type"].(string); ok && typ == "object" {
					if _, hasProps := paramsMap["properties"]; !hasProps {
						paramsMap["properties"] = map[string]any{}
					}
				}

				funcDef.Parameters = openai.FunctionParameters(paramsMap)
			}

			openaiTools = append(openaiTools, openai.ChatCompletionFunctionTool(funcDef))
		}
		params.Tools = openaiTools
	}

	if req.Extensions != nil {
		if toolChoice, ok := req.Extensions["tool_choice"].(string); ok {
			switch toolChoice {
			case "none", "auto", "required":
				params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
					OfAuto: openai.String(toolChoice),
				}
			default:
				params.ToolChoice = openai.ToolChoiceOptionFunctionToolChoice(
					openai.ChatCompletionNamedToolChoiceFunctionParam{
						Name: toolChoice,
					},
				)
			}
		}
	}
}

// getModel returns the model to use, preferring extensions["model"] if set.
func (c *OpenAIClient) getModel(extensions Metadata) openai.ChatModel {
	if extensions != nil {
		if modelID, ok := extensions["model"].(string); ok {
			return openai.ChatModel(modelID)
		}
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
		return FinishReasonToolUse
	case "content_filter":
		return FinishReasonContentFilter
	default:
		return FinishReason(reason)
	}
}

func (c *OpenAIClient) mapError(op string, err error) error {
	if err == nil {
		return nil
	}

	kind := KindInternal
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "429") || strings.Contains(errStr, "rate_limit"):
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

// modelSupportsAudioInput returns true if the OpenAI model accepts input_audio
// content parts.  Currently only gpt-4o-audio-* models support this.
func modelSupportsAudioInput(model string) bool {
	return strings.Contains(model, "audio")
}

// modelSupportsFileInput returns true if the OpenAI model accepts inline file
// content parts (the "file" content type with base64 file_data).
// Supported by gpt-4o family, gpt-4.1 family, o1, o3, o4-mini and newer.
// NOT supported by gpt-4-turbo, gpt-4, gpt-3.5-turbo, o1-mini, o3-mini.
func modelSupportsFileInput(model string) bool {
	switch {
	case strings.HasPrefix(model, "gpt-4o"):
		return true
	case strings.HasPrefix(model, "gpt-4.1"):
		return true
	case strings.HasPrefix(model, "gpt-4.5"):
		return true
	case model == "o1" || strings.HasPrefix(model, "o1-2"):
		// o1 (full) supports it, but o1-mini and o1-preview do not
		return true
	case strings.HasPrefix(model, "o3") && !strings.Contains(model, "mini"):
		return true
	case strings.HasPrefix(model, "o4-mini"):
		return true
	default:
		return false
	}
}
