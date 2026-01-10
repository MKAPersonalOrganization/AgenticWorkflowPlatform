package projections

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"school-with-ai/domain"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/infra/projection/whiteboard"
	"school-with-ai/services"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ErrThreadNotFound is returned when a thread is not found in the projection.
var ErrThreadNotFound = errors.New("thread not found")

// LLMContextProjection defines the interface for LLM context projections.
type LLMContextProjection interface {
	Get(threadID string) (*domain.Thread, bool)
	All() []domain.Thread
}

// ProjectionService implements the services.ProjectionService interface
// This is in the infra layer and can use concrete projection types.
type ProjectionService struct {
	aguiProjection       agui.AguiProjection
	whiteboardProjection whiteboard.WhiteboardStore
	llmContextProjection LLMContextProjection
}

// NewProjectionService creates a new projection service.
func NewProjectionService(
	aguiProj agui.AguiProjection,
	whiteboardProj whiteboard.WhiteboardStore,
	llmContextProj LLMContextProjection,
) *ProjectionService {
	return &ProjectionService{
		aguiProjection:       aguiProj,
		whiteboardProjection: whiteboardProj,
		llmContextProjection: llmContextProj,
	}
}

// GetWhiteboardContent retrieves whiteboard content for a thread.
func (s *ProjectionService) GetWhiteboardContent(threadID string) string {
	return s.whiteboardProjection.Get(threadID)
}

// GetThreadHistory retrieves formatted AGUI message history.
func (s *ProjectionService) GetThreadHistory(threadID string) (any, error) {
	state, exists := s.aguiProjection.GetThreadState(threadID)
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrThreadNotFound, threadID)
	}

	return state, nil
}

// GetThread retrieves a thread by ID.
func (s *ProjectionService) GetThread(threadID string) (*domain.Thread, bool) {
	return s.llmContextProjection.Get(threadID)
}

// GetAllThreads retrieves all threads.
func (s *ProjectionService) GetAllThreads() []domain.Thread {
	return s.llmContextProjection.All()
}

// GetThreadMessages retrieves AGUI-formatted messages for a thread.
func (s *ProjectionService) GetThreadMessages(threadID string) []aguievents.Message {
	return s.aguiProjection.GetMessages(threadID)
}

// ListThreadSummaries retrieves all threads with summaries, sorted by most recent.
func (s *ProjectionService) ListThreadSummaries() []services.ThreadSummary {
	threadIDs := s.aguiProjection.ListThreads()

	threads := make([]services.ThreadSummary, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		state, exists := s.aguiProjection.GetThreadState(threadID)
		if !exists {
			continue
		}

		// Get whiteboard content
		whiteboardContent := s.whiteboardProjection.Get(threadID)

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
			WhiteboardContent: whiteboardContent,
			CurrentInstructor: currentInstructor,
		})
	}

	// Sort threads by UpdatedAt descending (most recent first)
	sort.Slice(threads, func(i, j int) bool {
		return threads[i].UpdatedAt > threads[j].UpdatedAt
	})

	return threads
}
