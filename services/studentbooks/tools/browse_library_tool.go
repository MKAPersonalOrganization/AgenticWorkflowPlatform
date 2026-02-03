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

// InputSchema returns the JSON schema for the tool's parameters.
func (t *BrowseLibraryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to a directory (e.g., 'university' or 'university/Think-Stats'). Leave empty for root.",
			},
		},
	}
}

// Execute executes the tool with the given ToolContext and typed arguments.
func (t *BrowseLibraryTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params BrowseLibraryArgs
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("failed to parse browse_library args: %w", err)
	}

	nodes, err := t.books.ListNodes(params.Path)
	if err != nil {
		return jsonResult(map[string]any{
			"status":  "error",
			"message": fmt.Sprintf("Failed to browse path '%s': %v", params.Path, err),
		})
	}

	return jsonResult(map[string]any{
		"nodes": nodes,
	})
}

func jsonResult(v any) (*agent.ToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &agent.ToolResult{Output: string(b)}, nil
}
