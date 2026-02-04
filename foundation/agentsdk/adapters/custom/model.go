package custom

import (
	"context"
	"encoding/json"
	"iter"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/internal/llm"
)

// ModelAdapter adapts llm.LLM to the agent.Model interface.
type ModelAdapter struct {
	client   llm.LLM
	provider string
}

// NewModelAdapter creates an adapter from an llm.LLM.
func NewModelAdapter(client llm.LLM, provider string) agent.Model {
	return &ModelAdapter{client: client, provider: provider}
}

// Provider returns the model provider name.
func (m *ModelAdapter) Provider() string {
	return m.provider
}

// Generate produces a response to the given messages.
func (m *ModelAdapter) Generate(ctx context.Context, req *agent.GenerateRequest) iter.Seq2[*agent.GenerateResponse, error] {
	// Convert agent.Message to llm.Message
	messages := make([]llm.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		chatMsg := convertToLLMMessage(msg)
		messages = append(messages, chatMsg)
	}

	// Build Parameters
	parameters := llm.Parameters{
		Temperature:   &req.Temperature,
		MaxTokens:     req.MaxTokens,
		TopP:          &req.TopP,
		StopSequences: req.StopSequences,
	}

	// Convert tools to llm.ToolDeclaration
	var llmTools []llm.ToolDeclaration
	if len(req.Tools) > 0 {
		llmTools = make([]llm.ToolDeclaration, 0, len(req.Tools))
		for _, t := range req.Tools {
			var schema *llm.Schema
			if t.Parameters != nil {
				data, _ := json.Marshal(t.Parameters)
				_ = json.Unmarshal(data, &schema)
			}
			llmTools = append(llmTools, llm.ToolDeclaration{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  schema,
			})
		}
	}

	llmReq := &llm.Request{
		Messages:   messages,
		System:     req.SystemPrompt,
		Parameters: &parameters,
		Tools:      llmTools,
		Extensions: make(llm.Metadata),
	}

	llmReq.Extensions["provider"] = m.provider
	if req.ModelID != "" {
		llmReq.Extensions["model"] = req.ModelID
	}

	// Call the underlying model
	return func(yield func(*agent.GenerateResponse, error) bool) {
		for resp, err := range m.client.Generate(ctx, llmReq) {
			if err != nil {
				yield(nil, err)
				return
			}

			// Convert llm.Response to agent.GenerateResponse
			respConverted := convertToGenerateResponse(resp)
			if !yield(respConverted, nil) {
				return
			}
		}
	}
}

// convertToLLMMessage converts agent.Message to llm.Message.
func convertToLLMMessage(msg *agent.Message) llm.Message {
	if msg == nil {
		return llm.Message{}
	}

	role := llm.Role(msg.Role)

	// Convert content
	contents := make([]llm.Content, 0, len(msg.Content))
	for _, c := range msg.Content {
		switch c.Type {
		case agent.ContentTypeText:
			contents = append(contents, llm.TextContent{Text: c.Text})
		case agent.ContentTypeToolCall:
			if c.ToolCall != nil {
				contents = append(contents, llm.FunctionCall{
					ID:        c.ToolCall.ID,
					Name:      c.ToolCall.Name,
					Arguments: json.RawMessage(c.ToolCall.Arguments),
				})
			}
		case agent.ContentTypeToolResult:
			if c.ToolResult != nil {
				contents = append(contents, llm.FunctionResult{
					CallID:  c.ToolResult.ToolCallID,
					Name:    c.ToolResult.ToolName,
					Result:  json.RawMessage(c.ToolResult.Output),
					IsError: c.ToolResult.IsError || c.ToolResult.Error != "",
				})
			}
		}
	}

	return llm.Message{
		Role:    role,
		Content: contents,
	}
}

// convertToGenerateResponse converts llm.Response to agent.GenerateResponse.
func convertToGenerateResponse(resp *llm.Response) *agent.GenerateResponse {
	agentResp := &agent.GenerateResponse{
		Content: make([]agent.Content, 0),
	}

	// Convert content
	for _, c := range resp.Content {
		switch c.ContentType() {
		case "text":
			if textContent, ok := c.(llm.TextContent); ok {
				agentResp.Content = append(agentResp.Content, agent.Content{
					Type: agent.ContentTypeText,
					Text: textContent.Text,
				})
			}
		case "function_call": // Match the new ContentType identifier
			if fnCall, ok := c.(llm.FunctionCall); ok {
				agentResp.Content = append(agentResp.Content, agent.Content{
					Type: agent.ContentTypeToolCall,
					ToolCall: &agent.ToolCall{
						ID:        fnCall.ID,
						Name:      fnCall.Name,
						Arguments: string(fnCall.Arguments),
					},
				})
			}
		}
	}

	// Convert finish reason
	if resp.FinishReason != "" {
		agentResp.FinishReason = agent.FinishReason(resp.FinishReason)
	}

	// Convert usage
	if resp.Usage != nil {
		agentResp.Usage = &agent.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	agentResp.ModelID = resp.Extensions.GetString("model")

	return agentResp
}
