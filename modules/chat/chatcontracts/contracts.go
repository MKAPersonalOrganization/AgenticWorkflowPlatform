// Package chatcontracts defines shared interfaces and types used across
// the chat module's sub-packages. It exists as a separate sub-package to
// avoid circular imports between modules/chat and its children.
package chatcontracts

import (
	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ThreadQueryService provides read access to projections (queries) for UI.
type ThreadQueryService interface {
	// GetThreadMessages retrieves AGUI-formatted messages for a thread
	GetThreadMessages(threadID string) []aguievents.Message
}

// ThreadSummary represents a summary of a thread for list views.
type ThreadSummary struct {
	ID           string               `json:"id"`
	Title        string               `json:"title"`
	Messages     []aguievents.Message `json:"messages"`
	CreatedAt    string               `json:"createdAt"`
	UpdatedAt    string               `json:"updatedAt"`
	CurrentAgent string               `json:"currentAgent,omitempty"`
}
