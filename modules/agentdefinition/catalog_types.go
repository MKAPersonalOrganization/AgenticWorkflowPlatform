package agentdefinition

import "time"

// AgentCatalogEntry represents a shared, read-only agent template in the template store.
// These are available to all users and can be "added" (forked) into a user's personal agents.
//
// Entries with a non-empty WorkflowGraphJSON represent multi-agent workflow templates.
// When forked, the system creates the llm agents and a workflow definition for the user.
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

	// GlobalInstruction is a workflow-wide directive applied to every
	// agent in the tree — not just the root's own system prompt. The
	// orchestration engine only honours it when declared on the root
	// (see .READ/adk-go/agent/llmagent/llmagent.go:232), so it lives at
	// the top level of a catalog entry regardless of whether the entry
	// is a single agent or a composed workflow. Template syntax
	// ({key} / {key?}) is supported; values resolve against session
	// state before the LLM is called.
	GlobalInstruction string `json:"global_instruction,omitempty" db:"-"`

	// MemoryPolicy controls cross-session memory for the catalog entry's
	// root agent. Nil means memory is off. Mirrors
	// AgentDefinition.MemoryPolicy.
	MemoryPolicy *AgentMemoryPolicy `json:"memory_policy,omitempty" db:"-"`

	// ArtifactPolicy controls artifact support for the catalog entry's
	// root agent. Nil means artifacts are off. Mirrors
	// AgentDefinition.ArtifactPolicy.
	ArtifactPolicy *AgentArtifactPolicy `json:"artifact_policy,omitempty" db:"-"`

	// ── Workflow Template (optional) ──

	// WorkflowGraphJSON is a JSON-encoded WorkflowNode tree template.
	// Agent nodes use placeholder IDs (e.g. "sub-1") that map to LLMAgentsJSON.
	// When empty, this is a single-agent catalog entry.
	WorkflowGraphJSON string `json:"-" db:"workflow_graph"`

	// LLMAgentsJSON is a JSON-encoded array of CatalogSubAgent definitions.
	// Each entry maps a placeholder ID to the agent's full definition.
	// Note: the DB column is shared with the coordinator pattern (sub_agents).
	LLMAgentsJSON string `json:"-" db:"sub_agents"`

	// AgentToolsJSON is a JSON-encoded array of AgentToolRef for exposing sub-agents as tools.
	AgentToolsJSON string `json:"-" db:"agent_tools"`

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
	ID           string `json:"id"`                   // Placeholder ID (e.g. "sub-1")
	Name         string `json:"name"`                 // Display name
	Description  string `json:"description"`          // Short description
	SystemPrompt string `json:"system_prompt"`        // System prompt for the agent
	Icon         string `json:"icon"`                 // SVG icon markup
	OutputKey    string `json:"output_key,omitempty"` // Session state key to store this agent's output
}
