package agentdefinition

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// yamlCatalogSpec mirrors the YAML schema for a single catalog entry file.
type yamlCatalogSpec struct {
	Name         string            `yaml:"name"`
	Slug         string            `yaml:"slug"` // deterministic ID seed; defaults to Name
	Description  string            `yaml:"description"`
	SystemPrompt string            `yaml:"system_prompt"` // non-empty for single-agent entries
	Category     string            `yaml:"category"`
	Tags         []string          `yaml:"tags"`
	AuthorName   string            `yaml:"author_name"`
	Icon         string            `yaml:"icon"` // named icon (e.g. "bot") or raw SVG
	Featured     bool              `yaml:"featured"`
	Workflow     *yamlWorkflowNode `yaml:"workflow"`   // nil for single-agent entries
	SubAgents    []yamlSubAgent    `yaml:"sub_agents"` // nil for single-agent entries
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
}

// ParseCatalogEntryYAML parses one catalog-style YAML document: a single LLM agent
// or a workflow with optional sub_agents definitions.
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
		ID:           slug,
		Name:         s.Name,
		Description:  s.Description,
		SystemPrompt: s.SystemPrompt,
		Category:     s.Category,
		Tags:         s.Tags,
		AuthorName:   s.AuthorName,
		Icon:         s.Icon,
		Featured:     s.Featured,
	}

	if s.Workflow != nil {
		graphJSON, err := json.Marshal(s.Workflow.toMap())
		if err != nil {
			return e, fmt.Errorf("marshaling workflow: %w", err)
		}
		e.WorkflowGraphJSON = string(graphJSON)
	}

	if len(s.SubAgents) > 0 {
		subs := make([]CatalogSubAgent, len(s.SubAgents))
		for i, sa := range s.SubAgents {
			subs[i] = CatalogSubAgent{
				ID:           sa.ID,
				Name:         sa.Name,
				Description:  sa.Description,
				SystemPrompt: sa.SystemPrompt,
				Icon:         sa.Icon,
			}
		}
		subsJSON, err := json.Marshal(subs)
		if err != nil {
			return e, fmt.Errorf("marshaling sub_agents: %w", err)
		}
		e.SubAgentsJSON = string(subsJSON)
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
