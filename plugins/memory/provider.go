package memory

import (
	"context"
	"time"

	"school-with-ai/foundation/agent"
	"school-with-ai/platform/toolregistry"

	"github.com/a-h/templ"
)

// Provider implements the toolregistry.PluginProvider interface for the Memory toolset.
type Provider struct{}

func (p *Provider) ID() string { return "memory" }
func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(`<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5V19A9 3 0 0 0 21 19V5"/><path d="M3 12A9 3 0 0 0 21 12"/>`)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "memory-set",
			Name:        "Memory Management",
			Description: "Tools for managing long-term memory and information storage",
			Category:    "Productivity",
			ToolIDs:     []string{"save_memory", "forget_memory", "list_memories", "clear_memories"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{ID: "save_memory", Name: "Remember", Description: "Store information in long-term memory", Category: "Memory", ToolSetID: "memory-set", CreatedAt: now, UpdatedAt: now},
		{ID: "forget_memory", Name: "Forget", Description: "Remove information from memory", Category: "Memory", ToolSetID: "memory-set", CreatedAt: now, UpdatedAt: now},
		{ID: "list_memories", Name: "List Memories", Description: "List all stored memories", Category: "Memory", ToolSetID: "memory-set", CreatedAt: now, UpdatedAt: now},
		{ID: "clear_memories", Name: "Clear Memory", Description: "Clear all stored memories", Category: "Memory", ToolSetID: "memory-set", CreatedAt: now, UpdatedAt: now},
	}
}

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	memService := deps.MemoryService()

	switch toolID {
	case "save_memory":
		return NewMemoryTool(memService), nil
	case "forget_memory":
		return NewForgetMemoryTool(memService), nil
	case "list_memories":
		return NewListMemoriesTool(memService), nil
	case "clear_memories":
		return NewClearMemoriesTool(memService), nil
	default:
		return nil, nil // Not handled by this plugin
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"save_memory", "forget_memory", "list_memories", "clear_memories"}
}
