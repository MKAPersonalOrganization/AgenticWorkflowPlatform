package tools

import (
	"school-with-ai/foundation/agentsdk/core"
	sdktools "school-with-ai/foundation/agentsdk/tools"
	"school-with-ai/services/studentbooks"
)

var globalStudentBooks studentbooks.Service

// InitStudentBooks initializes the global student books service for tools.
func InitStudentBooks(svc studentbooks.Service) {
	globalStudentBooks = svc
}

// NewLibraryTools returns the generalized library discovery tools.
func NewLibraryTools() []core.Tool {
	return []core.Tool{
		sdktools.NewBrowseLibraryTool(globalStudentBooks),
	}
}
