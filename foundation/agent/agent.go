package agent

// ID is a unique identifier for an agent.
type ID = string

// Agent represents an AI agent definition — its identity and capability tree.
//
// Execution is delegated entirely to the framework adapter.
// Agents form a tree: each agent can have SubAgents for delegation/transfer.
type Agent interface {
	// ID returns the unique identifier for this agent.
	ID() ID

	// Name returns the agent's human-readable name (e.g., "math-expert").
	Name() string

	// Description returns a human-readable description of the agent's capabilities.
	Description() string

	// SubAgents returns child agents available for delegation.
	// Returns nil for leaf agents.
	SubAgents() []Agent
}

// Configurable is an optional interface for agents that expose their configuration.
// The adapter layer uses this to extract agent configuration without importing
// concrete agent types.
type Configurable interface {
	Agent
	Config() Config
}

// WorkflowAgent is an optional interface for agents that represent a
// multi-agent workflow composed as an arbitrary tree of nodes.
// Each node can be a leaf (referencing an AgentDefinition) or a composite
// (sequential, parallel, loop) containing child nodes — enabling arbitrary
// nesting like: Sequential → [Agent, Parallel → [Agent, Loop → [Agent, Agent]]].
//
// The adapter layer uses this to recursively build the appropriate
// workflow agent tree at runtime.
//
// See arch/adr/019-multi-agent-workflow-composition.md.
type WorkflowAgent interface {
	Agent

	// WorkflowGraph returns the root node of the composition tree.
	WorkflowGraph() WorkflowNodeInfo
}

// WorkflowNodeInfo describes a single node in a multi-agent workflow tree.
// Composite nodes (sequential, parallel, loop) contain children.
// Leaf nodes (agent) reference an AgentDefinition by ID.
// Remote nodes (remote) delegate to an external A2A connector.
type WorkflowNodeInfo struct {
	// ID is a unique identifier within this tree.
	ID string

	// Type is "agent", "sequential", "parallel", "loop", or "remote".
	Type string

	// Label is an optional display name for this node.
	Label string

	// AgentID is the ID of the AgentDefinition (only for Type="agent").
	AgentID string

	// A2AConnectorID references an A2A connector (only for Type="remote").
	// The connector is looked up at runtime and delegated to via the A2A protocol.
	A2AConnectorID string

	// OutputKey optionally stores this node's output in session state.
	OutputKey string

	// MaxIterations limits loop repetitions (only for Type="loop").
	MaxIterations int

	// AllowEarlyExit controls whether the exit_loop tool is injected into the
	// last child agent of a loop node. When false (the default) the loop runs
	// exactly MaxIterations times. Set to true to let the LLM break out early.
	AllowEarlyExit bool

	// DisallowTransferToParent overrides the agent's transfer policy
	// within this workflow context.
	DisallowTransferToParent *bool

	// DisallowTransferToPeers overrides the agent's transfer policy
	// within this workflow context.
	DisallowTransferToPeers *bool

	// IncludeContents overrides content forwarding for this node.
	// "default" or "none".
	IncludeContents string

	// Children are the sub-nodes (only for composite types).
	Children []WorkflowNodeInfo
}

// AgentMetadata represents metadata for an agent exposed via the API.
type AgentMetadata struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// AgentListResponse represents the response containing a list of agents.
type AgentListResponse struct {
	Agents []AgentMetadata `json:"agents"`
}

// AgentProvider provides discovery and lookup for available agents.
type AgentProvider interface {
	// Get retrieves an agent by name.
	Get(name string) (Agent, error)

	// List returns all available agents.
	List() []Agent
}

// Invocation carries the context for a single agent run.
// It is passed to Requirements and InstructionFunc for per-turn customisation.
//
// History is populated from the thread aggregate and used by Requirements
// (e.g. ConditionalRequirement) to inspect tool-call history.
//
// State provides read-only access to the live session state, mirroring
// ADK's InvocationContext.Session().State() — enabling InstructionFunc and
// Requirements to read values accumulated by prior agents (e.g. via OutputKey).
type Invocation struct {
	// ID is a unique identifier for this invocation.
	ID string

	// SessionID identifies the conversation session.
	SessionID string

	// UserID identifies the user making the request.
	UserID string

	// Message is the user's input message.
	Message *Message

	// History contains previous messages in the conversation.
	// Used by Requirements to evaluate tool-call history.
	History []*Message

	// State provides read-only access to the current session state.
	// nil when the adapter does not populate state (e.g. in tests).
	State ReadonlyState

	// Context is custom context data (e.g., attachment refs).
	Context map[string]any

	// Config contains agent execution configuration.
	Config *RunConfig
}

// Clone creates a shallow copy of the Invocation.
func (inv *Invocation) Clone() *Invocation {
	if inv == nil {
		return nil
	}
	clone := *inv
	if inv.Message != nil {
		clone.Message = inv.Message.Clone()
	}
	if inv.History != nil {
		clone.History = make([]*Message, len(inv.History))
		for i, m := range inv.History {
			clone.History[i] = m.Clone()
		}
	}
	if inv.Context != nil {
		clone.Context = make(map[string]any, len(inv.Context))
		for k, v := range inv.Context {
			clone.Context[k] = v
		}
	}
	return &clone
}

// RunConfig controls agent execution behaviour.
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
