package agent

import (
	"context"
	"iter"
)

// Agent represents an AI agent that can process user requests and produce responses.
// This interface is designed to be compatible with multiple agent frameworks:
// - Custom agentsdk implementation
// - Google ADK-Go
// - Go-Kratos Blades
type Agent interface {
	// Name returns the agent's identifier (e.g., "math-instructor").
	Name() string

	// Description returns a human-readable description of the agent's capabilities.
	Description() string

	// Run executes the agent with the given invocation context.
	// Returns an iterator that yields events as the agent processes the request.
	Run(context.Context, *Invocation) iter.Seq2[Event, error]

	// Tools returns the list of tools available to this agent.
	Tools() []Tool

	// SubAgents returns child agents for hierarchical delegation (optional).
	// Returns empty slice if the agent doesn't support sub-agents.
	SubAgents() []Agent
}

// AgentRegistry provides a list of available agents.
type AgentRegistry interface {
	List() []string
}

// Invocation contains all the information needed to execute an agent.
type Invocation struct {
	// ID is a unique identifier for this invocation (for resumability).
	ID string

	// SessionID identifies the conversation session.
	SessionID string

	// UserID identifies the user making the request.
	UserID string

	// Message is the user's input message.
	Message *Message

	// History contains previous messages in the conversation.
	History []*Message

	// Context is custom context data (e.g., student profile, memory).
	Context map[string]any

	// Config contains agent execution configuration (temperature, max tokens, etc.).
	Config *RunConfig
}

// RunConfig controls agent execution behavior.
type RunConfig struct {
	// Temperature controls randomness (0.0-2.0).
	Temperature float64

	// MaxTokens limits output length.
	MaxTokens int

	// MaxIterations limits tool calling loops.
	MaxIterations int

	// Streaming enables/disables event streaming.
	Streaming bool

	// ModelID optionally overrides the default model.
	ModelID string

	// StopSequences are strings that stop generation when encountered.
	StopSequences []string
}

// DefaultRunConfig returns sensible defaults for agent execution.
func DefaultRunConfig() *RunConfig {
	return &RunConfig{
		Temperature:   0.7,
		MaxTokens:     4096,
		MaxIterations: 10,
		Streaming:     true,
	}
}
