package agentdefinition

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// yamlCatalogSpec mirrors the YAML schema for a single catalog entry file.
type yamlCatalogSpec struct {
	Name              string             `yaml:"name"`
	Slug              string             `yaml:"slug"` // deterministic ID seed; defaults to Name
	Description       string             `yaml:"description"`
	SystemPrompt      string             `yaml:"system_prompt"`                // non-empty for single-agent entries
	GlobalInstruction string             `yaml:"global_instruction,omitempty"` // root-only directive shared by every agent in the tree
	Category          string             `yaml:"category"`
	Tags              []string           `yaml:"tags"`
	AuthorName        string             `yaml:"author_name"`
	Icon              string             `yaml:"icon"` // named icon (e.g. "bot") or raw SVG
	Featured          bool               `yaml:"featured"`
	MemoryPolicy      *yamlMemoryPolicy  `yaml:"memory_policy,omitempty"` // per-agent cross-session memory policy
	Workflow          *yamlWorkflowNode  `yaml:"workflow"`                // nil for single-agent entries
	LLMAgents         []yamlSubAgent     `yaml:"llm_agents"`              // nil for single-agent entries
	AgentTools        []yamlAgentToolRef `yaml:"agent_tools,omitempty"`   // expose sub-agents as callable tools
}

// yamlMemoryPolicy is the YAML shape for AgentMemoryPolicy. Fields map
// verbatim onto the persisted type. Values only matter on the ROOT agent
// (single agent or workflow root) — children inherit from the runner's
// memory service; there's no per-child policy.
type yamlMemoryPolicy struct {
	Enabled bool `yaml:"enabled"`
	Preload bool `yaml:"preload,omitempty"`
}

type yamlWorkflowNode struct {
	ID             string             `yaml:"id"`
	Type           string             `yaml:"type"` // agent | sequential | parallel | loop | conditional
	AgentID        string             `yaml:"agent_id,omitempty"`
	OutputKey      string             `yaml:"output_key,omitempty"`
	MaxIterations  int                `yaml:"max_iterations,omitempty"`
	AllowEarlyExit bool               `yaml:"allow_early_exit,omitempty"`
	Condition      string             `yaml:"condition,omitempty"` // for conditional routing branches
	Children       []yamlWorkflowNode `yaml:"children,omitempty"`
}

type yamlSubAgent struct {
	ID           string `yaml:"id"`
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	Icon         string `yaml:"icon"` // named icon or raw SVG
	SystemPrompt string `yaml:"system_prompt"`
	OutputKey    string `yaml:"output_key,omitempty"` // session-state key ADK writes the agent's output to; referenced by sibling prompts as {key}
}

type yamlAgentToolRef struct {
	AgentID           string `yaml:"agent_id"`
	SkipSummarization bool   `yaml:"skip_summarization,omitempty"`
	InputSchema       string `yaml:"input_schema,omitempty"` // JSON schema for callable signature when used as tool
}

// ParseCatalogEntryYAML parses one catalog-style YAML document: a single LLM agent
// or a workflow with optional llm_agents definitions.
func ParseCatalogEntryYAML(data []byte) (AgentCatalogEntry, error) {
	var spec yamlCatalogSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return AgentCatalogEntry{}, fmt.Errorf("parsing YAML: %w", err)
	}
	return spec.toEntry()
}

func (s *yamlCatalogSpec) toEntry() (AgentCatalogEntry, error) {
	slug := s.Slug
	if slug == "" {
		slug = s.Name
	}

	e := AgentCatalogEntry{
		ID:                slug,
		Name:              s.Name,
		Description:       s.Description,
		SystemPrompt:      s.SystemPrompt,
		GlobalInstruction: s.GlobalInstruction,
		Category:          s.Category,
		Tags:              s.Tags,
		AuthorName:        s.AuthorName,
		Icon:              s.Icon,
		Featured:          s.Featured,
	}

	if s.MemoryPolicy != nil {
		e.MemoryPolicy = &AgentMemoryPolicy{
			Enabled: s.MemoryPolicy.Enabled,
			Preload: s.MemoryPolicy.Preload,
		}
	}

	if s.Workflow != nil {
		graphJSON, err := json.Marshal(s.Workflow.toMap())
		if err != nil {
			return e, fmt.Errorf("marshaling workflow: %w", err)
		}
		e.WorkflowGraphJSON = string(graphJSON)
	}

	if len(s.LLMAgents) > 0 {
		subs := make([]CatalogSubAgent, len(s.LLMAgents))
		for i, sa := range s.LLMAgents {
			subs[i] = CatalogSubAgent{
				ID:           sa.ID,
				Name:         sa.Name,
				Description:  sa.Description,
				SystemPrompt: sa.SystemPrompt,
				Icon:         sa.Icon,
				OutputKey:    sa.OutputKey,
			}
		}
		subsJSON, err := json.Marshal(subs)
		if err != nil {
			return e, fmt.Errorf("marshaling llm_agents: %w", err)
		}
		e.LLMAgentsJSON = string(subsJSON)
	}

	if len(s.AgentTools) > 0 {
		tools := make([]AgentToolRef, len(s.AgentTools))
		for i, t := range s.AgentTools {
			tools[i] = AgentToolRef{
				AgentID:           t.AgentID,
				SkipSummarization: t.SkipSummarization,
			}
		}
		toolsJSON, err := json.Marshal(tools)
		if err != nil {
			return e, fmt.Errorf("marshaling agent_tools: %w", err)
		}
		e.AgentToolsJSON = string(toolsJSON)
	}

	return e, nil
}

func (n *yamlWorkflowNode) toMap() map[string]interface{} {
	m := map[string]interface{}{
		"id":   n.ID,
		"type": n.Type,
	}
	if n.AgentID != "" {
		m["agent_id"] = n.AgentID
	}
	if n.OutputKey != "" {
		m["output_key"] = n.OutputKey
	}
	if n.MaxIterations > 0 {
		m["max_iterations"] = n.MaxIterations
	}
	if n.AllowEarlyExit {
		m["allow_early_exit"] = true
	}
	if n.Condition != "" {
		m["condition"] = n.Condition
	}
	if len(n.Children) > 0 {
		children := make([]map[string]interface{}, len(n.Children))
		for i, c := range n.Children {
			children[i] = c.toMap()
		}
		m["children"] = children
	}
	return m
}
