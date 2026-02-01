package tools

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/services/studentbooks"
	bookstools "school-with-ai/services/studentbooks/tools"
)

var globalStudentBooks studentbooks.Service

// InitStudentBooks initializes the global student books service for tools.
func InitStudentBooks(svc studentbooks.Service) {
	globalStudentBooks = svc
}

// NewLibraryTools returns the generalized library discovery tools.
func NewLibraryTools() []core.Tool {
	return []core.Tool{
		bookstools.NewBrowseLibraryTool(globalStudentBooks),
		bookstools.NewLookupSectionTool(globalStudentBooks),
	}
}
