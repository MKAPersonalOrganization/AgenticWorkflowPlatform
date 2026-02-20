package memory

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
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

func TestMemoryTool_Integration(t *testing.T) {
	t.Parallel()
	store := NewMockArtifactService()
	saveTool := NewMemoryTool(store)
	forgetTool := NewForgetMemoryTool(store)

	// Create context with user
	user := &domain.User{ID: "test-user-123"}
	ctx := context.WithValue(context.Background(), domain.UserContextKey, user)

	// 1. Save new memory
	args := agent.Arguments{
		Raw:    []byte(`{"fact":"My favorite color is blue."}`),
		CallID: "test-call-1",
	}

	res, err := saveTool.Execute(ctx, args)
	if err != nil || res.Error != nil {
		t.Fatalf("Save Execute failed: %v", err)
	}

	// 2. Verify saved
	content, _ := saveTool.mgr.load(ctx)
	if !strings.Contains(content, "- My favorite color is blue.") {
		t.Error("Fact not saved")
	}

	// 3. Forget memory
	forgetArgs := agent.Arguments{
		Raw:    []byte(`{"fact":"blue"}`),
		CallID: "test-call-2",
	}
	res, err = forgetTool.Execute(ctx, forgetArgs)
	if err != nil || res.Error != nil {
		t.Fatalf("Forget Execute failed: %v", err)
	}

	// 4. Verify forgotten
	content, _ = saveTool.mgr.load(ctx)
	if strings.Contains(content, "blue") {
		t.Error("Fact not forgotten")
	}
}
