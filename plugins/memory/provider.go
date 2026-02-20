package memory

import (
	"context"
	"time"

	"github.com/a-h/templ"

	"school-with-ai/foundation/agent"
	"school-with-ai/services/toolregistry"
	"school-with-ai/services/memory"
)

// Provider implements the toolregistry.PluginProvider interface for the Memory toolset.
type Provider struct{}

func (p *Provider) ID() string {
	return "memory"
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "memory-set",
			Name:        "Memory Management",
			Description: "Tools for managing long-term memory and information storage",
			Category:    "Productivity",
			ToolIDs:     []string{"mem-remember", "mem-forget", "mem-list", "mem-clear"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{ID: "mem-remember", Name: "Remember", Description: "Store information in long-term memory", Category: "Memory", ToolSetID: "memory-set", CreatedAt: now, UpdatedAt: now},
		{ID: "mem-forget", Name: "Forget", Description: "Remove information from memory", Category: "Memory", ToolSetID: "memory-set", CreatedAt: now, UpdatedAt: now},
		{ID: "mem-list", Name: "List Memories", Description: "List all stored memories", Category: "Memory", ToolSetID: "memory-set", CreatedAt: now, UpdatedAt: now},
		{ID: "mem-clear", Name: "Clear Memory", Description: "Clear all stored memories", Category: "Memory", ToolSetID: "memory-set", CreatedAt: now, UpdatedAt: now},
	}
}

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	memService, ok := deps.MemoryService.(memory.Service)
	if !ok {
		// Just panic or fail since dependencies should be provided
	}

	switch toolID {
	case "mem-remember":
		return NewMemoryTool(memService), nil
	case "mem-forget":
		return NewForgetMemoryTool(memService), nil
	case "mem-list":
		return NewListMemoriesTool(memService), nil
	case "mem-clear":
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

func (p *Provider) UIHandledElicitations() []string {
	return nil
}
