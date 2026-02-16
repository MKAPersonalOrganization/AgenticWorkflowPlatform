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

	// Define default tool sets
	defaultToolSets := []ToolSet{
		{
			ID:          "calculator-set",
			Name:        "Calculator",
			Description: "Basic mathematical calculation tools",
			Category:    "Math",
			ToolIDs:     []string{"calc-add", "calc-subtract", "calc-multiply", "calc-divide", "calc-power", "calc-sqrt"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "visualization-set",
			Name:        "Visualization",
			Description: "Data visualization and plotting tools",
			Category:    "Math",
			ToolIDs:     []string{"viz-plot", "viz-animate"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "document-set",
			Name:        "Document Management",
			Description: "Tools for managing documents and artifacts",
			Category:    "Productivity",
			ToolIDs:     []string{"doc-create", "doc-update", "doc-delete"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "memory-set",
			Name:        "Memory Management",
			Description: "Tools for managing long-term memory and information storage",
			Category:    "Productivity",
			ToolIDs:     []string{"mem-remember", "mem-forget", "mem-list", "mem-clear"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "statistics-set",
			Name:        "Statistical Analysis",
			Description: "Statistical analysis and data processing tools",
			Category:    "Math",
			ToolIDs:     []string{"stats-mean", "stats-median", "stats-mode", "stats-variance", "stats-stddev", "stats-correlation", "stats-regression"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "diagramming-set",
			Name:        "Diagramming",
			Description: "Tools for creating diagrams and visual representations",
			Category:    "Visualization",
			ToolIDs:     []string{"diagram-mermaid"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "physics-set",
			Name:        "Physics Tools",
			Description: "Physics problem solving tools",
			Category:    "Science",
			ToolIDs:     []string{"physics-kinematics", "physics-dynamics"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "chemistry-set",
			Name:        "Chemistry Tools",
			Description: "Chemistry problem solving tools",
			Category:    "Science",
			ToolIDs:     []string{"chem-balance", "chem-molarity"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "knowledge-base-set",
			Name:        "Knowledge Base",
			Description: "Tools for querying attached knowledge base workspaces",
			Category:    "Knowledge",
			ToolIDs:     []string{"kb-query"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	// Define default tools with their tool set associations
	defaultTools := []Tool{
		// Calculator tools
		{ID: "calc-add", Name: "Add", Description: "Add two numbers together", Category: "Calculator", ToolSetID: "calculator-set"},
		{ID: "calc-subtract", Name: "Subtract", Description: "Subtract one number from another", Category: "Calculator", ToolSetID: "calculator-set"},
		{ID: "calc-multiply", Name: "Multiply", Description: "Multiply two numbers", Category: "Calculator", ToolSetID: "calculator-set"},
		{ID: "calc-divide", Name: "Divide", Description: "Divide one number by another", Category: "Calculator", ToolSetID: "calculator-set"},
		{ID: "calc-power", Name: "Power", Description: "Raise a number to a power", Category: "Calculator", ToolSetID: "calculator-set"},
		{ID: "calc-sqrt", Name: "Square Root", Description: "Calculate the square root of a number", Category: "Calculator", ToolSetID: "calculator-set"},

		// Visualization tools
		{ID: "viz-plot", Name: "Plot", Description: "Generate a chart or plot from data", Category: "Visualization", ToolSetID: "visualization-set"},
		{ID: "viz-animate", Name: "Animate", Description: "Create animated visualizations", Category: "Visualization", ToolSetID: "visualization-set"},

		// Document management
		{ID: "doc-create", Name: "Create Document", Description: "Create a new markdown document", Category: "Documents", ToolSetID: "document-set"},
		{ID: "doc-update", Name: "Update Document", Description: "Update an existing document", Category: "Documents", ToolSetID: "document-set"},
		{ID: "doc-delete", Name: "Delete Document", Description: "Delete a document", Category: "Documents", ToolSetID: "document-set"},

		// Memory management
		{ID: "mem-remember", Name: "Remember", Description: "Store information in long-term memory", Category: "Memory", ToolSetID: "memory-set"},
		{ID: "mem-forget", Name: "Forget", Description: "Remove information from memory", Category: "Memory", ToolSetID: "memory-set"},
		{ID: "mem-list", Name: "List Memories", Description: "List all stored memories", Category: "Memory", ToolSetID: "memory-set"},
		{ID: "mem-clear", Name: "Clear Memory", Description: "Clear all stored memories", Category: "Memory", ToolSetID: "memory-set"},

		// Statistics tools (for statistics instructor)
		{ID: "stats-mean", Name: "Mean", Description: "Calculate the mean of a dataset", Category: "Statistics", ToolSetID: "statistics-set"},
		{ID: "stats-median", Name: "Median", Description: "Calculate the median of a dataset", Category: "Statistics", ToolSetID: "statistics-set"},
		{ID: "stats-mode", Name: "Mode", Description: "Calculate the mode of a dataset", Category: "Statistics", ToolSetID: "statistics-set"},
		{ID: "stats-variance", Name: "Variance", Description: "Calculate the variance of a dataset", Category: "Statistics", ToolSetID: "statistics-set"},
		{ID: "stats-stddev", Name: "Standard Deviation", Description: "Calculate the standard deviation of a dataset", Category: "Statistics", ToolSetID: "statistics-set"},
		{ID: "stats-correlation", Name: "Correlation", Description: "Calculate correlation between two variables", Category: "Statistics", ToolSetID: "statistics-set"},
		{ID: "stats-regression", Name: "Linear Regression", Description: "Perform linear regression analysis", Category: "Statistics", ToolSetID: "statistics-set"},

		// Diagramming tools
		{ID: "diagram-mermaid", Name: "Mermaid Diagram", Description: "Create diagrams using Mermaid syntax", Category: "Diagramming", ToolSetID: "diagramming-set"},

		// Physics tools
		{ID: "physics-kinematics", Name: "Kinematics", Description: "Solve kinematics problems", Category: "Physics", ToolSetID: "physics-set"},
		{ID: "physics-dynamics", Name: "Dynamics", Description: "Solve dynamics problems", Category: "Physics", ToolSetID: "physics-set"},

		// Chemistry tools
		{ID: "chem-balance", Name: "Chemical Balance", Description: "Balance chemical equations", Category: "Chemistry", ToolSetID: "chemistry-set"},
		{ID: "chem-molarity", Name: "Molarity", Description: "Calculate molarity and concentration", Category: "Chemistry", ToolSetID: "chemistry-set"},

		// Knowledge base tools
		{ID: "kb-query", Name: "Query Knowledge Base", Description: "Search and retrieve information from attached knowledge base workspaces", Category: "Knowledge", ToolSetID: "knowledge-base-set"},
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
		tool.CreatedAt = now
		tool.UpdatedAt = now

		if err := r.repo.SaveTool(ctx, &tool); err != nil {
			return fmt.Errorf("failed to save default tool %s: %w", tool.ID, err)
		}

		r.toolCache[tool.ID] = &tool
	}

	return nil
}
