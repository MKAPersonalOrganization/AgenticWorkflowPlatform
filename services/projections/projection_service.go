package projections

import (
	"fmt"
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

// projectionService implements the services.ProjectionService interface
type projectionService struct {
	aguiProjection       AguiProjection
	whiteboardProjection WhiteboardReader
}

// NewProjectionService creates a new projection service
func NewProjectionService(
	aguiProj AguiProjection,
	whiteboardProj WhiteboardReader,
) *projectionService {
	return &projectionService{
		aguiProjection:       aguiProj,
		whiteboardProjection: whiteboardProj,
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
