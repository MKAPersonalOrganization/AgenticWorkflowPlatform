package taskmanagement

import (
	"context"
	"testing"
)

// TestServiceCreation verifies that the orchestration service can be created.
func TestServiceCreation(t *testing.T) {
	t.Parallel()
	// This test ensures the Service struct can be instantiated
	service := NewService(
		nil, // log
		nil, // orchestratorService
		nil, // threadCommandService
		nil, // agentFactory
		nil, // threadStore
		nil, // eventLog
		nil, // chatClient
		nil, // langfuseTracer
		ObservabilityConfig{ // observabilityConfig
			Release: "test-build-v1.0.0",
			Env:     "test",
		},
	)

	if service == nil {
		t.Fatal("NewService() returned nil")
	}

	if service.orchestratorService != nil {
		t.Error("Expected nil orchestratorService")
	}
}

// TestUserMessageParamsValidation verifies that UserMessageParams is created correctly.
func TestUserMessageParamsValidation(t *testing.T) {
	t.Parallel()
	params := UserMessageParams{
		ThreadID:  "thread-123",
		MessageID: "msg-456",
		Content:   "Hello, world!",
		UserID:    "user-789",
	}

	if params.ThreadID != "thread-123" {
		t.Errorf("Expected ThreadID 'thread-123', got '%s'", params.ThreadID)
	}

	if params.Content != "Hello, world!" {
		t.Errorf("Expected Content 'Hello, world!', got '%s'", params.Content)
	}
}

// TestHandleUserMessageCancellation verifies that HandleUserMessage respects context cancellation.
func TestHandleUserMessageCancellation(t *testing.T) {
	t.Parallel()
	service := NewService(nil, nil, nil, nil, nil, nil, nil, nil, ObservabilityConfig{Release: "test-build-v1.0.0"})

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	params := UserMessageParams{
		ThreadID:  "thread-123",
		MessageID: "msg-456",
		Content:   "test",
		UserID:    "user-789",
	}

	// Should return error due to cancelled context
	err := service.HandleUserMessage(ctx, params)
	if err == nil {
		t.Error("Expected error for cancelled context, got nil")
	}

	if ctx.Err() == nil {
		t.Error("Context should be cancelled")
	}
}
