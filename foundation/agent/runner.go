package agent

// RunRequest contains all parameters for a runner invocation.
type RunRequest struct {
	// AgentName identifies which agent to run.
	AgentName string

	// SessionID identifies the conversation session.
	// If empty, a new session will be created.
	SessionID string

	// UserID identifies the user making the request.
	UserID string

	// Message is the user's input.
	Message *Message

	// Config optionally overrides agent execution settings.
	Config *RunConfig

	// Context provides additional data for the invocation.
	Context map[string]any

	// Resumable indicates if the invocation can be resumed.
	Resumable bool

	// InvocationID allows resuming a specific invocation.
	// If empty, a new invocation ID is generated.
	InvocationID string
}

// ModelConfig specifies LLM provider, credentials, and global execution parameters.
// Framework adapters use this to create their internal LLM clients.
type ModelConfig struct {
	// Provider identifies the LLM provider ("openai", "gemini", "anthropic", etc.).
	Provider string

	// Model specifies the model name ("gpt-4", "gemini-pro", etc.).
	Model string

	// APIKey is the authentication key for the provider.
	APIKey string

	// Temperature controls randomness (0.0–2.0). nil = provider default.
	Temperature *float64

	// MaxIterations limits the number of tool-calling loops per request.
	// 0 = engine default.
	MaxIterations int

	// MaxTokens limits the number of tokens the model can generate per response.
	// nil = provider default.
	MaxTokens *int
}
