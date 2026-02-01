package custom

import (
	"context"
	"iter"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/llm"
)

// ModelAdapter adapts llm.ChatClient to the agent.Model interface.
type ModelAdapter struct {
	client llm.ChatClient
}

// NewModelAdapter creates an adapter from an llm.ChatClient.
func NewModelAdapter(client llm.ChatClient) agent.Model {
	return &ModelAdapter{client: client}
}

// Provider returns the model provider name.
func (m *ModelAdapter) Provider() string {
	return m.client.Provider()
}

// Generate produces a response to the given messages.
func (m *ModelAdapter) Generate(ctx context.Context, req *agent.GenerateRequest) iter.Seq2[*agent.GenerateResponse, error] {
	// Convert agent.Message to llm.ChatMessage
	messages := make([]llm.ChatMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		chatMsg := convertToLLMMessage(msg)
		messages = append(messages, chatMsg)
	}

	// Build ChatOptions
	options := &llm.ChatOptions{
		Temperature:     &req.Temperature,
		MaxOutputTokens: &req.MaxTokens,
		TopP:            &req.TopP,
		StopSequences:   req.StopSequences,
		Instructions:    req.SystemPrompt,
	}

	if req.ModelID != "" {
		options.ModelID = req.ModelID
	}

	// Convert tools to llm.AITool
	if len(req.Tools) > 0 {
		tools := make([]llm.AITool, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, llm.NewAIFunctionTool(
				t.Name,
				t.Description,
				t.Parameters,
			))
		}
		options.Tools = tools
	}

	// Call the underlying client
	return func(yield func(*agent.GenerateResponse, error) bool) {
		for update, err := range m.client.GetStreamingResponse(ctx, messages, options) {
			if err != nil {
				yield(nil, err)
				return
			}

			// Convert llm.ChatResponseUpdate to agent.GenerateResponse
			resp := convertToGenerateResponse(update)
			if !yield(resp, nil) {
				return
			}
		}
	}
}

// convertToLLMMessage converts agent.Message to llm.ChatMessage.
func convertToLLMMessage(msg *agent.Message) llm.ChatMessage {
	if msg == nil {
		return llm.ChatMessage{}
	}

	role := llm.ChatRole(msg.Role)

	// Convert content
	contents := make([]llm.AIContent, 0, len(msg.Content))
	for _, c := range msg.Content {
		switch c.Type {
		case agent.ContentTypeText:
			contents = append(contents, llm.TextContent{Text: c.Text})
		case agent.ContentTypeToolCall:
			if c.ToolCall != nil {
				contents = append(contents, llm.FunctionCallContent{
					CallID:       c.ToolCall.ID,
					Name:         c.ToolCall.Name,
					RawArguments: c.ToolCall.Arguments,
				})
			}
		case agent.ContentTypeToolResult:
			if c.ToolResult != nil {
				contents = append(contents, llm.FunctionResultContent{
					CallID: c.ToolResult.ToolCallID,
					Result: c.ToolResult.Output,
					Error:  c.ToolResult.Error,
				})
			}
		}
	}

	return llm.ChatMessage{
		Role:     role,
		Contents: contents,
	}
}

// convertToGenerateResponse converts llm.ChatResponseUpdate to agent.GenerateResponse.
func convertToGenerateResponse(update llm.ChatResponseUpdate) *agent.GenerateResponse {
	resp := &agent.GenerateResponse{
		Content: make([]agent.Content, 0),
	}

	// Convert content
	for _, c := range update.Contents {
		switch c.ContentType() {
		case "text":
			if textContent, ok := c.(llm.TextContent); ok {
				resp.Content = append(resp.Content, agent.Content{
					Type: agent.ContentTypeText,
					Text: textContent.Text,
				})
			}
		case "functionCall":
			if fnCall, ok := c.(llm.FunctionCallContent); ok {
				resp.Content = append(resp.Content, agent.Content{
					Type: agent.ContentTypeToolCall,
					ToolCall: &agent.ToolCall{
						ID:        fnCall.CallID,
						Name:      fnCall.Name,
						Arguments: fnCall.ArgumentsJSON(),
					},
				})
			}
		}
	}

	// Convert finish reason
	if update.FinishReason != "" {
		resp.FinishReason = agent.FinishReason(update.FinishReason)
	}

	// Convert usage
	if update.Usage != nil {
		resp.Usage = &agent.Usage{
			PromptTokens:     update.Usage.InputTokens,
			CompletionTokens: update.Usage.OutputTokens,
			TotalTokens:      update.Usage.TotalTokens,
		}
	}

	resp.ModelID = update.Model

	return resp
}
