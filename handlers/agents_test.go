package handlers_test

import (
	"testing"

	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents/math"
	"school-with-ai/services/chatwrite/agents/studentfacing"
)

func TestInstructorConfigurations(t *testing.T) {
	t.Parallel()
	// Mock notifier
	var mockNotifier core.EventNotifier = nil
	// Mock whiteboard projection
	mockWhiteboardProjection := &mockWhiteboardProjection{data: make(map[string]string)}

	studentAgent := studentfacing.CreateStudentFacingAgent(mockNotifier, mockWhiteboardProjection)
	if studentAgent.Name != "student-facing-agent" {
		t.Errorf("Expected student agent name 'student-facing-agent', got '%s'", studentAgent.Name)
	}

	mathAgent := math.CreateMathInstructor(mockNotifier, mockWhiteboardProjection)
	if mathAgent.Name != "math-instructor" {
		t.Errorf("Expected math name 'math-instructor', got '%s'", mathAgent.Name)
	}
}

// mockWhiteboardProjection implements the whiteboard projection interface for testing.
type mockWhiteboardProjection struct {
	data map[string]string
}

func (m *mockWhiteboardProjection) Get(threadID string) string {
	if m.data == nil {
		return ""
	}

	return m.data[threadID]
}

func (m *mockWhiteboardProjection) GetAll() map[string]string {
	if m.data == nil {
		return make(map[string]string)
	}

	return m.data
}

func (m *mockWhiteboardProjection) Update(event any) error {
	return nil
}
