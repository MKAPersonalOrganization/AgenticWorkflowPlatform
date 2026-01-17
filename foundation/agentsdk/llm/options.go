package llm

import "maps"

// ToolMode controls how the model uses tools/functions.
// This mirrors Microsoft.Extensions.AI.ChatToolMode from the .NET agent framework.
type ToolMode string

const (
	// ToolModeAuto lets the model decide when to use tools.
	ToolModeAuto ToolMode = "auto"

	// ToolModeRequired forces the model to use at least one tool.
	ToolModeRequired ToolMode = "required"

	// ToolModeNone prevents the model from using any tools.
	ToolModeNone ToolMode = "none"
)

// ChatOptions contains configuration for a chat completion request.
// This mirrors Microsoft.Extensions.AI.ChatOptions from the .NET agent framework.
type ChatOptions struct {
	// ModelID optionally overrides the default model.
	ModelID string `json:"modelId,omitempty"`

	// Temperature controls randomness in the output (0.0-2.0).
	// Lower values make output more deterministic.
	Temperature *float64 `json:"temperature,omitempty"`

	// MaxOutputTokens limits the maximum number of tokens to generate.
	MaxOutputTokens *int `json:"maxOutputTokens,omitempty"`

	// TopP controls nucleus sampling (0.0-1.0).
	// Alternative to temperature for controlling randomness.
	TopP *float64 `json:"topP,omitempty"`

	// FrequencyPenalty penalizes tokens based on their frequency (-2.0 to 2.0).
	FrequencyPenalty *float64 `json:"frequencyPenalty,omitempty"`

	// PresencePenalty penalizes tokens based on their presence (-2.0 to 2.0).
	PresencePenalty *float64 `json:"presencePenalty,omitempty"`

	// StopSequences are strings that stop generation when encountered.
	StopSequences []string `json:"stopSequences,omitempty"`

	// Instructions is the system prompt/instructions for the model.
	// Prepended to the conversation as a system message.
	Instructions string `json:"instructions,omitempty"`

	// Tools is the list of tools available to the model.
	Tools []AITool `json:"tools,omitempty"`

	// ToolMode controls how the model uses tools.
	ToolMode ToolMode `json:"toolMode,omitempty"`

	// ConversationID is an optional identifier for the conversation.
	// Useful for logging and tracking.
	ConversationID string `json:"conversationId,omitempty"`

	// Seed enables reproducible outputs when set.
	Seed *int64 `json:"seed,omitempty"`

	// ParallelToolCalls enables/disables parallel tool execution if supported by the provider.
	ParallelToolCalls *bool `json:"parallelToolCalls,omitempty"`

	// AdditionalProperties holds provider-specific options.
	AdditionalProperties map[string]any `json:"additionalProperties,omitempty"`
}

// Clone creates a deep copy of ChatOptions.
func (o ChatOptions) Clone() ChatOptions {
	if o.StopSequences != nil {
		seq := make([]string, len(o.StopSequences))
		copy(seq, o.StopSequences)
		o.StopSequences = seq
	}

	if o.Tools != nil {
		o.Tools = CloneTools(o.Tools)
	}

	if o.AdditionalProperties != nil {
		props := make(map[string]any, len(o.AdditionalProperties))
		maps.Copy(props, o.AdditionalProperties)
		o.AdditionalProperties = props
	}

	return o
}
