package mocks

import (
	"context"
	"fmt"
	"school-with-ai/platform/toolregistry"
	"sync"
)

// InMemoryRepository implements Repository interface with in-memory storage.
// This is suitable for development and testing. For production, use a persistent storage.
type InMemoryRepository struct {
	mutex    sync.RWMutex
	tools    map[string]*toolregistry.Tool    // toolID -> Tool
	toolSets map[string]*toolregistry.ToolSet // toolSetID -> ToolSet
}

// NewInMemoryRepository creates a new in-memory repository.
func NewInMemoryRepository() toolregistry.Repository {
	return &InMemoryRepository{
		tools:    make(map[string]*toolregistry.Tool),
		toolSets: make(map[string]*toolregistry.ToolSet),
	}
}

// GetTool retrieves a tool by ID.
func (r *InMemoryRepository) GetTool(ctx context.Context, id string) (*toolregistry.Tool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	tool, exists := r.tools[id]
	if !exists {
		return nil, fmt.Errorf("tool with id %s not found", id)
	}

	// Return a copy to prevent external modifications
	toolCopy := *tool
	return &toolCopy, nil
}

// ListTools returns all tools.
func (r *InMemoryRepository) ListTools(ctx context.Context) ([]toolregistry.Tool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	tools := make([]toolregistry.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, *tool) // Copy to prevent external modifications
	}

	return tools, nil
}

// SaveTool saves or updates a tool.
func (r *InMemoryRepository) SaveTool(ctx context.Context, tool *toolregistry.Tool) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Create a copy to store
	toolCopy := *tool
	r.tools[tool.ID] = &toolCopy

	return nil
}

// DeleteTool removes a tool by ID.
func (r *InMemoryRepository) DeleteTool(ctx context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.tools[id]; !exists {
		return fmt.Errorf("tool with id %s not found", id)
	}

	delete(r.tools, id)
	return nil
}

// GetToolSet retrieves a tool set by ID.
func (r *InMemoryRepository) GetToolSet(ctx context.Context, id string) (*toolregistry.ToolSet, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	toolSet, exists := r.toolSets[id]
	if !exists {
		return nil, fmt.Errorf("tool set with id %s not found", id)
	}

	// Return a copy to prevent external modifications
	toolSetCopy := *toolSet
	// Deep copy the slice to prevent external modifications
	toolSetCopy.ToolIDs = make([]string, len(toolSet.ToolIDs))
	copy(toolSetCopy.ToolIDs, toolSet.ToolIDs)

	return &toolSetCopy, nil
}

// ListToolSets returns all tool sets.
func (r *InMemoryRepository) ListToolSets(ctx context.Context) ([]toolregistry.ToolSet, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	toolSets := make([]toolregistry.ToolSet, 0, len(r.toolSets))
	for _, toolSet := range r.toolSets {
		// Create a copy with deep copy of slice
		toolSetCopy := *toolSet
		toolSetCopy.ToolIDs = make([]string, len(toolSet.ToolIDs))
		copy(toolSetCopy.ToolIDs, toolSet.ToolIDs)
		toolSets = append(toolSets, toolSetCopy)
	}

	return toolSets, nil
}

// SaveToolSet saves or updates a tool set.
func (r *InMemoryRepository) SaveToolSet(ctx context.Context, toolSet *toolregistry.ToolSet) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Create a copy to store with deep copy of slice
	toolSetCopy := *toolSet
	toolSetCopy.ToolIDs = make([]string, len(toolSet.ToolIDs))
	copy(toolSetCopy.ToolIDs, toolSet.ToolIDs)

	r.toolSets[toolSet.ID] = &toolSetCopy
	return nil
}

// DeleteToolSet removes a tool set by ID.
func (r *InMemoryRepository) DeleteToolSet(ctx context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.toolSets[id]; !exists {
		return fmt.Errorf("tool set with id %s not found", id)
	}

	delete(r.toolSets, id)
	return nil
}
