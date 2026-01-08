package projections

import (
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/infra/projection/llmcontext"
	"school-with-ai/infra/projection/whiteboard"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// projectionService implements the services.ProjectionService interface
// Services can depend on infra - this maintains clean architecture boundaries
type projectionService struct {
	aguiProjection       agui.AguiProjection
	whiteboardProjection whiteboard.WhiteboardStore
	llmContextProjection *llmcontext.InMemoryLLMContextProjection
}

// NewProjectionService creates a new projection service
func NewProjectionService(
	aguiProj agui.AguiProjection,
	whiteboardProj whiteboard.WhiteboardStore,
	llmContextProj *llmcontext.InMemoryLLMContextProjection,
) *projectionService {
	return &projectionService{
		aguiProjection:       aguiProj,
		whiteboardProjection: whiteboardProj,
		llmContextProjection: llmContextProj,
	}
}

// GetWhiteboardContent retrieves whiteboard content for a thread
func (s *projectionService) GetWhiteboardContent(threadID string) string {
	return s.whiteboardProjection.Get(threadID)
}

// GetThreadHistory retrieves formatted AGUI message history
func (s *projectionService) GetThreadHistory(threadID string) (interface{}, error) {
	state, exists := s.aguiProjection.GetThreadState(threadID)
	if !exists {
		return nil, fmt.Errorf("thread not found: %s", threadID)
	}
	return state, nil
}

// GetThread retrieves a thread by ID
func (s *projectionService) GetThread(threadID string) (*domain.Thread, bool) {
	return s.llmContextProjection.Get(threadID)
}

// GetAllThreads retrieves all threads
func (s *projectionService) GetAllThreads() []domain.Thread {
	return s.llmContextProjection.All()
}

// GetThreadMessages retrieves AGUI-formatted messages for a thread
func (s *projectionService) GetThreadMessages(threadID string) []aguievents.Message {
	return s.aguiProjection.GetMessages(threadID)
}
