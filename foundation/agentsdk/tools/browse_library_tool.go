package tools

import (
	"fmt"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/services/studentbooks"
	"strings"
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

// Parameters returns the JSON schema for the tool's parameters.
func (t *BrowseLibraryTool) Parameters() map[string]any {
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

// Invoke executes the tool with the given ToolContext and typed arguments.
func (t *BrowseLibraryTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params BrowseLibraryArgs
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse browse_library args: %w", err)
	}

	nodes, err := t.books.ListNodes(params.Path)
	if err != nil {
		return map[string]any{
			"status":  "error",
			"message": fmt.Sprintf("Failed to browse path '%s': %v", params.Path, err),
		}, nil
	}

	var sb strings.Builder
	if params.Path == "" || params.Path == "." {
		sb.WriteString("Academic Library (Root):\n\n")
	} else {
		// Detect if this is a resource map (headings) or directory listing
		isResourceMap := false
		for _, node := range nodes {
			if strings.HasPrefix(string(node.Type), "heading-") {
				isResourceMap = true
				break
			}
		}

		if isResourceMap {
			sb.WriteString(fmt.Sprintf("Resource Map for '%s':\n\n", params.Path))
		} else {
			sb.WriteString(fmt.Sprintf("Contents of '%s':\n\n", params.Path))
		}
	}

	for _, node := range nodes {
		if strings.HasPrefix(string(node.Type), "heading-") {
			// Prettify resource map
			level := 0
			fmt.Sscanf(string(node.Type), "heading-%d", &level)
			indent := strings.Repeat("  ", level-1)
			sb.WriteString(fmt.Sprintf("%s- %s\n", indent, node.Name))
		} else {
			sb.WriteString(fmt.Sprintf("- [%s] %s (Path: %s)\n", node.Type, node.Name, node.Path))
		}
	}

	if len(nodes) == 0 {
		sb.WriteString("No items found at this location.")
	}

	return map[string]any{
		"content": sb.String(),
		"nodes":   nodes,
	}, nil
}
