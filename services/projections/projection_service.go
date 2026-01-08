package projections

import (
	"fmt"

	"school-with-ai/domain"
)

// WhiteboardReader provides read access to whiteboard content
type WhiteboardReader interface {
	Get(threadID string) string
}

// AguiProjection provides access to AGUI thread state
// The specific type returned is implementation-dependent
type AguiProjection interface {
	GetThreadState(threadID string) (interface{}, bool)
}

// LLMContextReader provides read access to LLM context projections
type LLMContextReader interface {
	Get(threadID string) (*domain.Thread, bool)
	All() []domain.Thread
}

// projectionService implements the services.ProjectionService interface
type projectionService struct {
	aguiProjection       AguiProjection
	whiteboardProjection WhiteboardReader
	llmContextProjection LLMContextReader
}

// NewProjectionService creates a new projection service
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

// GetWhiteboardContent retrieves whiteboard content for a thread
func (s *projectionService) GetWhiteboardContent(threadID string) string {
	return s.whiteboardProjection.Get(threadID)
}

// GetThreadHistory retrieves formatted AGUI message history
func (s *projectionService) GetThreadHistory(threadID string) (interface{}, error) {
	// Return the thread state which includes messages
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

// GetAguiProjection returns the underlying AGUI projection
// This is used by services that need direct access to projection
func (s *projectionService) GetAguiProjection() AguiProjection {
	return s.aguiProjection
}

// GetWhiteboardProjection returns the underlying whiteboard projection
// This is used by services that need direct access to projection
func (s *projectionService) GetWhiteboardProjection() WhiteboardReader {
	return s.whiteboardProjection
}
