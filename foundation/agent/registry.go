package agent

import (
	"context"
	"errors"
)

// Provider dynamically supplies tools, supporting lazy loading and MCP (Model Context Protocol).
type Provider interface {
	// Load retrieves all tools from this provider.
	Load(ctx context.Context) ([]Tool, error)

	// Close releases any resources held by the provider.
	Close() error
}

// Registry manages tool registration and lookup.
type Registry interface {
	// Register adds a tool to the registry.
	Register(t Tool) error

	// Get retrieves a tool by name.
	Get(name string) (Tool, bool)

	// All returns all registered tools.
	All() []Tool

	// LoadFrom adds tools from a provider.
	LoadFrom(ctx context.Context, p Provider) error
}

// toolRegistry is the default implementation of Registry.
type toolRegistry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry.
func NewRegistry() Registry {
	return &toolRegistry{
		tools: make(map[string]Tool),
	}
}

func (r *toolRegistry) Register(t Tool) error {
	if t == nil {
		return errors.New("cannot register nil tool")
	}
	if t.Name() == "" {
		return errors.New("tool name cannot be empty")
	}
	r.tools[t.Name()] = t
	return nil
}

func (r *toolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *toolRegistry) All() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

func (r *toolRegistry) LoadFrom(ctx context.Context, p Provider) error {
	tools, err := p.Load(ctx)
	if err != nil {
		return err
	}
	for _, t := range tools {
		if err := r.Register(t); err != nil {
			return err
		}
	}
	return nil
}
