package taskmanagement

import (
	"context"
	"school-with-ai/domain"
	"testing"
	"time"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// mockEventEmitter implements EventEmitter for testing
type mockEventEmitter struct {
	events []any
}

func (m *mockEventEmitter) Emit(event any) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockEventEmitter) GetEvents() []any {
	return m.events
}

func (m *mockEventEmitter) GetEventTypes() []string {
	var types []string
	for _, event := range m.events {
		switch e := event.(type) {
		case *aguievents.RunStartedEvent:
			types = append(types, "RUN_STARTED")
		case *aguievents.RunFinishedEvent:
			types = append(types, "RUN_FINISHED")
		case *aguievents.CustomEvent:
			types = append(types, "CUSTOM:"+e.Name)
		default:
			types = append(types, "UNKNOWN")
		}
	}
	return types
}

// TestServiceCreation verifies that the orchestration service can be created.
func TestServiceCreation(t *testing.T) {
	t.Parallel()
	// This test ensures the Service struct can be instantiated
	service := NewService(
		nil, // log
		nil, // orchestratorService
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

// TestEventEmitterInterface verifies the EventEmitter interface works correctly.
func TestEventEmitterInterface(t *testing.T) {
	t.Parallel()
	emitter := &mockEventEmitter{}

	// Test emitting a RUN_STARTED event
	runStartedEvent := aguievents.NewRunStartedEvent("thread-123", "run-456")
	err := emitter.Emit(runStartedEvent)
	if err != nil {
		t.Fatalf("Failed to emit event: %v", err)
	}

	// Verify event was captured
	events := emitter.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	// Verify event type
	types := emitter.GetEventTypes()
	if len(types) != 1 || types[0] != "RUN_STARTED" {
		t.Errorf("Expected event type RUN_STARTED, got %v", types)
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
	service := NewService(nil, nil, nil, nil, nil, nil, nil, ObservabilityConfig{Release: "test-build-v1.0.0"})
	emitter := &mockEventEmitter{}

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
	err := service.HandleUserMessage(ctx, params, emitter)
	if err == nil {
		t.Error("Expected error for cancelled context, got nil")
	}

	if ctx.Err() == nil {
		t.Error("Context should be cancelled")
	}
}

// TestAguiEventAdapter verifies that the AGUI adapter converts domain events to AGUI events.
func TestAguiEventAdapter(t *testing.T) {
	t.Parallel()
	mockEmitter := &mockEventEmitter{}
	aguiAdapter := NewAguiEventEmitter(mockEmitter)

	// Test RunStartedEvent conversion
	domainEvent := &domain.RunStartedEvent{
		ThreadID: "thread-123",
		RunID:    "run-456",
	}
	err := aguiAdapter.Emit(domainEvent)
	if err != nil {
		t.Fatalf("Failed to emit domain event: %v", err)
	}

	// Verify AGUI event was emitted
	events := mockEmitter.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	types := mockEmitter.GetEventTypes()
	if len(types) != 1 || types[0] != "RUN_STARTED" {
		t.Errorf("Expected AGUI RUN_STARTED event, got %v", types)
	}

	// Test RunFinishedEvent conversion
	mockEmitter.events = nil // Reset
	finishedEvent := &domain.RunFinishedEvent{
		ThreadID: "thread-123",
		RunID:    "run-456",
	}
	err = aguiAdapter.Emit(finishedEvent)
	if err != nil {
		t.Fatalf("Failed to emit finished event: %v", err)
	}

	types = mockEmitter.GetEventTypes()
	if len(types) != 1 || types[0] != "RUN_FINISHED" {
		t.Errorf("Expected AGUI RUN_FINISHED event, got %v", types)
	}

	// Test RunErrorEvent conversion
	mockEmitter.events = nil // Reset
	errorEvent := &domain.RunErrorEvent{
		Message:   "Test error",
		ErrorCode: "TEST_ERROR",
	}
	err = aguiAdapter.Emit(errorEvent)
	if err != nil {
		t.Fatalf("Failed to emit error event: %v", err)
	}

	if len(mockEmitter.events) != 1 {
		t.Errorf("Expected 1 error event, got %d", len(mockEmitter.events))
	}

	// Test ThreadTitleUpdatedEvent conversion
	mockEmitter.events = nil // Reset
	titleEvent := &domain.ThreadTitleUpdatedEvent{
		ThreadID:  "thread-123",
		Title:     "New Title",
		Timestamp: time.Now(),
	}
	err = aguiAdapter.Emit(titleEvent)
	if err != nil {
		t.Fatalf("Failed to emit title event: %v", err)
	}

	types = mockEmitter.GetEventTypes()
	if len(types) != 1 || types[0] != "CUSTOM:THREAD_TITLE_UPDATED" {
		t.Errorf("Expected CUSTOM:THREAD_TITLE_UPDATED event, got %v", types)
	}
}
