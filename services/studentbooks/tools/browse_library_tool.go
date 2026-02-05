package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"school-with-ai/foundation/agent"
	"school-with-ai/services/studentbooks"
)

// BrowseLibraryArgs represents the arguments for the browse_library tool.
type BrowseLibraryArgs struct {
	// The path to browse (e.g., 'grade-10' or 'grade-12/Think-Stats').
	// If empty, returns available grade levels.
	Path string `json:"path,omitempty"`
}

// BrowseLibraryTool provides hierarchical navigation of the academic library.
type BrowseLibraryTool struct {
	books studentbooks.Service
}

// NewBrowseLibraryTool creates a new BrowseLibraryTool instance.
func NewBrowseLibraryTool(books studentbooks.Service) *BrowseLibraryTool {
	return &BrowseLibraryTool{
		books: books,
	}
}

// Name returns the unique name of the tool.
func (t *BrowseLibraryTool) Name() string {
	return "browse_library"
}

// Description returns a human-readable description of what the tool does.
func (t *BrowseLibraryTool) Description() string {
	return "Browses the academic library tree. Use it to discover grade levels, then books, then chapters. Provide a path or leave empty to start at the root."
}

// Schema returns the JSON schema for the tool's parameters.
func (t *BrowseLibraryTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: agent.TypeObject,
		Properties: map[string]*agent.Schema{
			"path": {
				Type:        agent.TypeString,
				Description: "Path to a directory (e.g., 'university' or 'university/Think-Stats'). Leave empty for root.",
			},
		},
	}
}

// Execute executes the tool with the given arguments.
func (t *BrowseLibraryTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params BrowseLibraryArgs
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse browse_library args: %w", err)), nil
	}

	nodes, err := t.books.ListNodes(params.Path)
	if err != nil {
		return agent.JSONResult(map[string]any{
			"status":  "error",
			"message": fmt.Sprintf("Failed to browse path '%s': %v", params.Path, err),
		}), nil
	}

	return agent.JSONResult(map[string]any{
		"nodes": nodes,
	}), nil
}

func jsonResult(v any) agent.Result {
	b, _ := json.Marshal(v)
	return agent.TextResult(string(b))
}
