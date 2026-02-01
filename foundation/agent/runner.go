package agent

import (
	"context"
	"iter"
)

// Runner orchestrates agent execution across sessions.
// This is the main entry point for running agents in the application.
type Runner interface {
	// Run executes an agent with the given request.
	// Returns an iterator that yields events as they occur.
	Run(context.Context, *RunRequest) iter.Seq2[Event, error]

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
	SessionID string

	// UserID identifies the user making the request.
	UserID string

	// Message is the user's input.
	Message *Message

	// Config optionally overrides agent execution settings.
	Config *RunConfig

	// Context provides additional data (e.g., student profile).
	Context map[string]any
}
