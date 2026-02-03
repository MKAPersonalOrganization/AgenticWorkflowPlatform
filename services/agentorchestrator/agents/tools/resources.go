package tools

import (
	"school-with-ai/foundation/agent"
	"school-with-ai/services/studentbooks"
	bookstools "school-with-ai/services/studentbooks/tools"
)

var globalStudentBooks studentbooks.Service

// InitStudentBooks initializes the global student books service for tools.
func InitStudentBooks(svc studentbooks.Service) {
	globalStudentBooks = svc
}

// NewLibraryTools returns the generalized library discovery tools.
func NewLibraryTools() []agent.Tool {
	return []agent.Tool{
		bookstools.NewBrowseLibraryTool(globalStudentBooks),
		bookstools.NewLookupSectionTool(globalStudentBooks),
	}
}
