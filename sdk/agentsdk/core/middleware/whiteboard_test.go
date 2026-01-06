package middleware

import (
	"context"
	"encoding/json"
	"testing"

	"school-with-ai/sdk/agentsdk/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhiteboardMiddleware(t *testing.T) {
	t.Run("adds read and write whiteboard tools", func(t *testing.T) {
		middleware := NewWhiteboardMiddleware()
		reqCtx := &core.RequestContext{
			Tools: []core.Tool{},
			State: make(map[string]interface{}),
		}

		called := false
		next := func(ctx context.Context, rc *core.RequestContext) error {
			called = true
			// Check that tools were added
			tools := rc.GetEffectiveTools()
			assert.Len(t, tools, 2, "should have 2 tools")

			var hasRead, hasWrite bool
			for _, tool := range tools {
				if tool.Name() == "read_whiteboard" {
					hasRead = true
				}
				if tool.Name() == "write_whiteboard" {
					hasWrite = true
				}
			}
			assert.True(t, hasRead, "should have read_whiteboard tool")
			assert.True(t, hasWrite, "should have write_whiteboard tool")
			return nil
		}

		err := middleware.Process(context.Background(), reqCtx, next)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("adds system prompt", func(t *testing.T) {
		middleware := NewWhiteboardMiddleware()
		reqCtx := &core.RequestContext{
			SystemPrompt: "",
			State:        make(map[string]interface{}),
		}

		next := func(ctx context.Context, rc *core.RequestContext) error {
			prompt := rc.GetEffectiveSystemPrompt()
			assert.Contains(t, prompt, "Whiteboard Tools")
			assert.Contains(t, prompt, "read_whiteboard")
			assert.Contains(t, prompt, "write_whiteboard")
			return nil
		}

		err := middleware.Process(context.Background(), reqCtx, next)
		require.NoError(t, err)
	})

	t.Run("initializes whiteboard state", func(t *testing.T) {
		middleware := NewWhiteboardMiddleware()
		reqCtx := &core.RequestContext{
			State: make(map[string]interface{}),
		}

		next := func(ctx context.Context, rc *core.RequestContext) error {
			content := GetWhiteboardContent(rc)
			assert.Equal(t, "", content, "should initialize to empty string")
			return nil
		}

		err := middleware.Process(context.Background(), reqCtx, next)
		require.NoError(t, err)
	})

	t.Run("custom system prompt", func(t *testing.T) {
		customPrompt := "Custom whiteboard instructions"
		middleware := NewWhiteboardMiddlewareWithPrompt(customPrompt)
		reqCtx := &core.RequestContext{
			SystemPrompt: "",
			State:        make(map[string]interface{}),
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

func TestReadWhiteboardTool(t *testing.T) {
	t.Run("tool metadata", func(t *testing.T) {
		reqCtx := &core.RequestContext{State: make(map[string]interface{})}
		tool := NewReadWhiteboardTool(reqCtx)

		assert.Equal(t, "read_whiteboard", tool.Name())
		assert.Contains(t, tool.Description(), "Read the current content")
		params := tool.Parameters()
		assert.Equal(t, "object", params["type"])
	})

	t.Run("returns empty message when whiteboard is empty", func(t *testing.T) {
		reqCtx := &core.RequestContext{State: make(map[string]interface{})}
		SetWhiteboardContent(reqCtx, "")

		tool := NewReadWhiteboardTool(reqCtx)
		result, err := tool.Invoke(context.Background(), "{}")

		require.NoError(t, err)
		assert.Equal(t, "The whiteboard is currently empty.", result)
	})

	t.Run("returns whiteboard content", func(t *testing.T) {
		reqCtx := &core.RequestContext{State: make(map[string]interface{})}
		content := "# Formula\nF = ma"
		SetWhiteboardContent(reqCtx, content)

		tool := NewReadWhiteboardTool(reqCtx)
		result, err := tool.Invoke(context.Background(), "{}")

		require.NoError(t, err)
		assert.Equal(t, content, result)
	})
}

func TestWriteWhiteboardTool(t *testing.T) {
	t.Run("tool metadata", func(t *testing.T) {
		reqCtx := &core.RequestContext{State: make(map[string]interface{})}
		tool := NewWriteWhiteboardTool(reqCtx)

		assert.Equal(t, "write_whiteboard", tool.Name())
		assert.Contains(t, tool.Description(), "Write information to the shared whiteboard")
		params := tool.Parameters()
		assert.Equal(t, "object", params["type"])

		props := params["properties"].(map[string]interface{})
		assert.Contains(t, props, "content")
		assert.Contains(t, props, "action")
	})

	t.Run("write action replaces content", func(t *testing.T) {
		reqCtx := &core.RequestContext{State: make(map[string]interface{})}
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
		reqCtx := &core.RequestContext{State: make(map[string]interface{})}
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
		reqCtx := &core.RequestContext{State: make(map[string]interface{})}
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
		reqCtx := &core.RequestContext{State: make(map[string]interface{})}
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
		assert.Equal(t, "", GetWhiteboardContent(reqCtx))
	})

	t.Run("invalid action returns error", func(t *testing.T) {
		reqCtx := &core.RequestContext{State: make(map[string]interface{})}
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
		reqCtx := &core.RequestContext{State: make(map[string]interface{})}
		tool := NewWriteWhiteboardTool(reqCtx)

		_, err := tool.Invoke(context.Background(), "invalid json")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
	})
}

func TestGetSetWhiteboardContent(t *testing.T) {
	t.Run("get and set whiteboard content", func(t *testing.T) {
		reqCtx := &core.RequestContext{State: make(map[string]interface{})}

		// Initially empty
		content := GetWhiteboardContent(reqCtx)
		assert.Equal(t, "", content)

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
		reqCtx := &core.RequestContext{State: make(map[string]interface{})}
		// Don't set anything
		content := GetWhiteboardContent(reqCtx)
		assert.Equal(t, "", content)
	})
}

func TestWhiteboardMiddlewareIntegration(t *testing.T) {
	t.Run("read-modify-write pattern", func(t *testing.T) {
		middleware := NewWhiteboardMiddleware()
		reqCtx := &core.RequestContext{
			Tools: []core.Tool{},
			State: make(map[string]interface{}),
		}

		next := func(ctx context.Context, rc *core.RequestContext) error {
			tools := rc.GetEffectiveTools()

			// Find tools
			var readTool, writeTool core.Tool
			for _, tool := range tools {
				if tool.Name() == "read_whiteboard" {
					readTool = tool
				}
				if tool.Name() == "write_whiteboard" {
					writeTool = tool
				}
			}

			require.NotNil(t, readTool)
			require.NotNil(t, writeTool)

			// Initial write
			writeArgs, _ := json.Marshal(map[string]string{
				"content": "# Initial Content\nFirst section",
				"action":  "write",
			})
			_, err := writeTool.Invoke(ctx, string(writeArgs))
			require.NoError(t, err)

			// Read
			readResult, err := readTool.Invoke(ctx, "{}")
			require.NoError(t, err)
			assert.Contains(t, readResult, "Initial Content")

			// Modify and write
			modified := readResult + "\n\n# New Section\nAdded content"
			writeArgs2, _ := json.Marshal(map[string]string{
				"content": modified,
				"action":  "write",
			})
			_, err = writeTool.Invoke(ctx, string(writeArgs2))
			require.NoError(t, err)

			// Verify
			finalResult, err := readTool.Invoke(ctx, "{}")
			require.NoError(t, err)
			assert.Contains(t, finalResult, "Initial Content")
			assert.Contains(t, finalResult, "New Section")

			return nil
		}

		err := middleware.Process(context.Background(), reqCtx, next)
		require.NoError(t, err)
	})
}
