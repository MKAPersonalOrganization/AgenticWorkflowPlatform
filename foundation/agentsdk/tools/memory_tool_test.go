package tools

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/services/artifact"
)

// MockArtifactService for testing
type MockArtifactService struct {
	files map[string][]byte
}

func NewMockArtifactService() *MockArtifactService {
	return &MockArtifactService{files: make(map[string][]byte)}
}

func (m *MockArtifactService) Save(ctx context.Context, userID, sessionID, fileName string, content io.Reader) (int64, error) {
	data, _ := io.ReadAll(content)
	key := userID + "/" + sessionID + "/" + fileName
	m.files[key] = data
	return int64(len(data)), nil
}

func (m *MockArtifactService) Load(ctx context.Context, userID, sessionID, fileName string) (io.ReadCloser, error) {
	key := userID + "/" + sessionID + "/" + fileName
	data, ok := m.files[key]
	if !ok {
		return nil, artifact.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *MockArtifactService) List(ctx context.Context, userID, sessionID string) ([]string, error) {
	return nil, nil
}
func (m *MockArtifactService) Delete(ctx context.Context, userID, sessionID, fileName string) error {
	return nil
}

func TestMemoryTool_SaveMemory(t *testing.T) {
	store := NewMockArtifactService()
	tool := NewMemoryTool(store)
	
	// Create context with user
	user := &domain.User{ID: "test-user-123"}
	ctx := context.WithValue(context.Background(), domain.UserContextKey, user)

	// 1. Save new memory
	args := map[string]any{"fact": "My favorite color is blue."}
	_, err := tool.Definition()[0].Invoke(core.ToolContext{Context: ctx}, args)
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}

	// Verify content - should be saved under user ID
	content, err := tool.LoadMemory(ctx)
	if err != nil {
		t.Fatalf("LoadMemory failed: %v", err)
	}
	if !strings.Contains(content, "## SchoolAI Memories") {
		t.Error("Missing header")
	}
	if !strings.Contains(content, "- My favorite color is blue.") {
		t.Error("Missing fact")
	}
	
	// Verify it used the correct path in mock store
	key := "test-user-123/user_shared/SCHOOLAI.md"
	if _, ok := store.files[key]; !ok {
		t.Error("File NOT saved under correct user path")
	}

	// 2. Append memory
	args2 := map[string]any{"fact": "I like pizza."}
	_, err = tool.Definition()[0].Invoke(core.ToolContext{Context: ctx}, args2)
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}

	// Verify content again
	content, err = tool.LoadMemory(ctx)
	if err != nil {
		t.Fatalf("LoadMemory failed: %v", err)
	}
	
	if !strings.Contains(content, "- My favorite color is blue.") {
		t.Error("Missing first fact")
	}
	if !strings.Contains(content, "- I like pizza.") {
		t.Error("Missing second fact")
	}
	// Verify header appears only once
	if strings.Count(content, "## SchoolAI Memories") != 1 {
		t.Errorf("Header appeared %d times", strings.Count(content, "## SchoolAI Memories"))
	}
}

func TestMemoryTool_ComputeNewContent(t *testing.T) {
	tool := &MemoryTool{}
	
	t.Run("Empty Content", func(t *testing.T) {
		res := tool.computeNewContent("", "Fact 1")
		if !strings.Contains(res, "## SchoolAI Memories") {
			t.Error("Missing header")
		}
		if !strings.Contains(res, "- Fact 1") {
			t.Error("Missing fact")
		}
	})

	t.Run("Existing Content with Header", func(t *testing.T) {
		initial := "## SchoolAI Memories\n- Old Fact\n"
		res := tool.computeNewContent(initial, "New Fact")
		
		expectedLines := []string{
			"## SchoolAI Memories",
			"- Old Fact", 
			"- New Fact",
		}
		for _, line := range expectedLines {
			if !strings.Contains(res, line) {
				t.Errorf("Missing expected content: %s", line)
			}
		}
		// Basic check for newlines
		if !strings.Contains(res, "\n- New Fact\n") {
			t.Errorf("Format issue: %q", res)
		}
	})
	
	t.Run("Existing Content without Header/New Section", func(t *testing.T) {
		initial := "Some existing system prompt content."
		res := tool.computeNewContent(initial, "New Fact")
		
		if !strings.Contains(res, "Some existing system prompt content.") {
			t.Error("Lost original content")
		}
		if !strings.Contains(res, "\n\n## SchoolAI Memories") {
			t.Error("Missing header with newline separation")
		}
	})
}
