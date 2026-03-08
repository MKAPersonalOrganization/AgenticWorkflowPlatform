package agent

import (
	"context"
	"iter"
)

// Runner orchestrates agent execution across sessions.
// This is the main entry point for running agents in the application.
//
// Per ADR 002: The Runner does NOT emit lifecycle events (RunStarted, RunFinished).
// Those are the responsibility of the orchestration service layer.
// The Runner only translates framework events to domain events.
type Runner interface {
	// Run executes an agent with the given request.
	// Returns an iterator that yields events as they occur.
	//
	// The runner handles:
	// - Agent lookup by name
	// - Session management
	// - Event translation from framework to domain
	// - Tool execution orchestration
	Run(context.Context, *RunRequest) iter.Seq2[Event, error]

	// RunSync executes an agent and returns the final result.
	// This is a convenience method for non-streaming use cases.
	RunSync(context.Context, *RunRequest) (*RunResult, error)

	// GetAgent returns an agent by name.
	GetAgent(name string) (Agent, error)

	// ListAgents returns all available agent names.
	ListAgents() []string
}

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

	// Context provides additional data (e.g., student profile).
	Context map[string]any

	// Resumable indicates if the invocation can be resumed.
	Resumable bool

	// InvocationID allows resuming a specific invocation.
	// If empty, a new invocation ID is generated.
	InvocationID string
}

// RunResult contains the final result of a synchronous run.
type RunResult struct {
	// Message is the final assistant message.
	Message *Message

	// Usage contains token consumption for the run.
	Usage *Usage

	// Events contains all events that occurred during the run.
	Events []Event
}

// RunnerConfig contains configuration for creating a Runner.
type RunnerConfig struct {
	// AppName identifies the application.
	AppName string

	// RootAgent is the primary agent for handling requests.
	RootAgent Agent

	// ArtifactStore manages artifact storage (optional).
	ArtifactStore ArtifactStore

	// Middlewares are applied to all agent invocations.
	Middlewares []Middleware

	// ModelConfig specifies which LLM to use.
	// Each framework adapter is responsible for creating its own LLM client.
	ModelConfig *ModelConfig
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

// ArtifactStore persists artifacts.
type ArtifactStore interface {
	// Save stores an artifact.
	Save(ctx context.Context, sessionID, name string, data []byte, mimeType string) error

	// Load retrieves an artifact.
	Load(ctx context.Context, sessionID, name string) ([]byte, string, error)

	// List returns artifact names for a session.
	List(ctx context.Context, sessionID string) ([]string, error)
}
