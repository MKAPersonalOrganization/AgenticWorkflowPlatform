package llm

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

// AITool represents a tool/function that can be called by the model.
// This mirrors Microsoft.Extensions.AI.AITool from the .NET agent framework.
type AITool struct {
	// Name is the unique identifier for this tool.
	Name string `json:"name"`

	// Description explains what the tool does.
	// This helps the model decide when to use the tool.
	Description string `json:"description"`

	// Parameters is the JSON Schema describing the tool's parameters.
	// Typically includes "type": "object", "properties", and "required".
	Parameters map[string]any `json:"parameters,omitempty"`
}

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

	// AdditionalProperties holds provider-specific options.
	AdditionalProperties map[string]any `json:"additionalProperties,omitempty"`
}

// Clone creates a deep copy of ChatOptions.
func (o ChatOptions) Clone() ChatOptions {
	clone := o

	// Deep copy slices
	if o.StopSequences != nil {
		clone.StopSequences = make([]string, len(o.StopSequences))
		copy(clone.StopSequences, o.StopSequences)
	}

	if o.Tools != nil {
		clone.Tools = make([]AITool, len(o.Tools))
		for i, t := range o.Tools {
			clone.Tools[i] = t.Clone()
		}
	}

	// Deep copy pointers
	if o.Temperature != nil {
		v := *o.Temperature
		clone.Temperature = &v
	}
	if o.MaxOutputTokens != nil {
		v := *o.MaxOutputTokens
		clone.MaxOutputTokens = &v
	}
	if o.TopP != nil {
		v := *o.TopP
		clone.TopP = &v
	}
	if o.FrequencyPenalty != nil {
		v := *o.FrequencyPenalty
		clone.FrequencyPenalty = &v
	}
	if o.PresencePenalty != nil {
		v := *o.PresencePenalty
		clone.PresencePenalty = &v
	}
	if o.Seed != nil {
		v := *o.Seed
		clone.Seed = &v
	}

	// Deep copy map
	if o.AdditionalProperties != nil {
		clone.AdditionalProperties = make(map[string]any, len(o.AdditionalProperties))
		for k, v := range o.AdditionalProperties {
			clone.AdditionalProperties[k] = v
		}
	}

	return clone
}

// Clone creates a deep copy of AITool.
func (t AITool) Clone() AITool {
	clone := t

	if t.Parameters != nil {
		clone.Parameters = make(map[string]any, len(t.Parameters))
		for k, v := range t.Parameters {
			clone.Parameters[k] = v
		}
	}

	return clone
}

// WithTemperature sets the temperature and returns the options for chaining.
func (o ChatOptions) WithTemperature(temp float64) ChatOptions {
	o.Temperature = &temp
	return o
}

// WithMaxTokens sets the max output tokens and returns the options for chaining.
func (o ChatOptions) WithMaxTokens(tokens int) ChatOptions {
	o.MaxOutputTokens = &tokens
	return o
}

// WithInstructions sets the system instructions and returns the options for chaining.
func (o ChatOptions) WithInstructions(instructions string) ChatOptions {
	o.Instructions = instructions
	return o
}

// WithTools sets the available tools and returns the options for chaining.
func (o ChatOptions) WithTools(tools []AITool) ChatOptions {
	o.Tools = tools
	return o
}
