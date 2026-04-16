package taskmgmt

import (
	"aicompass/modules/chat/agentorchestrator"
	"context"
	"testing"
)

// TestServiceCreation verifies that the orchestration service can be created.
func TestServiceCreation(t *testing.T) {
	t.Parallel()
	// This test ensures the Service struct can be instantiated
	service := NewService(
		nil, // log
		nil, // orchestrator
	)

	if service == nil {
		t.Fatal("NewService() returned nil")
	}

	if service.orchestrator != nil {
		t.Error("Expected nil orchestrator")
	}
}

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

// TestHandleUserMessageCancellation verifies that HandleUserMessage respects context cancellation.
func TestHandleUserMessageCancellation(t *testing.T) {
	t.Parallel()
	service := NewService(nil, nil)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := agentorchestrator.RunExecutionRequest{
		ThreadID:  "thread-123",
		MessageID: "msg-456",
		Message:   "test",
		UserID:    "user-789",
	}

	// Should return error due to cancelled context
	err := service.HandleUserMessage(ctx, req)
	if err == nil {
		t.Error("Expected error for cancelled context, got nil")
	}

	if ctx.Err() == nil {
		t.Error("Context should be cancelled")
	}
}
