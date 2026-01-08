package projections

import (
	"fmt"
	"school-with-ai/domain"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// WhiteboardReader provides read access to whiteboard content.
type WhiteboardReader interface {
	Get(threadID string) string
}

// AguiProjection provides access to AGUI thread state.
type AguiProjection interface {
	GetThreadState(threadID string) (any, bool)
	GetMessages(threadID string) []aguievents.Message
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
		return nil, fmt.Errorf("thread not found: %s", threadID)
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
