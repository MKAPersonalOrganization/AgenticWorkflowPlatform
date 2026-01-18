package tools

import (
	"school-with-ai/foundation/agentsdk/core"
	sdktools "school-with-ai/foundation/agentsdk/tools"
)

// NewGrepTool creates a new GrepTool instance for use in academic agents.
// It strictly constrains searches to the 'books/' directory (the Academic Library).
func NewGrepTool() []core.Tool {
	return sdktools.NewGrepTool("books").Definition()
}
