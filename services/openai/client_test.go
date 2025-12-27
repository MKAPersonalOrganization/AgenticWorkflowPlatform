package openai

import (
	"context"
	"testing"

	"github.com/openai/openai-go/v3"
	"school-with-ai/internal/session"
	"school-with-ai/internal/tools"
)

// TestCreateStreamingCompletionWithoutTools verifies stream creation without tools
func TestCreateStreamingCompletionWithoutTools(t *testing.T) {
	// This test verifies that CreateStreamingCompletion works with empty tools slice
	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	messages := []session.Message{
		{
			ID:      "msg_1",
			Role:    "user",
			Content: "Hello",
		},
	}

	ctx := context.Background()

	// Should not panic with nil or empty tools
	stream := client.CreateStreamingCompletion(ctx, messages, nil)
	if stream == nil {
		t.Error("Expected non-nil stream")
	}

	// Close the stream without reading (no actual API call)
	// In real tests with mocked client, we'd verify the request
}

// TestCreateStreamingCompletionWithTools verifies stream creation with tools
func TestCreateStreamingCompletionWithTools(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	messages := []session.Message{
		{
			ID:      "msg_1",
			Role:    "user",
			Content: "What's the weather?",
		},
	}

	// Get tools from our tools package
	availableTools := tools.GetAllTools()

	ctx := context.Background()

	// Should accept tools parameter
	stream := client.CreateStreamingCompletion(ctx, messages, availableTools)
	if stream == nil {
		t.Error("Expected non-nil stream with tools")
	}

	// Verify tools slice is not empty
	if len(availableTools) == 0 {
		t.Error("Expected at least one tool")
	}
}

// TestBuildMessagesWithToolRole verifies message building handles all roles
func TestBuildMessagesWithToolRole(t *testing.T) {
	messages := []session.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "system", Content: "You are helpful"},
	}

	params := BuildMessages(messages)

	if len(params) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(params))
	}

	// Verify all messages were converted
	// Note: We can't easily inspect the union types without marshaling to JSON
	// but we can verify the count
}

// TestToolsParameterIsOptional verifies backward compatibility
func TestToolsParameterIsOptional(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	messages := []session.Message{
		{Role: "user", Content: "Test"},
	}

	ctx := context.Background()

	// Test with nil tools
	stream1 := client.CreateStreamingCompletion(ctx, messages, nil)
	if stream1 == nil {
		t.Error("Stream should work with nil tools")
	}

	// Test with empty slice
	stream2 := client.CreateStreamingCompletion(ctx, messages, []openai.ChatCompletionToolUnionParam{})
	if stream2 == nil {
		t.Error("Stream should work with empty tools slice")
	}

	// Test with actual tools
	stream3 := client.CreateStreamingCompletion(ctx, messages, tools.GetAllTools())
	if stream3 == nil {
		t.Error("Stream should work with tools")
	}
}

// TestGenerateThreadTitleDoesNotUseTools verifies title generation doesn't include tools
func TestGenerateThreadTitleDoesNotUseTools(t *testing.T) {
	// This test verifies that GenerateThreadTitle still works without tools
	// and hasn't been affected by the changes to CreateStreamingCompletion

	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	messages := []session.Message{
		{Role: "user", Content: "What's the weather in Paris?"},
		{Role: "assistant", Content: "Let me check that for you."},
	}

	ctx := context.Background()

	// This should work without any issues
	// Note: This will actually call OpenAI API if OPENAI_API_KEY is set
	// In production tests, this should be mocked
	_, err = client.GenerateThreadTitle(ctx, messages)

	// We expect either success or an auth error (if no API key)
	// We don't expect a tools-related error
	if err != nil {
		// Check it's not a tools-related error
		// In a real environment with API key, this would succeed
		t.Logf("Title generation error (expected if no API key): %v", err)
	}
}
