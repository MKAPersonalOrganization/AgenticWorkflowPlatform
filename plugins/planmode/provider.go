package planmode

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/toolregistry"
	"context"
	"time"

	"github.com/a-h/templ"
)

type Provider struct{}

func (p *Provider) ID() string { return "planmode" }

func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(
		`<path d="M12 20h9"/>` +
			`<path d="M16.376 3.622a1 1 0 0 1 3.002 3.002L7.368 18.635a2 2 0 0 1-.855.506l-2.872.838a.5.5 0 0 1-.62-.62l.838-2.872a2 2 0 0 1 .506-.854z"/>` +
			`<path d="m15 5 3 3"/>`,
	)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "planmode-set",
			Name:        "Plan Mode",
			Description: "Tools for research-first planning with user approval before execution",
			Category:    "Planning",
			ToolIDs:     []string{"enter_plan_mode", "exit_plan_mode"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{
			ID:          "enter_plan_mode",
			Name:        "Enter Plan Mode",
			Description: "Switch to research-only plan mode for safe autonomous research",
			Category:    "Planning",
			ToolSetID:   "planmode-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "exit_plan_mode",
			Name:        "Exit Plan Mode",
			Description: "Present the completed plan for user approval and exit plan mode",
			Category:    "Planning",
			ToolSetID:   "planmode-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

func (p *Provider) CreateExecutable(_ context.Context, toolID string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	switch toolID {
	case "enter_plan_mode":
		return &EnterPlanModeTool{}, nil
	case "exit_plan_mode":
		return &ExitPlanModeTool{}, nil
	default:
		return nil, nil
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"enter_plan_mode", "exit_plan_mode"}
}
