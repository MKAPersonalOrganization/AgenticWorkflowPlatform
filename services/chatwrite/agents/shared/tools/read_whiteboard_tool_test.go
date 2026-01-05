package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestReadWhiteboardTool_Success(t *testing.T) {
	tool := NewReadWhiteboardTool()

	// Set up mock whiteboard store
	mockContent := "# Test Whiteboard\n\nThis is test content on the whiteboard."
	mockStore := &mockWhiteboardStore{
		whiteboards: map[string]string{
			"thread-123": mockContent,
		},
	}
	tool.SetWhiteboardStore(mockStore, "thread-123")

	result, err := tool.Invoke(context.Background(), "{}")
	if err != nil {
		t.Fatalf("failed to invoke read_whiteboard tool: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("expected success to be true")
	}

	if response["content"].(string) != mockContent {
		t.Errorf("expected content %q, got %q", mockContent, response["content"])
	}
}

func TestReadWhiteboardTool_NoStoreSet(t *testing.T) {
	tool := NewReadWhiteboardTool()

	// No store set - should return empty
	result, err := tool.Invoke(context.Background(), "{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "" {
		t.Errorf("expected empty result when no getter is set, got: %s", result)
	}
}

func TestReadWhiteboardTool_Name(t *testing.T) {
	tool := NewReadWhiteboardTool()
	if tool.Name() != "read_whiteboard" {
		t.Errorf("expected name 'read_whiteboard', got %q", tool.Name())
	}
}

func TestReadWhiteboardTool_Parameters(t *testing.T) {
	tool := NewReadWhiteboardTool()
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Error("expected type to be 'object'")
	}

	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("expected required to be a string array")
	}

	if len(required) != 0 {
		t.Errorf("expected no required parameters, got: %v", required)
	}
}

// mockWhiteboardStore is a simple mock implementation of WhiteboardReader
type mockWhiteboardStore struct {
	whiteboards map[string]string
}

func (m *mockWhiteboardStore) Get(threadID string) string {
	return m.whiteboards[threadID]
}

func TestReadWhiteboardTool_DifferentThreads(t *testing.T) {
	mockStore := &mockWhiteboardStore{
		whiteboards: map[string]string{
			"thread-123": "Thread 123 content",
			"thread-456": "Thread 456 content",
		},
	}

	// Test with thread-123
	tool1 := NewReadWhiteboardTool()
	tool1.SetWhiteboardStore(mockStore, "thread-123")
	result1, _ := tool1.Invoke(context.Background(), "{}")
	var response1 map[string]interface{}
	_ = json.Unmarshal([]byte(result1), &response1)

	if response1["content"].(string) != "Thread 123 content" {
		t.Errorf("thread-123: expected 'Thread 123 content', got %q", response1["content"])
	}

	// Test with thread-456
	tool2 := NewReadWhiteboardTool()
	tool2.SetWhiteboardStore(mockStore, "thread-456")
	result2, _ := tool2.Invoke(context.Background(), "{}")
	var response2 map[string]interface{}
	_ = json.Unmarshal([]byte(result2), &response2)

	if response2["content"].(string) != "Thread 456 content" {
		t.Errorf("thread-456: expected 'Thread 456 content', got %q", response2["content"])
	}
}
