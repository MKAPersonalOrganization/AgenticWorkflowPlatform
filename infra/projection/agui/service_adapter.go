package agui

import (
	"context"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ServiceAdapter wraps AguiProjection to match the services interface
// This adapter exists to decouple services from infrastructure types.
type ServiceAdapter struct {
	projection AguiProjection
}

// NewServiceAdapter creates a new adapter for services layer.
func NewServiceAdapter(proj AguiProjection) *ServiceAdapter {
	return &ServiceAdapter{projection: proj}
}

// GetThreadState returns thread state as interface{} to match services contract.
func (a *ServiceAdapter) GetThreadState(ctx context.Context, threadID string) (any, bool) {
	return a.projection.GetThreadState(ctx, threadID)
}

// GetMessages retrieves all UI-formatted messages for a thread.
func (a *ServiceAdapter) GetMessages(ctx context.Context, threadID string) []aguievents.Message {
	return a.projection.GetMessages(ctx, threadID)
}
