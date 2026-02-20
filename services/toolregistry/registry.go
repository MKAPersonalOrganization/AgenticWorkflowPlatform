package toolregistry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"school-with-ai/foundation/logger"
)

// Tool represents a tool available in the registry.
type Tool struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	ToolSetID   string            `json:"tool_set_id,omitempty"` // Optional: which tool set this tool belongs to
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ToolSet represents a collection of related tools.
type ToolSet struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	ToolIDs     []string          `json:"tool_ids"` // List of tool IDs in this set
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Repository defines the storage interface for tool registry data.
type Repository interface {
	// Tools
	GetTool(ctx context.Context, id string) (*Tool, error)
	ListTools(ctx context.Context) ([]Tool, error)
	SaveTool(ctx context.Context, tool *Tool) error
	DeleteTool(ctx context.Context, id string) error

	// Tool Sets
	GetToolSet(ctx context.Context, id string) (*ToolSet, error)
	ListToolSets(ctx context.Context) ([]ToolSet, error)
	SaveToolSet(ctx context.Context, toolSet *ToolSet) error
	DeleteToolSet(ctx context.Context, id string) error
}

// Registry manages tools, tool sets, and their configurations per agent.
type Registry struct {
	log          *logger.Logger
	repo         Repository
	mutex        sync.RWMutex
	toolCache    map[string]*Tool    // toolID -> Tool
	toolSetCache map[string]*ToolSet // toolSetID -> ToolSet
}

// NewRegistry creates a new tool registry.
func NewRegistry(log *logger.Logger, repo Repository) *Registry {
	return &Registry{
		log:          log,
		repo:         repo,
		toolCache:    make(map[string]*Tool),
		toolSetCache: make(map[string]*ToolSet),
	}
}

// Initialize loads default tools and tool sets.
func (r *Registry) Initialize(ctx context.Context) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Load default tools if registry is empty
	tools, err := r.repo.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to load tools: %w", err)
	}

	// Load tool sets
	toolSets, err := r.repo.ListToolSets(ctx)
	if err != nil {
		return fmt.Errorf("failed to load tool sets: %w", err)
	}

	if len(tools) == 0 {
		r.log.Info(ctx, "No tools found, initializing default tools and tool sets")
		if err := r.initializeDefaultTools(ctx); err != nil {
			return fmt.Errorf("failed to initialize default tools: %w", err)
		}
	} else {
		// Cache existing tools
		for _, tool := range tools {
			r.toolCache[tool.ID] = &tool
		}

		// Cache existing tool sets
		for _, toolSet := range toolSets {
			r.toolSetCache[toolSet.ID] = &toolSet
		}
	}

	r.log.Info(ctx, "Tool registry initialized", "tool_count", len(r.toolCache), "tool_set_count", len(r.toolSetCache))
	return nil
}

// GetTool retrieves a tool by ID.
func (r *Registry) GetTool(ctx context.Context, id string) (*Tool, error) {
	r.mutex.RLock()
	if tool, exists := r.toolCache[id]; exists {
		r.mutex.RUnlock()
		return tool, nil
	}
	r.mutex.RUnlock()

	// Not in cache, load from repository
	tool, err := r.repo.GetTool(ctx, id)
	if err != nil {
		return nil, err
	}

	r.mutex.Lock()
	r.toolCache[id] = tool
	r.mutex.Unlock()

	return tool, nil
}

// ListTools returns all available tools.
func (r *Registry) ListTools(ctx context.Context) ([]Tool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	tools := make([]Tool, 0, len(r.toolCache))
	for _, tool := range r.toolCache {
		tools = append(tools, *tool)
	}

	return tools, nil
}

// RegisterTool adds a new tool to the registry.
func (r *Registry) RegisterTool(ctx context.Context, tool *Tool) error {
	now := time.Now()
	if tool.CreatedAt.IsZero() {
		tool.CreatedAt = now
	}
	tool.UpdatedAt = now

	if err := r.repo.SaveTool(ctx, tool); err != nil {
		return fmt.Errorf("failed to save tool: %w", err)
	}

	r.mutex.Lock()
	r.toolCache[tool.ID] = tool
	r.mutex.Unlock()

	r.log.Info(ctx, "Tool registered", "tool_id", tool.ID, "name", tool.Name)
	return nil
}

// GetToolSet retrieves a tool set by ID.
func (r *Registry) GetToolSet(ctx context.Context, id string) (*ToolSet, error) {
	r.mutex.RLock()
	if toolSet, exists := r.toolSetCache[id]; exists {
		r.mutex.RUnlock()
		return toolSet, nil
	}
	r.mutex.RUnlock()

	// Not in cache, load from repository
	toolSet, err := r.repo.GetToolSet(ctx, id)
	if err != nil {
		return nil, err
	}

	r.mutex.Lock()
	r.toolSetCache[id] = toolSet
	r.mutex.Unlock()

	return toolSet, nil
}

// ListToolSets returns all available tool sets.
func (r *Registry) ListToolSets(ctx context.Context) ([]ToolSet, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	toolSets := make([]ToolSet, 0, len(r.toolSetCache))
	for _, toolSet := range r.toolSetCache {
		toolSets = append(toolSets, *toolSet)
	}

	return toolSets, nil
}

// RegisterToolSet adds a new tool set to the registry.
func (r *Registry) RegisterToolSet(ctx context.Context, toolSet *ToolSet) error {
	now := time.Now()
	if toolSet.CreatedAt.IsZero() {
		toolSet.CreatedAt = now
	}
	toolSet.UpdatedAt = now

	if err := r.repo.SaveToolSet(ctx, toolSet); err != nil {
		return fmt.Errorf("failed to save tool set: %w", err)
	}

	r.mutex.Lock()
	r.toolSetCache[toolSet.ID] = toolSet
	r.mutex.Unlock()

	r.log.Info(ctx, "Tool set registered", "tool_set_id", toolSet.ID, "name", toolSet.Name)
	return nil
}

// GetToolsInSet retrieves all tools that belong to a specific tool set.
func (r *Registry) GetToolsInSet(ctx context.Context, toolSetID string) ([]Tool, error) {
	toolSet, err := r.GetToolSet(ctx, toolSetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool set %s: %w", toolSetID, err)
	}

	tools := make([]Tool, 0, len(toolSet.ToolIDs))
	for _, toolID := range toolSet.ToolIDs {
		tool, err := r.GetTool(ctx, toolID)
		if err != nil {
			r.log.Warn(ctx, "Tool in set not found", "tool_id", toolID, "tool_set_id", toolSetID)
			continue
		}
		tools = append(tools, *tool)
	}

	return tools, nil
}

// initializeDefaultTools creates the initial set of tools and tool sets.
func (r *Registry) initializeDefaultTools(ctx context.Context) error {
	now := time.Now()

	// Gather default tools and sets from registered plugins
	var defaultToolSets []ToolSet
	var defaultTools []Tool

	for _, provider := range GetAllPluginProviders() {
		// Collect tool sets
		for _, ts := range provider.DefaultSets() {
			ts.CreatedAt = now
			ts.UpdatedAt = now
			defaultToolSets = append(defaultToolSets, ts)
		}

		// Collect tools
		for _, t := range provider.DefaultTools() {
			t.CreatedAt = now
			t.UpdatedAt = now
			defaultTools = append(defaultTools, t)
		}
	}

	// Save tool sets first
	for _, toolSet := range defaultToolSets {
		if err := r.repo.SaveToolSet(ctx, &toolSet); err != nil {
			return fmt.Errorf("failed to save default tool set %s: %w", toolSet.ID, err)
		}
		r.toolSetCache[toolSet.ID] = &toolSet
	}

	// Save tools
	for _, tool := range defaultTools {
		if err := r.repo.SaveTool(ctx, &tool); err != nil {
			return fmt.Errorf("failed to save default tool %s: %w", tool.ID, err)
		}
		r.toolCache[tool.ID] = &tool
	}

	return nil
}
