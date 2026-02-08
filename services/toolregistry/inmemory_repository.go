package toolregistry

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryRepository implements Repository interface with in-memory storage.
// This is suitable for development and testing. For production, use a persistent storage.
type InMemoryRepository struct {
	mutex sync.RWMutex
	tools map[string]*Tool // toolID -> Tool
}

// NewInMemoryRepository creates a new in-memory repository.
func NewInMemoryRepository() Repository {
	return &InMemoryRepository{
		tools: make(map[string]*Tool),
	}
}

// GetTool retrieves a tool by ID.
func (r *InMemoryRepository) GetTool(ctx context.Context, id string) (*Tool, error) {
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
func (r *InMemoryRepository) ListTools(ctx context.Context) ([]Tool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, *tool) // Copy to prevent external modifications
	}

	return tools, nil
}

// SaveTool saves or updates a tool.
func (r *InMemoryRepository) SaveTool(ctx context.Context, tool *Tool) error {
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
