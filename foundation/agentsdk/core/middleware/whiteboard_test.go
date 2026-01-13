package middleware

import (
	"context"
	"encoding/json"
	"school-with-ai/foundation/agentsdk/core"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhiteboardMiddleware(t *testing.T) {
	t.Parallel()
	t.Run("adds write whiteboard tool and injects content in system prompt", func(t *testing.T) {
		t.Parallel()

		middleware := NewWhiteboardMiddleware()
		reqCtx := &core.RequestContext{
			ThreadID: "thread-123",
			Tools:    []core.Tool{},
			State:    make(map[string]any),
		}

		called := false
		next := func(ctx context.Context, rc *core.RequestContext) error {
			called = true
			// Check that write tool was added
			tools := rc.GetEffectiveTools()
			assert.Len(t, tools, 1, "should have 1 tool")

			var hasWrite bool
			for _, tool := range tools {
				if tool.Name() == "write_whiteboard" {
					hasWrite = true
				}
			}
			assert.True(t, hasWrite, "should have write_whiteboard tool")

			// Check that whiteboard content is in system prompt
			prompt := rc.GetEffectiveSystemPrompt()
			assert.Contains(t, prompt, "Current Whiteboard Content")
			assert.Contains(t, prompt, "The whiteboard is currently empty")

			return nil
		}

		err := middleware.Process(context.Background(), reqCtx, next)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("adds system prompt with whiteboard info", func(t *testing.T) {
		t.Parallel()

		middleware := NewWhiteboardMiddleware()
		reqCtx := &core.RequestContext{
			SystemPrompt: "",
			State:        make(map[string]any),
		}

		next := func(ctx context.Context, rc *core.RequestContext) error {
			prompt := rc.GetEffectiveSystemPrompt()
			assert.Contains(t, prompt, "Shared Whiteboard")
			assert.Contains(t, prompt, "write_whiteboard")
			assert.NotContains(t, prompt, "read_whiteboard")

			return nil
		}

		err := middleware.Process(context.Background(), reqCtx, next)
		require.NoError(t, err)
	})

	t.Run("initializes whiteboard state", func(t *testing.T) {
		t.Parallel()

		middleware := NewWhiteboardMiddleware()
		reqCtx := &core.RequestContext{
			ThreadID: "thread-123",
			State:    make(map[string]any),
		}

		next := func(ctx context.Context, rc *core.RequestContext) error {
			return nil
		}

		err := middleware.Process(context.Background(), reqCtx, next)
		require.NoError(t, err)
	})

	t.Run("injects existing whiteboard content into system prompt", func(t *testing.T) {
		t.Parallel()

		middleware := NewWhiteboardMiddleware()
		reqCtx := &core.RequestContext{
			ThreadID: "thread-123",
			State:    make(map[string]any),
		}

		// Pre-populate whiteboard content in request context
		SetWhiteboardContent(reqCtx, "# Existing Content\n- Item 1\n- Item 2")

		next := func(ctx context.Context, rc *core.RequestContext) error {
			prompt := rc.GetEffectiveSystemPrompt()
			assert.Contains(t, prompt, "Current Whiteboard Content")
			assert.Contains(t, prompt, "# Existing Content")
			assert.Contains(t, prompt, "- Item 1")
			assert.NotContains(t, prompt, "currently empty")

			return nil
		}

		err := middleware.Process(context.Background(), reqCtx, next)
		require.NoError(t, err)
	})

	t.Run("custom system prompt", func(t *testing.T) {
		t.Parallel()
		customPrompt := "Custom whiteboard instructions"

		middleware := NewWhiteboardMiddlewareWithPrompt(customPrompt)
		reqCtx := &core.RequestContext{
			ThreadID:     "thread-123",
			SystemPrompt: "",
			State:        make(map[string]any),
		}

		next := func(ctx context.Context, rc *core.RequestContext) error {
			prompt := rc.GetEffectiveSystemPrompt()
			assert.Contains(t, prompt, customPrompt)

			return nil
		}

		err := middleware.Process(context.Background(), reqCtx, next)
		require.NoError(t, err)
	})
}

func TestWriteWhiteboardTool(t *testing.T) {
	t.Parallel()
	t.Run("tool metadata", func(t *testing.T) {
		t.Parallel()
		reqCtx := &core.RequestContext{State: make(map[string]any)}
		tool := NewWriteWhiteboardTool(reqCtx)

		assert.Equal(t, "write_whiteboard", tool.Name())
		assert.Contains(t, tool.Description(), "Write information to the shared whiteboard")
		params := tool.Parameters()
		assert.Equal(t, "object", params["type"])

		props := params["properties"].(map[string]any)
		assert.Contains(t, props, "content")
		assert.Contains(t, props, "action")
	})

	t.Run("write action replaces content", func(t *testing.T) {
		t.Parallel()
		reqCtx := &core.RequestContext{State: make(map[string]any)}
		SetWhiteboardContent(reqCtx, "Old content")

		tool := NewWriteWhiteboardTool(reqCtx)
		args := map[string]string{
			"content": "New content",
			"action":  "write",
		}
		argsJSON, _ := json.Marshal(args)

		result, err := tool.Invoke(context.Background(), string(argsJSON))

		require.NoError(t, err)
		assert.Contains(t, result, "Whiteboard updated")
		assert.Equal(t, "New content", GetWhiteboardContent(reqCtx))
	})

	t.Run("append action adds to existing content", func(t *testing.T) {
		t.Parallel()
		reqCtx := &core.RequestContext{State: make(map[string]any)}
		SetWhiteboardContent(reqCtx, "First line")

		tool := NewWriteWhiteboardTool(reqCtx)
		args := map[string]string{
			"content": "Second line",
			"action":  "append",
		}
		argsJSON, _ := json.Marshal(args)

		result, err := tool.Invoke(context.Background(), string(argsJSON))

		require.NoError(t, err)
		assert.Contains(t, result, "Whiteboard updated")
		assert.Equal(t, "First line\nSecond line", GetWhiteboardContent(reqCtx))
	})

	t.Run("append to empty whiteboard", func(t *testing.T) {
		t.Parallel()
		reqCtx := &core.RequestContext{State: make(map[string]any)}
		SetWhiteboardContent(reqCtx, "")

		tool := NewWriteWhiteboardTool(reqCtx)
		args := map[string]string{
			"content": "First content",
			"action":  "append",
		}
		argsJSON, _ := json.Marshal(args)

		_, err := tool.Invoke(context.Background(), string(argsJSON))

		require.NoError(t, err)
		assert.Equal(t, "First content", GetWhiteboardContent(reqCtx))
	})

	t.Run("clear action removes all content", func(t *testing.T) {
		t.Parallel()
		reqCtx := &core.RequestContext{State: make(map[string]any)}
		SetWhiteboardContent(reqCtx, "Some content to clear")

		tool := NewWriteWhiteboardTool(reqCtx)
		args := map[string]string{
			"content": "",
			"action":  "clear",
		}
		argsJSON, _ := json.Marshal(args)

		result, err := tool.Invoke(context.Background(), string(argsJSON))

		require.NoError(t, err)
		assert.Contains(t, result, "Whiteboard updated")
		assert.Empty(t, GetWhiteboardContent(reqCtx))
	})

	t.Run("invalid action returns error", func(t *testing.T) {
		t.Parallel()
		reqCtx := &core.RequestContext{State: make(map[string]any)}
		tool := NewWriteWhiteboardTool(reqCtx)

		args := map[string]string{
			"content": "content",
			"action":  "invalid",
		}
		argsJSON, _ := json.Marshal(args)

		_, err := tool.Invoke(context.Background(), string(argsJSON))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid action")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()
		reqCtx := &core.RequestContext{State: make(map[string]any)}
		tool := NewWriteWhiteboardTool(reqCtx)

		_, err := tool.Invoke(context.Background(), "invalid json")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
	})
}

func TestGetSetWhiteboardContent(t *testing.T) {
	t.Parallel()
	t.Run("get and set whiteboard content", func(t *testing.T) {
		t.Parallel()
		reqCtx := &core.RequestContext{State: make(map[string]any)}

		// Initially empty
		content := GetWhiteboardContent(reqCtx)
		assert.Empty(t, content)

		// Set content
		SetWhiteboardContent(reqCtx, "Test content")
		content = GetWhiteboardContent(reqCtx)
		assert.Equal(t, "Test content", content)

		// Update content
		SetWhiteboardContent(reqCtx, "Updated content")
		content = GetWhiteboardContent(reqCtx)
		assert.Equal(t, "Updated content", content)
	})

	t.Run("get from context without whiteboard_content key", func(t *testing.T) {
		t.Parallel()
		reqCtx := &core.RequestContext{State: make(map[string]any)}
		// Don't set anything
		content := GetWhiteboardContent(reqCtx)
		assert.Empty(t, content)
	})
}

func TestWhiteboardMiddlewareIntegration(t *testing.T) {
	t.Parallel()
	t.Run("write and verify content in system prompt", func(t *testing.T) {
		t.Parallel()

		middleware := NewWhiteboardMiddleware()
		reqCtx := &core.RequestContext{
			Tools: []core.Tool{},
			State: make(map[string]any),
		}

		next := func(ctx context.Context, rc *core.RequestContext) error {
			tools := rc.GetEffectiveTools()

			// Find write tool
			var writeTool core.Tool
			for _, tool := range tools {
				if tool.Name() == "write_whiteboard" {
					writeTool = tool
				}
			}

			require.NotNil(t, writeTool)

			// Initial write
			writeArgs, _ := json.Marshal(map[string]string{
				"content": "# Initial Content\nFirst section",
				"action":  "write",
			})
			_, err := writeTool.Invoke(ctx, string(writeArgs))
			require.NoError(t, err)

			// Verify content is accessible via GetWhiteboardContent
			content := GetWhiteboardContent(rc)
			assert.Contains(t, content, "Initial Content")
			assert.Contains(t, content, "First section")

			// Modify and write (simulating read-modify-write using GetWhiteboardContent)
			modified := content + "\n\n# New Section\nAdded content"
			writeArgs2, _ := json.Marshal(map[string]string{
				"content": modified,
				"action":  "write",
			})
			_, err = writeTool.Invoke(ctx, string(writeArgs2))
			require.NoError(t, err)

			// Verify final content
			finalContent := GetWhiteboardContent(rc)
			assert.Contains(t, finalContent, "Initial Content")
			assert.Contains(t, finalContent, "New Section")

			return nil
		}

		err := middleware.Process(context.Background(), reqCtx, next)
		require.NoError(t, err)
	})
}
