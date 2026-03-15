package agentstore

import (
	"context"
	"time"
)

// AgentDefinition represents a user-created agent definition.
// Each user can create agents that are scoped only to them.
//
// Multi-agent orchestration fields map directly to ADK-Go's llmagent.Config:
//   - TransferPolicy controls transfer_to_agent delegation
//   - OutputKey stores agent output in session state for downstream agents
//   - IncludeContents controls history forwarding in sub-agent contexts
//   - GlobalInstruction is shared across the entire agent tree (root only)
//   - InputSchema/OutputSchema constrain agent I/O
type AgentDefinition struct {
	ID              string `json:"id" db:"id"`
	UserID          string `json:"user_id" db:"user_id"`
	Name            string `json:"name" db:"name"`
	Description     string `json:"description" db:"description"`
	SystemPrompt    string `json:"system_prompt" db:"system_prompt"`
	Icon            string `json:"icon" db:"icon"`
	SourceCatalogID string `json:"source_catalog_id,omitempty" db:"source_catalog_id"`

	// ── Multi-Agent Orchestration ──

	// DisallowTransferToParent prevents delegation back to the parent agent.
	DisallowTransferToParent bool `json:"disallow_transfer_to_parent" db:"disallow_transfer_to_parent"`

	// DisallowTransferToPeers prevents delegation to sibling agents.
	DisallowTransferToPeers bool `json:"disallow_transfer_to_peers" db:"disallow_transfer_to_peers"`

	// OutputKey stores the agent's final output in session state under this key.
	// Downstream agents can reference it via {output_key} template syntax.
	OutputKey string `json:"output_key,omitempty" db:"output_key"`

	// IncludeContents controls whether conversation history is forwarded
	// when this agent runs as a sub-agent. "default" or "none".
	IncludeContents string `json:"include_contents,omitempty" db:"include_contents"`

	// GlobalInstruction is shared across the entire agent tree (root agent only).
	GlobalInstruction string `json:"global_instruction,omitempty" db:"global_instruction"`

	// InputSchemaJSON is the JSON-encoded input schema for when this agent
	// is used as a tool (AgentTool pattern). Stored as JSONB.
	InputSchemaJSON string `json:"input_schema,omitempty" db:"input_schema"`

	// OutputSchemaJSON is the JSON-encoded output schema that constrains the
	// agent's response format. When set, tool use is disabled. Stored as JSONB.
	OutputSchemaJSON string `json:"output_schema,omitempty" db:"output_schema"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// AgentDefinitionRepository defines the storage contract for user-created agents.
type AgentDefinitionRepository interface {
	// Create persists a new agent definition.
	Create(ctx context.Context, agent *AgentDefinition) error

	// Update modifies an existing agent definition (must be owned by user).
	Update(ctx context.Context, agent *AgentDefinition) error

	// Delete removes an agent definition by ID (must be owned by user).
	Delete(ctx context.Context, id, userID string) error

	// GetByID retrieves a single agent by ID (must be owned by user).
	GetByID(ctx context.Context, id, userID string) (*AgentDefinition, error)

	// ListByUser returns all agents created by a specific user.
	ListByUser(ctx context.Context, userID string) ([]AgentDefinition, error)

	// Search finds agents matching a query string within a user's agents.
	Search(ctx context.Context, userID, query string) ([]AgentDefinition, error)

	// ListAddedCatalogIDs returns the set of catalog entry IDs that the
	// user has already forked into their personal agents.
	ListAddedCatalogIDs(ctx context.Context, userID string) ([]string, error)

	// ExistsByCatalogID checks whether the user already has an agent
	// forked from the given catalog entry. Used to prevent duplicates.
	ExistsByCatalogID(ctx context.Context, userID, catalogID string) (bool, error)
}
