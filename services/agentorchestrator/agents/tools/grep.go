package tools

import (
	bookstools "school-with-ai/services/studentbooks/tools"

	"school-with-ai/foundation/agentsdk/core"
)

// NewGrepTool creates a new GrepTool instance for use in academic agents.
// It strictly constrains searches to the 'books/' directory (the Academic Library).
func NewGrepTool() []core.Tool {
	return bookstools.NewGrepTool("books").Definition()
}
