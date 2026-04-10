package agentdefinition

import "time"

// AgentCatalogEntry represents a shared, read-only agent template in the template store.
// These are available to all users and can be "added" (forked) into a user's personal agents.
//
// Entries with a non-empty WorkflowGraphJSON represent multi-agent workflow templates.
// When forked, the system creates the sub-agents and a workflow definition for the user.
type AgentCatalogEntry struct {
	ID           string   `json:"id" db:"id"`
	Name         string   `json:"name" db:"name"`
	Description  string   `json:"description" db:"description"`
	SystemPrompt string   `json:"system_prompt" db:"system_prompt"`
	Category     string   `json:"category" db:"category"`
	Tags         []string `json:"tags" db:"tags"`
	AuthorName   string   `json:"author_name" db:"author_name"`
	Icon         string   `json:"icon" db:"icon"` // SVG icon markup
	Featured     bool     `json:"featured" db:"featured"`
	AddCount     int64    `json:"add_count" db:"add_count"` // Popularity counter

	// ── Workflow Template (optional) ──

	// WorkflowGraphJSON is a JSON-encoded WorkflowNode tree template.
	// Agent nodes use placeholder IDs (e.g. "sub-1") that map to SubAgentsJSON.
	// When empty, this is a single-agent catalog entry.
	WorkflowGraphJSON string `json:"-" db:"workflow_graph"`

	// SubAgentsJSON is a JSON-encoded array of CatalogSubAgent definitions.
	// Each entry maps a placeholder ID to the agent's full definition.
	SubAgentsJSON string `json:"-" db:"sub_agents"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// IsWorkflow returns true if this catalog entry represents a multi-agent workflow.
func (e *AgentCatalogEntry) IsWorkflow() bool {
	return e.WorkflowGraphJSON != "" && e.WorkflowGraphJSON != "null"
}

// CatalogSubAgent defines a sub-agent template embedded in a workflow catalog entry.
// When the catalog entry is forked, each sub-agent becomes a real AgentDefinition.
type CatalogSubAgent struct {
	ID           string `json:"id"`            // Placeholder ID (e.g. "sub-1")
	Name         string `json:"name"`          // Display name
	Description  string `json:"description"`   // Short description
	SystemPrompt string `json:"system_prompt"` // System prompt for the agent
	Icon         string `json:"icon"`          // SVG icon markup
}
