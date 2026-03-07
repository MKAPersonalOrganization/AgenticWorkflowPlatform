package projections

import (
	"context"
	"errors"
	"school-with-ai/modules/chat/projection/agui"
	"school-with-ai/services"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ErrThreadNotFound is returned when a thread is not found in the projection.
var ErrThreadNotFound = errors.New("thread not found")

// UIProjectionService implements the services.ThreadQueryService interface.
// This is in the infra layer and can use concrete projection types.
// It provides read access to AGUI projection for UI queries only.
type UIProjectionService struct {
	aguiProjection agui.AguiProjection
}

// NewUIProjectionService creates a new UI projection service.
func NewUIProjectionService(
	aguiProj agui.AguiProjection,
) *UIProjectionService {
	return &UIProjectionService{
		aguiProjection: aguiProj,
	}
}

// GetThreadMessages retrieves AGUI-formatted messages for a thread.
func (s *UIProjectionService) GetThreadMessages(threadID string) []aguievents.Message {
	return s.aguiProjection.GetMessages(context.Background(), threadID)
}

// ListThreadSummaries retrieves all threads with summaries, sorted by most recent.
// Deprecated: Use ListThreads on the projection directly with pagination.
func (s *UIProjectionService) ListThreadSummaries() []services.ThreadSummary {
	// Note: This method is deprecated. The handler should call projection.ListThreads directly
	// with proper user ID from auth context. Keeping for backward compatibility.
	return []services.ThreadSummary{}
}

// ListThreadsPaginated retrieves paginated threads for a user.
func (s *UIProjectionService) ListThreadsPaginated(req agui.ThreadListRequest) (*agui.ThreadListResponse, error) {
	return s.aguiProjection.ListThreads(context.Background(), req)
}
