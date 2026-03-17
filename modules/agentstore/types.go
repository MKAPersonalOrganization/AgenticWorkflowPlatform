package agentstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// AgentDefinition — the universal building block
//
// Every agent in the system — whether a simple LLM agent or a composed
// multi-agent workflow — is an AgentDefinition. This mirrors ADK-Go where
// every agent type (llmagent, sequentialagent, parallelagent, loopagent)
// implements the same Agent interface.
//
// An agent with a non-nil Graph is a "composed agent" that orchestrates
// sub-agents via sequential, parallel, or loop patterns. An agent without
// a graph is a simple LLM agent with an instruction (system prompt).
//
// Creating an agent programmatically (via ADK-Go constructors) and through
// the web interface (via this definition) are completely equivalent.
// ──────────────────────────────────────────────────────────────────────────────

type AgentDefinition struct {
	ID              string `json:"id" db:"id"`
	UserID          string `json:"user_id" db:"user_id"`
	Name            string `json:"name" db:"name"`
	Description     string `json:"description" db:"description"`
	SystemPrompt    string `json:"system_prompt" db:"system_prompt"`
	Icon            string `json:"icon" db:"icon"`
	SourceCatalogID string `json:"source_catalog_id,omitempty" db:"source_catalog_id"`

	// WorkflowID is deprecated. Kept for migration compatibility.
	// In the unified model all agents are first-class and visible.
	WorkflowID string `json:"-" db:"workflow_id"`

	// ── Composition ──
	// Graph holds the composition tree for composed agents.
	// When non-nil, this agent orchestrates sub-agents (sequential, parallel,
	// loop patterns). Leaf nodes reference other AgentDefinitions by ID.
	// Nil/empty means this is a simple LLM agent.
	Graph     *CompositionNode `json:"graph,omitempty" db:"-"`
	GraphJSON string           `json:"-" db:"graph"`

	// ── Multi-Agent Orchestration (used when this agent participates as a sub-agent) ──

	DisallowTransferToParent bool   `json:"disallow_transfer_to_parent" db:"disallow_transfer_to_parent"`
	DisallowTransferToPeers  bool   `json:"disallow_transfer_to_peers" db:"disallow_transfer_to_peers"`
	OutputKey                string `json:"output_key,omitempty" db:"output_key"`
	IncludeContents          string `json:"include_contents,omitempty" db:"include_contents"`
	GlobalInstruction        string `json:"global_instruction,omitempty" db:"global_instruction"`
	InputSchemaJSON          string `json:"input_schema,omitempty" db:"input_schema"`
	OutputSchemaJSON         string `json:"output_schema,omitempty" db:"output_schema"`

	// ── Sharing ──
	// ShareToken is a unique token that enables sharing this agent with other users.
	// Empty means the agent is private. Generate via the share endpoint.
	ShareToken string `json:"share_token,omitempty" db:"share_token"`
	// Visibility controls who can see this agent: "private" (default) or "shared".
	Visibility string `json:"visibility,omitempty" db:"visibility"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// IsComposed returns true if this agent has a composition graph
// (i.e., it orchestrates sub-agents rather than being a simple LLM agent).
func (a *AgentDefinition) IsComposed() bool {
	return a.Graph != nil
}

// MarshalGraph serializes the Graph to GraphJSON for storage.
func (a *AgentDefinition) MarshalGraph() error {
	if a.Graph == nil {
		a.GraphJSON = ""
		return nil
	}
	b, err := json.Marshal(a.Graph)
	if err != nil {
		return fmt.Errorf("marshalling composition graph: %w", err)
	}
	a.GraphJSON = string(b)
	return nil
}

// UnmarshalGraph deserializes GraphJSON into Graph.
func (a *AgentDefinition) UnmarshalGraph() error {
	if a.GraphJSON == "" || a.GraphJSON == "null" {
		a.Graph = nil
		return nil
	}
	var g CompositionNode
	if err := json.Unmarshal([]byte(a.GraphJSON), &g); err != nil {
		return fmt.Errorf("unmarshalling composition graph: %w", err)
	}
	a.Graph = &g
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// CompositionNode — recursive building block for agent composition
//
// Maps 1:1 to ADK-Go's agent types:
//   - "agent"      → llmagent.New (leaf — references another AgentDefinition)
//   - "sequential" → sequentialagent.New (runs children in order)
//   - "parallel"   → parallelagent.New (runs children concurrently)
//   - "loop"       → loopagent.New (repeats children until exit condition)
//   - "remote"     → custom agent.New wrapping an A2A connector (delegates via A2A protocol)
//
// Example tree:
//
//   sequential
//   ├─ agent(researcher)
//   ├─ parallel
//   │  ├─ agent(writer)
//   │  └─ agent(fact-checker)
//   └─ loop
//      ├─ agent(reviewer)
//      └─ agent(editor)
// ──────────────────────────────────────────────────────────────────────────────

// NodeType identifies what kind of node this is in the composition tree.
type NodeType string

const (
	NodeAgent      NodeType = "agent"
	NodeSequential NodeType = "sequential"
	NodeParallel   NodeType = "parallel"
	NodeLoop       NodeType = "loop"
	NodeRemote     NodeType = "remote"
)

// ValidNodeTypes is the set of supported node types.
var ValidNodeTypes = map[NodeType]bool{
	NodeAgent:      true,
	NodeSequential: true,
	NodeParallel:   true,
	NodeLoop:       true,
	NodeRemote:     true,
}

// IsComposite returns true if the node type can contain children.
func (nt NodeType) IsComposite() bool {
	return nt == NodeSequential || nt == NodeParallel || nt == NodeLoop
}

// CompositionNode is the recursive building block for agent composition.
// Composite nodes hold children, leaf nodes reference agent definitions.
type CompositionNode struct {
	ID    string   `json:"id"`
	Type  NodeType `json:"type"`
	Label string   `json:"label,omitempty"`

	// ── Leaf-node fields (Type = "agent") ──
	AgentID string `json:"agent_id,omitempty"`

	// ── Remote-node fields (Type = "remote") ──
	// A2AConnectorID references a registered A2A connector for remote delegation.
	A2AConnectorID string `json:"a2a_connector_id,omitempty"`
	OutputKey                string `json:"output_key,omitempty"`
	DisallowTransferToParent *bool  `json:"disallow_transfer_to_parent,omitempty"`
	DisallowTransferToPeers  *bool  `json:"disallow_transfer_to_peers,omitempty"`
	IncludeContents          string `json:"include_contents,omitempty"`

	// ── Composite-node fields ──
	Children      []CompositionNode `json:"children,omitempty"`
	MaxIterations int               `json:"max_iterations,omitempty"`
}

// CollectAgentIDs returns all agent IDs referenced in the tree.
func (n *CompositionNode) CollectAgentIDs() []string {
	var ids []string
	n.Walk(func(node *CompositionNode) {
		if node.Type == NodeAgent && node.AgentID != "" {
			ids = append(ids, node.AgentID)
		}
	})
	return ids
}

// CountNodes returns the total number of nodes in the tree.
func (n *CompositionNode) CountNodes() int {
	count := 0
	n.Walk(func(_ *CompositionNode) { count++ })
	return count
}

// Depth returns the maximum depth of the tree (1 for a single leaf).
func (n *CompositionNode) Depth() int {
	if len(n.Children) == 0 {
		return 1
	}
	maxChildDepth := 0
	for i := range n.Children {
		d := n.Children[i].Depth()
		if d > maxChildDepth {
			maxChildDepth = d
		}
	}
	return 1 + maxChildDepth
}

// Walk visits every node in the tree depth-first.
func (n *CompositionNode) Walk(fn func(*CompositionNode)) {
	fn(n)
	for i := range n.Children {
		n.Children[i].Walk(fn)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Repository
// ──────────────────────────────────────────────────────────────────────────────

// AgentDefinitionRepository defines the storage contract for user-created agents.
type AgentDefinitionRepository interface {
	Create(ctx context.Context, agent *AgentDefinition) error
	Update(ctx context.Context, agent *AgentDefinition) error
	Delete(ctx context.Context, id, userID string) error
	GetByID(ctx context.Context, id, userID string) (*AgentDefinition, error)
	ListByUser(ctx context.Context, userID string) ([]AgentDefinition, error)
	Search(ctx context.Context, userID, query string) ([]AgentDefinition, error)
	ListAddedCatalogIDs(ctx context.Context, userID string) ([]string, error)
	ExistsByCatalogID(ctx context.Context, userID, catalogID string) (bool, error)

	// GetByShareToken retrieves a shared agent by its share token (no user scoping).
	GetByShareToken(ctx context.Context, token string) (*AgentDefinition, error)

	// UpdateShareToken sets or clears the share token for an agent.
	UpdateShareToken(ctx context.Context, id, userID, token, visibility string) error
}
