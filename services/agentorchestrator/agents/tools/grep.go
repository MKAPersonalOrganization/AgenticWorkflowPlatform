package tools

import (
	bookstools "school-with-ai/services/studentbooks/tools"

	"school-with-ai/foundation/agent"
)

// NewGrepTool creates a new GrepTool instance for use in academic agents.
// It strictly constrains searches to the 'books/' directory (the Academic Library).
func NewGrepTool() []agent.Tool {
	return []agent.Tool{bookstools.NewGrepTool("books")}
}
