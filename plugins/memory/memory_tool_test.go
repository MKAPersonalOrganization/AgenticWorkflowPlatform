package memory

import (
	"context"
	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"strings"
	"testing"
)

// MockMemoryService for testing
type MockMemoryService struct {
	memories map[string]string
}

func NewMockMemoryService() *MockMemoryService {
	return &MockMemoryService{memories: make(map[string]string)}
}

func (m *MockMemoryService) Save(ctx context.Context, userID, content string) error {
	m.memories[userID] = content
	return nil
}

func (m *MockMemoryService) Load(ctx context.Context, userID string) (string, error) {
	return m.memories[userID], nil
}

func TestMemoryTool_Integration(t *testing.T) {
	t.Parallel()
	service := NewMockMemoryService()
	saveTool := NewMemoryTool(service)
	forgetTool := NewForgetMemoryTool(service)

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
	content, _ := saveTool.service.Load(ctx, user.ID)
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
	content, _ = saveTool.service.Load(ctx, user.ID)
	if strings.Contains(content, "blue") {
		t.Error("Fact not forgotten")
	}
}
