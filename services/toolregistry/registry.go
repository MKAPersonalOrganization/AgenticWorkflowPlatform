package toolregistry

import (
	"context"
	"fmt"
	"strings"
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

	// Cache existing tools (skip MCP tools — they are ephemeral and
	// will be re-registered when connectors reconnect).
	for _, tool := range tools {
		if strings.HasPrefix(tool.ID, "mcp-") {
			// Clean up stale MCP tools from the DB.
			_ = r.repo.DeleteTool(ctx, tool.ID)
			continue
		}
		r.toolCache[tool.ID] = &tool
	}

	// Cache existing tool sets (skip MCP sets — same reason).
	for _, toolSet := range toolSets {
		if strings.HasPrefix(toolSet.ID, "mcp-") {
			_ = r.repo.DeleteToolSet(ctx, toolSet.ID)
			continue
		}
		r.toolSetCache[toolSet.ID] = &toolSet
	}

	// Initialize default tools (creates them if they don't exist yet)
	if err := r.initializeDefaultTools(ctx); err != nil {
		return fmt.Errorf("failed to initialize default tools: %w", err)
	}

	r.log.Info(ctx, "Tool registry initialized", "tool_count", len(r.toolCache), "tool_set_count", len(r.toolSetCache))
	return nil
}

// GetTool retrieves a tool by ID.
func (r *Registry) GetTool(ctx context.Context, id string) (*Tool, error) {
	r.mutex.RLock()
	if tool, exists := r.toolCache[id]; exists {
		toolCopy := *tool
		r.mutex.RUnlock()
		return &toolCopy, nil
	}
	r.mutex.RUnlock()

	// Not in cache, load from repository
	tool, err := r.repo.GetTool(ctx, id)
	if err != nil {
		return nil, err
	}

	toolCopy := *tool
	r.mutex.Lock()
	r.toolCache[id] = &toolCopy
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

	toolCopy := *tool
	r.mutex.Lock()
	r.toolCache[tool.ID] = &toolCopy
	r.mutex.Unlock()

	r.log.Info(ctx, "Tool registered", "tool_id", tool.ID, "name", tool.Name)
	return nil
}

// UnregisterTool removes a tool from the registry (DB + cache).
func (r *Registry) UnregisterTool(ctx context.Context, id string) error {
	if err := r.repo.DeleteTool(ctx, id); err != nil {
		return fmt.Errorf("failed to delete tool: %w", err)
	}

	r.mutex.Lock()
	delete(r.toolCache, id)
	r.mutex.Unlock()

	return nil
}

// GetToolSet retrieves a tool set by ID.
func (r *Registry) GetToolSet(ctx context.Context, id string) (*ToolSet, error) {
	r.mutex.RLock()
	if toolSet, exists := r.toolSetCache[id]; exists {
		toolSetCopy := *toolSet
		r.mutex.RUnlock()
		return &toolSetCopy, nil
	}
	r.mutex.RUnlock()

	// Not in cache, load from repository
	toolSet, err := r.repo.GetToolSet(ctx, id)
	if err != nil {
		return nil, err
	}

	toolSetCopy := *toolSet
	r.mutex.Lock()
	r.toolSetCache[id] = &toolSetCopy
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

	toolSetCopy := *toolSet
	r.mutex.Lock()
	r.toolSetCache[toolSet.ID] = &toolSetCopy
	r.mutex.Unlock()

	r.log.Info(ctx, "Tool set registered", "tool_set_id", toolSet.ID, "name", toolSet.Name)
	return nil
}

// UnregisterToolSet removes a tool set and its tools from the registry (DB + cache).
func (r *Registry) UnregisterToolSet(ctx context.Context, id string) error {
	// Remove child tools first.
	r.mutex.RLock()
	ts, exists := r.toolSetCache[id]
	var toolIDs []string
	if exists {
		toolIDs = append(toolIDs, ts.ToolIDs...)
	}
	r.mutex.RUnlock()

	for _, toolID := range toolIDs {
		_ = r.UnregisterTool(ctx, toolID)
	}

	if err := r.repo.DeleteToolSet(ctx, id); err != nil {
		return fmt.Errorf("failed to delete tool set: %w", err)
	}

	r.mutex.Lock()
	delete(r.toolSetCache, id)
	r.mutex.Unlock()

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

	// Save tool sets first if they don't exist
	for _, toolSet := range defaultToolSets {
		if _, exists := r.toolSetCache[toolSet.ID]; !exists {
			if err := r.repo.SaveToolSet(ctx, &toolSet); err != nil {
				return fmt.Errorf("failed to save default tool set %s: %w", toolSet.ID, err)
			}
			r.toolSetCache[toolSet.ID] = &toolSet
		}
	}

	// Save tools if they don't exist
	for _, tool := range defaultTools {
		if _, exists := r.toolCache[tool.ID]; !exists {
			if err := r.repo.SaveTool(ctx, &tool); err != nil {
				return fmt.Errorf("failed to save default tool %s: %w", tool.ID, err)
			}
			r.toolCache[tool.ID] = &tool
		}
	}

	return nil
}
