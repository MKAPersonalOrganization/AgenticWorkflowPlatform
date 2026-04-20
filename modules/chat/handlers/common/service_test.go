package common

import (
	"aicompass/modules/chat/agentorchestrator"
	"testing"
)

// TestRunExecutionRequestValidation verifies that RunExecutionRequest fields are set correctly.
func TestRunExecutionRequestValidation(t *testing.T) {
	t.Parallel()
	req := agentorchestrator.RunExecutionRequest{
		ThreadID:  "thread-123",
		MessageID: "msg-456",
		Message:   "Hello, world!",
		UserID:    "user-789",
	}

	if req.ThreadID != "thread-123" {
		t.Errorf("Expected ThreadID 'thread-123', got '%s'", req.ThreadID)
	}

	if req.Message != "Hello, world!" {
		t.Errorf("Expected Message 'Hello, world!', got '%s'", req.Message)
	}
}
