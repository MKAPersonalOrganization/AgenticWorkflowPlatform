package skills

import (
	"context"
	"time"

	"school-with-ai/foundation/agent"
	"school-with-ai/platform/toolregistry"

	"github.com/a-h/templ"
)

// SkillResolver is the interface the plugin uses to look up skills at runtime.
// The composition root provides an adapter that maps skillstore.UserSkill → Skill.
type SkillResolver interface {
	ListSkillsForAgent(ctx context.Context, userID, agentID string) ([]Skill, error)
	GetSkillByNameForAgent(ctx context.Context, userID, agentID, skillName string) (*Skill, error)
}

// Provider implements the toolregistry.PluginProvider interface for the
// skills activation toolset.
//
// Inspired by Gemini CLI's activate_skill tool — enables agents to dynamically
// load specialized instruction sets at runtime, gaining both instructions and
// context for specialized tasks.
type Provider struct {
	// SkillResolver is set during initialization (from main.go) to allow the
	// plugin to look up skills without importing the skillstore module directly.
	SkillResolver SkillResolver

	// Tracker records which skills have been activated per thread so that
	// instructions can be injected into the system prompt on subsequent turns.
	Tracker *SkillActivationTracker
}

func (p *Provider) ID() string { return "skills" }

func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(
		`<path d="M13 2 3 14h9l-1 8 10-12h-9l1-8z"/>`,
	)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "skills-set",
			Name:        "Skills",
			Description: "Tools for dynamically activating specialized agent skills at runtime",
			Category:    "Skills",
			ToolIDs:     []string{"list_skills", "activate_skill"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{
			ID:          "list_skills",
			Name:        "List Skills",
			Description: "Lists all skills available to the current agent. Use this to discover which skills can be activated.",
			Category:    "Skills",
			ToolSetID:   "skills-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "activate_skill",
			Name:        "Activate Skill",
			Description: "Dynamically activates a specialized skill by name, loading its instructions and resources into the agent's context. Use this when you need specialized expertise for a task.",
			Category:    "Skills",
			ToolSetID:   "skills-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

func (p *Provider) CreateExecutable(_ context.Context, toolID string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	switch toolID {
	case "list_skills":
		return &ListSkillsTool{
			resolver: p.SkillResolver,
		}, nil
	case "activate_skill":
		return &ActivateSkillTool{
			resolver: p.SkillResolver,
			tracker:  p.Tracker,
		}, nil
	default:
		return nil, nil
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"list_skills", "activate_skill"}
}
