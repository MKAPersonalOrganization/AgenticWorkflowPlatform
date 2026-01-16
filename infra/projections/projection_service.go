package projections

import (
	"errors"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/services"
	"sort"
	"time"

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
	return s.aguiProjection.GetMessages(threadID)
}

// ListThreadSummaries retrieves all threads with summaries, sorted by most recent.
func (s *UIProjectionService) ListThreadSummaries() []services.ThreadSummary {
	threadIDs := s.aguiProjection.ListThreads()

	threads := make([]services.ThreadSummary, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		state, exists := s.aguiProjection.GetThreadState(threadID)
		if !exists {
			continue
		}

		// Determine current instructor from agent activity
		var currentInstructor string
		if len(state.AgentActivity) > 0 {
			var latestTime time.Time
			for agentName, activity := range state.AgentActivity {
				if activity.LastActiveAt.After(latestTime) {
					latestTime = activity.LastActiveAt
					currentInstructor = agentName
				}
			}
		}

		threads = append(threads, services.ThreadSummary{
			ID:                threadID,
			Title:             state.Title,
			Messages:          state.Messages,
			CreatedAt:         state.CreatedAt.Format(time.RFC3339),
			UpdatedAt:         state.UpdatedAt.Format(time.RFC3339),
			CurrentInstructor: currentInstructor,
		})
	}

	// Sort threads by UpdatedAt descending (most recent first)
	sort.Slice(threads, func(i, j int) bool {
		return threads[i].UpdatedAt > threads[j].UpdatedAt
	})

	return threads
}
