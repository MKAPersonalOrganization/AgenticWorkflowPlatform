package projections

import (
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/infra/projection/whiteboard"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// LLMContextProjection defines the interface for LLM context projections
type LLMContextProjection interface {
	Get(threadID string) (*domain.Thread, bool)
	All() []domain.Thread
}

// ProjectionService implements the services.ProjectionService interface
// This is in the infra layer and can use concrete projection types
type ProjectionService struct {
	aguiProjection       agui.AguiProjection
	whiteboardProjection whiteboard.WhiteboardStore
	llmContextProjection LLMContextProjection
}

// NewProjectionService creates a new projection service
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

// GetWhiteboardContent retrieves whiteboard content for a thread
func (s *ProjectionService) GetWhiteboardContent(threadID string) string {
	return s.whiteboardProjection.Get(threadID)
}

// GetThreadHistory retrieves formatted AGUI message history
func (s *ProjectionService) GetThreadHistory(threadID string) (interface{}, error) {
	state, exists := s.aguiProjection.GetThreadState(threadID)
	if !exists {
		return nil, fmt.Errorf("thread not found: %s", threadID)
	}
	return state, nil
}

// GetThread retrieves a thread by ID
func (s *ProjectionService) GetThread(threadID string) (*domain.Thread, bool) {
	return s.llmContextProjection.Get(threadID)
}

// GetAllThreads retrieves all threads
func (s *ProjectionService) GetAllThreads() []domain.Thread {
	return s.llmContextProjection.All()
}

// GetThreadMessages retrieves AGUI-formatted messages for a thread
func (s *ProjectionService) GetThreadMessages(threadID string) []aguievents.Message {
	return s.aguiProjection.GetMessages(threadID)
}
