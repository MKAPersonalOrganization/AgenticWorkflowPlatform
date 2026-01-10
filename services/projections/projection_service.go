package projections

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"school-with-ai/domain"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/services"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

var ErrThreadNotFound = errors.New("thread not found")

// WhiteboardReader provides read access to whiteboard content.
type WhiteboardReader interface {
	Get(threadID string) string
}

// AguiProjection provides access to AGUI thread state.
type AguiProjection interface {
	GetThreadState(threadID string) (*agui.AguiThreadState, bool)
	GetMessages(threadID string) []aguievents.Message
	ListThreads() []string
}

// LLMContextReader provides read access to LLM context projections.
type LLMContextReader interface {
	Get(threadID string) (*domain.Thread, bool)
	All() []domain.Thread
}

// projectionService implements the services.ProjectionService interface.
type projectionService struct {
	aguiProjection       AguiProjection
	whiteboardProjection WhiteboardReader
	llmContextProjection LLMContextReader
}

// NewProjectionService creates a new projection service.
func NewProjectionService(
	aguiProj AguiProjection,
	whiteboardProj WhiteboardReader,
	llmContextProj LLMContextReader,
) *projectionService {
	return &projectionService{
		aguiProjection:       aguiProj,
		whiteboardProjection: whiteboardProj,
		llmContextProjection: llmContextProj,
	}
}

// GetWhiteboardContent retrieves whiteboard content for a thread.
func (s *projectionService) GetWhiteboardContent(threadID string) string {
	return s.whiteboardProjection.Get(threadID)
}

// GetThreadHistory retrieves formatted AGUI message history.
func (s *projectionService) GetThreadHistory(threadID string) (any, error) {
	state, exists := s.aguiProjection.GetThreadState(threadID)
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrThreadNotFound, threadID)
	}

	return state, nil
}

// GetThread retrieves a thread by ID.
func (s *projectionService) GetThread(threadID string) (*domain.Thread, bool) {
	return s.llmContextProjection.Get(threadID)
}

// GetAllThreads retrieves all threads.
func (s *projectionService) GetAllThreads() []domain.Thread {
	return s.llmContextProjection.All()
}

// GetThreadMessages retrieves AGUI-formatted messages for a thread.
func (s *projectionService) GetThreadMessages(threadID string) []aguievents.Message {
	return s.aguiProjection.GetMessages(threadID)
}

// ListThreadSummaries retrieves all threads with summaries, sorted by most recent.
func (s *projectionService) ListThreadSummaries() []services.ThreadSummary {
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
