package whiteboard

import (
	"encoding/json"
	"testing"

	"school-with-ai/sdk/agentsdk/core"
)

func TestWhiteboardProjection_Write(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()
	threadID := "thread-123"

	args, _ := json.Marshal(map[string]interface{}{
		"action":  "write",
		"content": "Hello, World!",
	})
	event := &core.ToolCallExecutedEvent{
		BaseEvent: core.BaseEvent{
			Type:     "ToolCallExecuted",
			ThreadID: threadID,
		},
		ToolName:      "write_whiteboard",
		ToolArguments: string(args),
		AgentName:     "test-agent",
	}
	err := store.Update(event)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	content := store.Get(threadID)
	if content != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", content)
	}
}

func TestWhiteboardProjection_Append(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()
	threadID := "thread-123"

	args1, _ := json.Marshal(map[string]interface{}{"action": "write", "content": "First line"})
	event1 := &core.ToolCallExecutedEvent{
		BaseEvent:     core.BaseEvent{Type: "ToolCallExecuted", ThreadID: threadID},
		ToolName:      "write_whiteboard",
		ToolArguments: string(args1),
		AgentName:     "test-agent",
	}
	_ = store.Update(event1)

	args2, _ := json.Marshal(map[string]interface{}{"action": "append", "content": "Second line"})
	event2 := &core.ToolCallExecutedEvent{
		BaseEvent:     core.BaseEvent{Type: "ToolCallExecuted", ThreadID: threadID},
		ToolName:      "write_whiteboard",
		ToolArguments: string(args2),
		AgentName:     "test-agent",
	}
	_ = store.Update(event2)

	content := store.Get(threadID)
	expected := "First line\nSecond line"
	if content != expected {
		t.Errorf("Expected '%s', got '%s'", expected, content)
	}
}

func TestWhiteboardProjection_AppendToEmpty(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()
	threadID := "thread-123"

	args, _ := json.Marshal(map[string]interface{}{"action": "append", "content": "First line"})
	event := &core.ToolCallExecutedEvent{
		BaseEvent:     core.BaseEvent{Type: "ToolCallExecuted", ThreadID: threadID},
		ToolName:      "write_whiteboard",
		ToolArguments: string(args),
		AgentName:     "test-agent",
	}
	_ = store.Update(event)

	content := store.Get(threadID)
	if content != "First line" {
		t.Errorf("Expected 'First line', got '%s'", content)
	}
}

func TestWhiteboardProjection_Clear(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()
	threadID := "thread-123"

	args1, _ := json.Marshal(map[string]interface{}{"action": "write", "content": "Some content"})
	event1 := &core.ToolCallExecutedEvent{
		BaseEvent:     core.BaseEvent{Type: "ToolCallExecuted", ThreadID: threadID},
		ToolName:      "write_whiteboard",
		ToolArguments: string(args1),
		AgentName:     "test-agent",
	}
	_ = store.Update(event1)

	args2, _ := json.Marshal(map[string]interface{}{"action": "clear", "content": ""})
	event2 := &core.ToolCallExecutedEvent{
		BaseEvent:     core.BaseEvent{Type: "ToolCallExecuted", ThreadID: threadID},
		ToolName:      "write_whiteboard",
		ToolArguments: string(args2),
		AgentName:     "test-agent",
	}
	_ = store.Update(event2)

	content := store.Get(threadID)
	if content != "" {
		t.Errorf("Expected empty string, got '%s'", content)
	}
}

func TestWhiteboardProjection_MultipleThreads(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()

	args1, _ := json.Marshal(map[string]interface{}{"action": "write", "content": "Thread 1 content"})
	event1 := &core.ToolCallExecutedEvent{
		BaseEvent:     core.BaseEvent{Type: "ToolCallExecuted", ThreadID: "thread-1"},
		ToolName:      "write_whiteboard",
		ToolArguments: string(args1),
		AgentName:     "agent-1",
	}
	_ = store.Update(event1)

	args2, _ := json.Marshal(map[string]interface{}{"action": "write", "content": "Thread 2 content"})
	event2 := &core.ToolCallExecutedEvent{
		BaseEvent:     core.BaseEvent{Type: "ToolCallExecuted", ThreadID: "thread-2"},
		ToolName:      "write_whiteboard",
		ToolArguments: string(args2),
		AgentName:     "agent-2",
	}
	_ = store.Update(event2)

	content1 := store.Get("thread-1")
	if content1 != "Thread 1 content" {
		t.Errorf("Thread 1: Expected 'Thread 1 content', got '%s'", content1)
	}

	content2 := store.Get("thread-2")
	if content2 != "Thread 2 content" {
		t.Errorf("Thread 2: Expected 'Thread 2 content', got '%s'", content2)
	}

	all := store.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 whiteboards, got %d", len(all))
	}
}

func TestWhiteboardProjection_NonExistentThread(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()

	content := store.Get("non-existent")
	if content != "" {
		t.Errorf("Expected empty string for non-existent thread, got '%s'", content)
	}
}

func TestWhiteboardProjection_IgnoreNonWhiteboardEvents(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()
	threadID := "thread-123"

	// Non-whiteboard tool event
	nonWhiteboardEvent := &core.ToolCallExecutedEvent{
		BaseEvent:     core.BaseEvent{Type: "ToolCallExecuted", ThreadID: threadID},
		ToolName:      "some_other_tool",
		ToolArguments: `{"content": "This should be ignored"}`,
		AgentName:     "test-agent",
	}

	err := store.Update(nonWhiteboardEvent)
	if err != nil {
		t.Fatalf("Update should not fail for non-whiteboard events: %v", err)
	}

	content := store.Get(threadID)
	if content != "" {
		t.Errorf("Expected empty string, got '%s'", content)
	}
}

func TestWhiteboardProjection_UnknownAction(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()
	threadID := "thread-123"

	args1, _ := json.Marshal(map[string]interface{}{"action": "write", "content": "Initial content"})
	event1 := &core.ToolCallExecutedEvent{
		BaseEvent:     core.BaseEvent{Type: "ToolCallExecuted", ThreadID: threadID},
		ToolName:      "write_whiteboard",
		ToolArguments: string(args1),
		AgentName:     "test-agent",
	}
	_ = store.Update(event1)

	args2, _ := json.Marshal(map[string]interface{}{"action": "unknown_action", "content": "This should be ignored"})
	event2 := &core.ToolCallExecutedEvent{
		BaseEvent:     core.BaseEvent{Type: "ToolCallExecuted", ThreadID: threadID},
		ToolName:      "write_whiteboard",
		ToolArguments: string(args2),
		AgentName:     "test-agent",
	}
	_ = store.Update(event2)

	content := store.Get(threadID)
	if content != "Initial content" {
		t.Errorf("Expected 'Initial content', got '%s'", content)
	}
}
