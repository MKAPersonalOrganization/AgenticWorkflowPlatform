package projections

import (
	"fmt"

	"school-with-ai/infra/projection/agui"
	"school-with-ai/infra/projection/whiteboard"
)

// projectionService implements the services.ProjectionService interface
type projectionService struct {
	aguiProjection       agui.AguiProjection
	whiteboardProjection whiteboard.WhiteboardStore
}

// NewProjectionService creates a new projection service
func NewProjectionService(
	aguiProj agui.AguiProjection,
	whiteboardProj whiteboard.WhiteboardStore,
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
func (s *projectionService) GetAguiProjection() agui.AguiProjection {
	return s.aguiProjection
}

// GetWhiteboardProjection returns the underlying whiteboard projection
// This is used by services that need direct access to projection
func (s *projectionService) GetWhiteboardProjection() whiteboard.WhiteboardStore {
	return s.whiteboardProjection
}
