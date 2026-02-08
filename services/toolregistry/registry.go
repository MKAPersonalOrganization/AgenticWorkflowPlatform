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
}

// Registry manages tools and their configurations per agent.
type Registry struct {
	log       *logger.Logger
	repo      Repository
	mutex     sync.RWMutex
	toolCache map[string]*Tool // toolID -> Tool
}

// NewRegistry creates a new tool registry.
func NewRegistry(log *logger.Logger, repo Repository) *Registry {
	return &Registry{
		log:       log,
		repo:      repo,
		toolCache: make(map[string]*Tool),
	}
}

// Initialize loads default tools and agent configurations.
func (r *Registry) Initialize(ctx context.Context) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Load default tools if registry is empty
	tools, err := r.repo.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to load tools: %w", err)
	}

	if len(tools) == 0 {
		r.log.Info(ctx, "No tools found, initializing default tools")
		if err := r.initializeDefaultTools(ctx); err != nil {
			return fmt.Errorf("failed to initialize default tools: %w", err)
		}
	} else {
		// Cache existing tools
		for _, tool := range tools {
			r.toolCache[tool.ID] = &tool
		}
	}

	r.log.Info(ctx, "Tool registry initialized", "tool_count", len(r.toolCache))
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

// initializeDefaultTools creates the initial set of tools.
func (r *Registry) initializeDefaultTools(ctx context.Context) error {
	defaultTools := []Tool{
		// Calculator tools
		{ID: "calc-add", Name: "Add", Description: "Add two numbers together", Category: "Calculator"},
		{ID: "calc-subtract", Name: "Subtract", Description: "Subtract one number from another", Category: "Calculator"},
		{ID: "calc-multiply", Name: "Multiply", Description: "Multiply two numbers", Category: "Calculator"},
		{ID: "calc-divide", Name: "Divide", Description: "Divide one number by another", Category: "Calculator"},
		{ID: "calc-power", Name: "Power", Description: "Raise a number to a power", Category: "Calculator"},
		{ID: "calc-sqrt", Name: "Square Root", Description: "Calculate the square root of a number", Category: "Calculator"},

		// Visualization tools
		{ID: "viz-plot", Name: "Plot", Description: "Generate a chart or plot from data", Category: "Visualization"},
		{ID: "viz-animate", Name: "Animate", Description: "Create animated visualizations", Category: "Visualization"},

		// Document management
		{ID: "doc-create", Name: "Create Document", Description: "Create a new markdown document", Category: "Documents"},
		{ID: "doc-update", Name: "Update Document", Description: "Update an existing document", Category: "Documents"},
		{ID: "doc-delete", Name: "Delete Document", Description: "Delete a document", Category: "Documents"},

		// Memory management
		{ID: "mem-remember", Name: "Remember", Description: "Store information in long-term memory", Category: "Memory"},
		{ID: "mem-forget", Name: "Forget", Description: "Remove information from memory", Category: "Memory"},
		{ID: "mem-list", Name: "List Memories", Description: "List all stored memories", Category: "Memory"},
		{ID: "mem-clear", Name: "Clear Memory", Description: "Clear all stored memories", Category: "Memory"},

		// Statistics tools (for statistics instructor)
		{ID: "stats-mean", Name: "Mean", Description: "Calculate the mean of a dataset", Category: "Statistics"},
		{ID: "stats-median", Name: "Median", Description: "Calculate the median of a dataset", Category: "Statistics"},
		{ID: "stats-mode", Name: "Mode", Description: "Calculate the mode of a dataset", Category: "Statistics"},
		{ID: "stats-variance", Name: "Variance", Description: "Calculate the variance of a dataset", Category: "Statistics"},
		{ID: "stats-stddev", Name: "Standard Deviation", Description: "Calculate the standard deviation of a dataset", Category: "Statistics"},
		{ID: "stats-correlation", Name: "Correlation", Description: "Calculate correlation between two variables", Category: "Statistics"},
		{ID: "stats-regression", Name: "Linear Regression", Description: "Perform linear regression analysis", Category: "Statistics"},

		// Diagramming tools
		{ID: "diagram-mermaid", Name: "Mermaid Diagram", Description: "Create diagrams using Mermaid syntax", Category: "Diagramming"},

		// Physics tools
		{ID: "physics-kinematics", Name: "Kinematics", Description: "Solve kinematics problems", Category: "Physics"},
		{ID: "physics-dynamics", Name: "Dynamics", Description: "Solve dynamics problems", Category: "Physics"},

		// Chemistry tools
		{ID: "chem-balance", Name: "Chemical Balance", Description: "Balance chemical equations", Category: "Chemistry"},
		{ID: "chem-molarity", Name: "Molarity", Description: "Calculate molarity and concentration", Category: "Chemistry"},
	}

	now := time.Now()
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
