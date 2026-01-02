package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestReadWhiteboardTool_Success(t *testing.T) {
	tool := NewReadWhiteboardTool()

	// Set up mock content getter
	mockContent := "# Test Whiteboard\n\nThis is test content on the whiteboard."
	tool.SetContentGetter(func() string {
		return mockContent
	})

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

	t.Logf("Whiteboard content retrieved: %s", response["content"])
}

func TestReadWhiteboardTool_EmptyContent(t *testing.T) {
	tool := NewReadWhiteboardTool()

	// Set up mock content getter returning empty string
	tool.SetContentGetter(func() string {
		return ""
	})

	result, err := tool.Invoke(context.Background(), "{}")
	if err != nil {
		t.Fatalf("failed to invoke read_whiteboard tool: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if response["content"].(string) != "" {
		t.Errorf("expected empty content, got %q", response["content"])
	}
}

func TestReadWhiteboardTool_NoGetterSet(t *testing.T) {
	tool := NewReadWhiteboardTool()

	// No getter set - should return empty
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
