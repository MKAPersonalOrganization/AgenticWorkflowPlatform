package agent

import (
	"context"
	"iter"
)

// ID is a unique identifier for an agent.
type ID = string

// Identifiable represents an entity that can be identified by ID, name, and description.
type Identifiable interface {
	// ID returns the unique identifier for this agent.
	ID() ID

	// Name returns the agent's human-readable name (e.g., "math-instructor").
	Name() string

	// Description returns a human-readable description of the agent's capabilities.
	// Used by LLMs to determine whether to delegate to this agent.
	Description() string
}

// Runnable represents an entity that can execute with input and produce events.
type Runnable interface {
	// Run executes the agent with the given invocation.
	// Returns an iterator that yields events as the agent processes the request.
	Run(ctx context.Context, inv *Invocation) iter.Seq2[Event, error]
}

// Agent represents an AI agent that can process user requests and produce responses.
// This interface is designed to be compatible with multiple agent frameworks:
// - Custom agentsdk implementation
// - Google ADK-Go
// - Go-Kratos Blades
//
// Per ADR 001: This interface is the sole access point for agent interactions.
// All framework-specific details are hidden behind adapter implementations.
type Agent interface {
	Identifiable
	Runnable
}

// Toolable extends Agent with tool capabilities.
type Toolable interface {
	Agent

	// Tools returns the list of tools available to this agent.
	Tools() []Tool
}

// AgentMetadata represents metadata for an agent exposed via the API.
type AgentMetadata struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AgentListResponse represents the response containing a list of agents.
type AgentListResponse struct {
	Agents []AgentMetadata `json:"agents"`
}

// Hierarchical extends Agent with sub-agent capabilities for hierarchical delegation.
type Hierarchical interface {
	Agent

	// SubAgents returns child agents for hierarchical delegation.
	// Returns empty slice if the agent doesn't support sub-agents.
	SubAgents() []Agent

	// Parent returns the parent agent, or nil if this is a root agent.
	Parent() Agent
}

// AgentWithMiddleware extends Agent with middleware support.
// This is optional - compatible with Blades' middleware pattern.
type AgentWithMiddleware interface {
	Agent

	// AgentMiddlewares returns the middleware chain for the agent run.
	AgentMiddlewares() []Middleware

	// ChatMiddlewares returns the middleware chain for LLM calls.
	ChatMiddlewares() []Middleware

	// FunctionMiddlewares returns the middleware chain for tool execution.
	FunctionMiddlewares() []Middleware
}

// AgentProvider provides discovery and lookup for available agents.
// It replaces the previous AgentRegistry interface with a read-only contract,
// promoting direct instantiation and dependency injection over global state.
type AgentProvider interface {
	// Get retrieves an agent by name.
	Get(name string) (Agent, error)

	// List returns all available agents.
	List() []Agent
}

// Invocation contains all the information needed to execute an agent.
// This struct bridges ADK-Go's InvocationContext and Blades' Invocation.
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

	// Instruction is an optional system instruction override.
	Instruction *Message

	// Tools are additional tools available for this invocation.
	// These are merged with the agent's default tools.
	Tools []Tool

	// Context is custom context data (e.g., student profile, memory).
	Context map[string]any

	// Config contains agent execution configuration.
	Config *RunConfig

	// Resumable indicates if the invocation can be resumed after interruption.
	Resumable bool

	// Streamable indicates if the response should be streamed.
	Streamable bool
}

// Clone creates a deep copy of the Invocation.
func (inv *Invocation) Clone() *Invocation {
	if inv == nil {
		return nil
	}
	clone := *inv
	if inv.Message != nil {
		clone.Message = inv.Message.Clone()
	}
	if inv.Instruction != nil {
		clone.Instruction = inv.Instruction.Clone()
	}
	if inv.History != nil {
		clone.History = make([]*Message, len(inv.History))
		for i, m := range inv.History {
			clone.History[i] = m.Clone()
		}
	}
	if inv.Tools != nil {
		clone.Tools = make([]Tool, len(inv.Tools))
		copy(clone.Tools, inv.Tools)
	}
	if inv.Context != nil {
		clone.Context = make(map[string]any, len(inv.Context))
		for k, v := range inv.Context {
			clone.Context[k] = v
		}
	}
	return &clone
}

// RunConfig controls agent execution behavior.
type RunConfig struct {
	// Temperature controls randomness (0.0-2.0).
	Temperature float64

	// MaxTokens limits output length.
	MaxTokens int

	// MaxIterations limits tool calling loops.
	MaxIterations int

	// StreamingMode controls how events are streamed.
	StreamingMode StreamingMode

	// ModelID optionally overrides the default model.
	ModelID string

	// StopSequences are strings that stop generation when encountered.
	StopSequences []string

	// SaveInputBlobsAsArtifacts saves user input files as artifacts.
	SaveInputBlobsAsArtifacts bool
}

// DefaultRunConfig returns sensible defaults for agent execution.
func DefaultRunConfig() *RunConfig {
	return &RunConfig{
		Temperature:   0.7,
		MaxTokens:     4096,
		MaxIterations: 10,
		StreamingMode: StreamingModeSSE,
	}
}
