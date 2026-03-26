package agentstore

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed catalog/*.yaml
var catalogFS embed.FS

// yamlCatalogSpec mirrors the YAML schema for a single catalog entry file.
//
// Each file in catalog/*.yaml is one agent — single or workflow. When
// loaded, its ID is derived deterministically from the slug field so that
// re-seeding always targets the same row.
type yamlCatalogSpec struct {
	Name         string            `yaml:"name"`
	Slug         string            `yaml:"slug"`         // deterministic ID seed; defaults to Name
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
	Type           string             `yaml:"type"` // agent | sequential | parallel | loop
	AgentID        string             `yaml:"agent_id,omitempty"`
	OutputKey      string             `yaml:"output_key,omitempty"`
	MaxIterations  int                `yaml:"max_iterations,omitempty"`
	AllowEarlyExit bool               `yaml:"allow_early_exit,omitempty"`
	Children       []yamlWorkflowNode `yaml:"children,omitempty"`
}

type yamlSubAgent struct {
	ID           string `yaml:"id"`
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	Icon         string `yaml:"icon"` // named icon or raw SVG
	SystemPrompt string `yaml:"system_prompt"`
}

// loadYAMLCatalog reads every *.yaml file from the embedded catalog/ directory
// and returns the parsed AgentCatalogEntry slice.
func loadYAMLCatalog() ([]AgentCatalogEntry, error) {
	dirs, err := fs.ReadDir(catalogFS, "catalog")
	if err != nil {
		return nil, fmt.Errorf("reading catalog dir: %w", err)
	}

	var out []AgentCatalogEntry
	for _, de := range dirs {
		if de.IsDir() || filepath.Ext(de.Name()) != ".yaml" {
			continue
		}
		raw, err := catalogFS.ReadFile("catalog/" + de.Name())
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", de.Name(), err)
		}

		var spec yamlCatalogSpec
		if err := yaml.Unmarshal(raw, &spec); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", de.Name(), err)
		}

		entry, err := spec.toEntry()
		if err != nil {
			return nil, fmt.Errorf("converting %s: %w", de.Name(), err)
		}
		out = append(out, entry)
	}
	return out, nil
}

func (s *yamlCatalogSpec) toEntry() (AgentCatalogEntry, error) {
	slug := s.Slug
	if slug == "" {
		slug = s.Name
	}

	e := AgentCatalogEntry{
		ID:           deterministicID("catalog-yaml:" + slug),
		Name:         s.Name,
		Description:  s.Description,
		SystemPrompt: s.SystemPrompt,
		Category:     s.Category,
		Tags:         s.Tags,
		AuthorName:   s.AuthorName,
		Icon:         resolveIcon(s.Icon),
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
				Icon:         resolveIcon(sa.Icon),
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
	if len(n.Children) > 0 {
		children := make([]map[string]interface{}, len(n.Children))
		for i, c := range n.Children {
			children[i] = c.toMap()
		}
		m["children"] = children
	}
	return m
}

// resolveIcon maps a short icon name to its Lucide SVG string.
// If the name is not recognised (e.g. it is already raw SVG), it is returned as-is.
func resolveIcon(name string) string {
	m := map[string]string{
		"bot":             iconBot,
		"terminal":        iconTerminal,
		"search-code":     iconSearchCode,
		"code":            iconCode,
		"git-branch":      iconGitBranch,
		"workflow":        iconWorkflow,
		"pipeline":        iconPipeline,
		"refresh":         iconRefresh,
		"graduation-cap":  iconGraduationCap,
		"book-open":       iconBookOpen,
		"pencil-line":     iconPencilLine,
		"clipboard-check": iconClipboardCheck,
		"gamepad":         iconGamepad,
		"plane":           iconPlane,
		"briefcase":       iconBriefcase,
		"camera":          iconCamera,
		"target":          iconTarget,
		"users":           iconUsers,
		"clapperboard":    iconClapperboard,
		"trending-up":     iconTrendingUp,
		"calendar-clock":  iconCalendarClock,
		"map-pin":         iconMapPin,
		"book-open-check": iconBookOpenCheck,
		"sparkles":        iconSparkles,
	}
	if svg, ok := m[name]; ok {
		return svg
	}
	return name
}
