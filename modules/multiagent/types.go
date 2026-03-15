package multiagent

import (
	"context"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Node Types — the atomic building blocks for arbitrary composition
// ──────────────────────────────────────────────────────────────────────────────

// NodeType identifies what kind of node this is in the composition tree.
type NodeType string

const (
	// NodeAgent is a leaf node referencing an existing AgentDefinition.
	NodeAgent NodeType = "agent"

	// NodeSequential is a composite node that runs children one after another.
	NodeSequential NodeType = "sequential"

	// NodeParallel is a composite node that runs all children concurrently.
	NodeParallel NodeType = "parallel"

	// NodeLoop is a composite node that repeats children until max iterations
	// or until the last child's LLM invokes the exit_loop tool.
	NodeLoop NodeType = "loop"
)

// ValidNodeTypes is the set of supported node types.
var ValidNodeTypes = map[NodeType]bool{
	NodeAgent:      true,
	NodeSequential: true,
	NodeParallel:   true,
	NodeLoop:       true,
}

// IsComposite returns true if the node type can contain children.
func (nt NodeType) IsComposite() bool {
	return nt == NodeSequential || nt == NodeParallel || nt == NodeLoop
}

// ──────────────────────────────────────────────────────────────────────────────
// WorkflowNode — the recursive building block
//
// Users compose arbitrary trees of nodes. Composite nodes (sequential,
// parallel, loop) contain children. Leaf nodes (agent) reference an
// existing AgentDefinition by ID.
//
// Example:
//
//sequential
//├─ agent(researcher)
//├─ parallel
//│  ├─ agent(writer)
//│  └─ agent(fact-checker)
//└─ loop
//   ├─ agent(reviewer)
//   └─ agent(editor)
//
// This maps directly to ADK-Go:
//
//sequentialagent.New(SubAgents: [researcher, parallelagent.New(...), loopagent.New(...)])
//
// ──────────────────────────────────────────────────────────────────────────────

// WorkflowNode is the recursive building block for arbitrary agent composition.
// It forms a tree: composite nodes hold children, leaf nodes reference agents.
//
// Agent nodes can optionally override the referenced agent's transfer policy
// and content forwarding behaviour within this specific workflow context.
type WorkflowNode struct {
	// ID is a unique identifier within this tree (e.g., "node-1", "node-2").
	// Used by the frontend to track nodes for editing.
	ID string `json:"id"`

	// Type determines whether this is a leaf (agent) or composite (sequential/parallel/loop).
	Type NodeType `json:"type"`

	// Label is an optional display name for this node (shown in the UI).
	// For agent nodes, defaults to the referenced agent's name.
	Label string `json:"label,omitempty"`

	// ── Leaf-node fields (Type = "agent") ──

	// AgentID references an existing AgentDefinition owned by the same user.
	// Only set when Type = "agent".
	AgentID string `json:"agent_id,omitempty"`

	// OutputKey optionally stores this node's output in session state,
	// making it available to subsequent nodes via {output_key} template syntax.
	// Only meaningful for agent nodes inside sequential or loop parents.
	OutputKey string `json:"output_key,omitempty"`

	// DisallowTransferToParent overrides the agent's transfer policy for
	// this workflow context. When true, the agent cannot delegate back to
	// its parent in the workflow tree.
	DisallowTransferToParent *bool `json:"disallow_transfer_to_parent,omitempty"`

	// DisallowTransferToPeers overrides the agent's transfer policy for
	// this workflow context. When true, the agent cannot delegate to
	// sibling agents.
	DisallowTransferToPeers *bool `json:"disallow_transfer_to_peers,omitempty"`

	// IncludeContents overrides the agent's content forwarding for this
	// workflow context. "default" includes full history, "none" starts fresh.
	IncludeContents string `json:"include_contents,omitempty"`

	// ── Composite-node fields (Type = sequential/parallel/loop) ──

	// Children are the sub-nodes of this composite node.
	// Only set when Type is a composite type.
	Children []WorkflowNode `json:"children,omitempty"`

	// MaxIterations limits loop repetitions. Only used when Type = "loop".
	// 0 means use the default (3).
	MaxIterations int `json:"max_iterations,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// WorkflowDefinition — the top-level stored entity
// ──────────────────────────────────────────────────────────────────────────────

// WorkflowDefinition represents a user-composed multi-agent workflow.
// The composition is stored as a single recursive tree (Graph) encoded
// as JSONB in the database.
type WorkflowDefinition struct {
	ID          string       `json:"id" db:"id"`
	UserID      string       `json:"user_id" db:"user_id"`
	Name        string       `json:"name" db:"name"`
	Description string       `json:"description" db:"description"`
	Icon        string       `json:"icon" db:"icon"`
	Graph       WorkflowNode `json:"graph" db:"-"` // the composition tree
	GraphJSON   string       `json:"-" db:"graph"` // raw JSONB for DB scanning
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at" db:"updated_at"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Repository Interface
// ──────────────────────────────────────────────────────────────────────────────

// WorkflowDefinitionRepository defines the storage contract for workflows.
type WorkflowDefinitionRepository interface {
	// Create persists a new workflow definition.
	Create(ctx context.Context, wf *WorkflowDefinition) error

	// Update modifies an existing workflow (must be owned by user).
	Update(ctx context.Context, wf *WorkflowDefinition) error

	// Delete removes a workflow by ID (must be owned by user).
	Delete(ctx context.Context, id, userID string) error

	// GetByID retrieves a single workflow by ID (must be owned by user).
	GetByID(ctx context.Context, id, userID string) (*WorkflowDefinition, error)

	// ListByUser returns all workflows for a user, ordered by creation time.
	ListByUser(ctx context.Context, userID string) ([]WorkflowDefinition, error)
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// CollectAgentIDs returns all agent IDs referenced in the tree.
func (n *WorkflowNode) CollectAgentIDs() []string {
	var ids []string
	n.walk(func(node *WorkflowNode) {
		if node.Type == NodeAgent && node.AgentID != "" {
			ids = append(ids, node.AgentID)
		}
	})
	return ids
}

// CountNodes returns the total number of nodes in the tree.
func (n *WorkflowNode) CountNodes() int {
	count := 0
	n.walk(func(_ *WorkflowNode) { count++ })
	return count
}

// Depth returns the maximum depth of the tree (1 for a single leaf).
func (n *WorkflowNode) Depth() int {
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

// walk visits every node in the tree depth-first.
func (n *WorkflowNode) walk(fn func(*WorkflowNode)) {
	fn(n)
	for i := range n.Children {
		n.Children[i].walk(fn)
	}
}
