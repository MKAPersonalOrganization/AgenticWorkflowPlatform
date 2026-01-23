package llm

// FinishReason indicates why the model stopped generating.
type FinishReason string

const (
	// FinishReasonStop indicates the model reached a natural stopping point.
	FinishReasonStop FinishReason = "stop"

	// FinishReasonLength indicates the model hit the token limit.
	FinishReasonLength FinishReason = "length"

	// FinishReasonToolCalls indicates the model wants to call tools.
	FinishReasonToolCalls FinishReason = "tool_calls"

	// FinishReasonContentFilter indicates content was filtered.
	FinishReasonContentFilter FinishReason = "content_filter"
)

// ChatResponseUpdate represents a streaming response chunk from the model.
// Multiple updates are yielded that together form a complete response.
type ChatResponseUpdate struct {
	// Role indicates the role of the message being streamed.
	// Typically ChatRoleAssistant for model responses.
	Role ChatRole `json:"role,omitempty"`

	// Model identifies the model that generated this response.
	Model string `json:"model,omitempty"`

	// AuthorName identifies who is generating this content.
	// For agents, this is the agent name.
	AuthorName string `json:"authorName,omitempty"`

	// Contents holds the content delta for this update.
	// May contain partial text, tool call updates, or other content.
	Contents []AIContent `json:"contents,omitempty"`

	// ResponseID is a provider-assigned identifier for the response.
	ResponseID string `json:"responseId,omitempty"`

	// MessageID identifies which message this update belongs to.
	// Multiple updates with the same MessageID form a single message.
	MessageID string `json:"messageId,omitempty"`

	// FinishReason is set when streaming completes.
	// Empty string while streaming is in progress.
	FinishReason FinishReason `json:"finishReason,omitempty"`

	// ToolCallIndex indicates which tool call this update relates to.
	// Used when streaming tool call arguments incrementally.
	ToolCallIndex int `json:"toolCallIndex,omitempty"`

	// ToolCallID is the ID of the tool call being updated.
	// Set when streaming tool call content.
	ToolCallID string `json:"toolCallId,omitempty"`

	// Usage is set when token usage information is available.
	// Typically set in the final update of a stream.
	Usage *UsageDetails `json:"usage,omitempty"`
}

// Text returns the text content delta from this update.
// Returns empty string if this update contains no text.
func (u ChatResponseUpdate) Text() string {
	for _, c := range u.Contents {
		if tc, ok := c.(TextContent); ok {
			return tc.Text
		}
	}

	return ""
}

// FunctionCalls returns any function calls in this update.
func (u ChatResponseUpdate) FunctionCalls() []FunctionCallContent {
	var calls []FunctionCallContent
	for _, c := range u.Contents {
		if fc, ok := c.(FunctionCallContent); ok {
			calls = append(calls, fc)
		}
	}

	return calls
}

// HasContent returns true if this update contains any content.
func (u ChatResponseUpdate) HasContent() bool {
	return len(u.Contents) > 0
}

// IsComplete returns true if this is the final update (has finish reason).
func (u ChatResponseUpdate) IsComplete() bool {
	return u.FinishReason != ""
}

// UsageDetails tracks token usage for a request.
type UsageDetails struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`

	// Detailed breakdown
	CachedTokens             int `json:"cachedTokens,omitempty"`
	AudioInputTokens         int `json:"audioInputTokens,omitempty"`
	ReasoningTokens          int `json:"reasoningTokens,omitempty"`
	AudioOutputTokens        int `json:"audioOutputTokens,omitempty"`
	AcceptedPredictionTokens int `json:"acceptedPredictionTokens,omitempty"`
	RejectedPredictionTokens int `json:"rejectedPredictionTokens,omitempty"`
}
