package toolregistry

import (
	"context"
	"fmt"
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
	SaveTools(ctx context.Context, tools []*Tool) error // batch upsert
	DeleteTool(ctx context.Context, id string) error

	// Tool Sets
	GetToolSet(ctx context.Context, id string) (*ToolSet, error)
	ListToolSets(ctx context.Context) ([]ToolSet, error)
	SaveToolSet(ctx context.Context, toolSet *ToolSet) error
	DeleteToolSet(ctx context.Context, id string) error
}

// Registry manages tools and tool sets. Built-in tools from registered plugins
// are cached in-memory at startup, eliminating DB round-trips for the common
// case. MCP (dynamic) tools still go through PostgreSQL.
type Registry struct {
	log  *logger.Logger
	repo Repository

	// In-memory indices for built-in tools (populated during Initialize).
	builtinTools    map[string]*Tool    // toolID → Tool
	builtinToolSets map[string]*ToolSet // toolSetID → ToolSet
}

// NewRegistry creates a new tool registry.
func NewRegistry(log *logger.Logger, repo Repository) *Registry {
	return &Registry{
		log:             log,
		repo:            repo,
		builtinTools:    make(map[string]*Tool),
		builtinToolSets: make(map[string]*ToolSet),
	}
}

// Initialize ensures default tools and tool sets exist in the database.
func (r *Registry) Initialize(ctx context.Context) error {
	if err := r.initializeDefaultTools(ctx); err != nil {
		return fmt.Errorf("failed to initialize default tools: %w", err)
	}

	tools, err := r.repo.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to count tools: %w", err)
	}
	toolSets, err := r.repo.ListToolSets(ctx)
	if err != nil {
		return fmt.Errorf("failed to count tool sets: %w", err)
	}

	r.log.Info(ctx, "Tool registry initialized", "tool_count", len(tools), "tool_set_count", len(toolSets))
	return nil
}

// GetTool retrieves a tool by ID. Checks the in-memory built-in index first.
func (r *Registry) GetTool(ctx context.Context, id string) (*Tool, error) {
	if t, ok := r.builtinTools[id]; ok {
		return t, nil
	}
	return r.repo.GetTool(ctx, id)
}

// ListTools returns all available tools.
func (r *Registry) ListTools(ctx context.Context) ([]Tool, error) {
	return r.repo.ListTools(ctx)
}

// RegisterTool adds or updates a tool in the registry.
func (r *Registry) RegisterTool(ctx context.Context, tool *Tool) error {
	now := time.Now()
	if tool.CreatedAt.IsZero() {
		tool.CreatedAt = now
	}
	tool.UpdatedAt = now

	if err := r.repo.SaveTool(ctx, tool); err != nil {
		return fmt.Errorf("failed to save tool: %w", err)
	}

	r.log.Info(ctx, "Tool registered", "tool_id", tool.ID, "name", tool.Name)
	return nil
}

// RegisterTools adds or updates multiple tools in a single batch operation.
// This is significantly more efficient than calling RegisterTool in a loop
// when registering many tools (e.g. an MCP connector with 50 tools).
func (r *Registry) RegisterTools(ctx context.Context, tools []*Tool) error {
	if len(tools) == 0 {
		return nil
	}
	now := time.Now()
	for _, tool := range tools {
		if tool.CreatedAt.IsZero() {
			tool.CreatedAt = now
		}
		tool.UpdatedAt = now
	}

	if err := r.repo.SaveTools(ctx, tools); err != nil {
		return fmt.Errorf("failed to batch save tools: %w", err)
	}

	r.log.Info(ctx, "Tools batch registered", "count", len(tools))
	return nil
}

// UnregisterTool removes a tool from the registry.
func (r *Registry) UnregisterTool(ctx context.Context, id string) error {
	if err := r.repo.DeleteTool(ctx, id); err != nil {
		return fmt.Errorf("failed to delete tool: %w", err)
	}
	return nil
}

// GetToolSet retrieves a tool set by ID. Checks the in-memory built-in index first.
func (r *Registry) GetToolSet(ctx context.Context, id string) (*ToolSet, error) {
	if ts, ok := r.builtinToolSets[id]; ok {
		return ts, nil
	}
	return r.repo.GetToolSet(ctx, id)
}

// ListToolSets returns all available tool sets.
func (r *Registry) ListToolSets(ctx context.Context) ([]ToolSet, error) {
	return r.repo.ListToolSets(ctx)
}

// RegisterToolSet adds or updates a tool set in the registry.
func (r *Registry) RegisterToolSet(ctx context.Context, toolSet *ToolSet) error {
	now := time.Now()
	if toolSet.CreatedAt.IsZero() {
		toolSet.CreatedAt = now
	}
	toolSet.UpdatedAt = now

	if err := r.repo.SaveToolSet(ctx, toolSet); err != nil {
		return fmt.Errorf("failed to save tool set: %w", err)
	}

	r.log.Info(ctx, "Tool set registered", "tool_set_id", toolSet.ID, "name", toolSet.Name)
	return nil
}

// UnregisterToolSet removes a tool set and its child tools from the registry.
func (r *Registry) UnregisterToolSet(ctx context.Context, id string) error {
	// Load tool set from DB to find child tool IDs.
	ts, err := r.repo.GetToolSet(ctx, id)
	if err != nil {
		// If the tool set doesn't exist, nothing to unregister.
		return nil
	}

	// Remove child tools first.
	for _, toolID := range ts.ToolIDs {
		_ = r.repo.DeleteTool(ctx, toolID)
	}

	if err := r.repo.DeleteToolSet(ctx, id); err != nil {
		return fmt.Errorf("failed to delete tool set: %w", err)
	}

	return nil
}

// GetToolsInSet retrieves all tools that belong to a specific tool set.
// For built-in tool sets, uses the in-memory index (no DB round-trips).
func (r *Registry) GetToolsInSet(ctx context.Context, toolSetID string) ([]Tool, error) {
	toolSet, err := r.GetToolSet(ctx, toolSetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool set %s: %w", toolSetID, err)
	}

	// Fast path: resolve all tools from in-memory built-in index.
	tools := make([]Tool, 0, len(toolSet.ToolIDs))
	allBuiltin := true
	for _, toolID := range toolSet.ToolIDs {
		if t, ok := r.builtinTools[toolID]; ok {
			tools = append(tools, *t)
		} else {
			allBuiltin = false
			break
		}
	}
	if allBuiltin {
		return tools, nil
	}

	// Slow path (MCP or mixed): fall back to individual lookups.
	tools = make([]Tool, 0, len(toolSet.ToolIDs))
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

// initializeDefaultTools ensures default tools and tool sets from registered
// plugins exist in the database. Uses upsert (ON CONFLICT DO UPDATE) so this
// is safe to run on every pod startup concurrently.
// Also populates the in-memory built-in index for fast lookups.
func (r *Registry) initializeDefaultTools(ctx context.Context) error {
	now := time.Now()

	// Gather default tools and sets from registered plugins.
	for _, provider := range GetAllPluginProviders() {
		for _, ts := range provider.DefaultSets() {
			ts.CreatedAt = now
			ts.UpdatedAt = now
			if err := r.repo.SaveToolSet(ctx, &ts); err != nil {
				return fmt.Errorf("failed to save default tool set %s: %w", ts.ID, err)
			}
			// Populate in-memory index.
			tsCopy := ts
			r.builtinToolSets[ts.ID] = &tsCopy
		}

		for _, t := range provider.DefaultTools() {
			t.CreatedAt = now
			t.UpdatedAt = now
			if err := r.repo.SaveTool(ctx, &t); err != nil {
				return fmt.Errorf("failed to save default tool %s: %w", t.ID, err)
			}
			// Populate in-memory index.
			tCopy := t
			r.builtinTools[t.ID] = &tCopy
		}
	}

	return nil
}
