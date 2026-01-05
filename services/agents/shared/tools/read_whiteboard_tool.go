package tools

import (
	"context"
	"encoding/json"
)

// WhiteboardReader is an interface for reading whiteboard content.
// This allows decoupling from the concrete WhiteboardStore implementation.
type WhiteboardReader interface {
	Get(threadID string) string
}

// ReadWhiteboardArgs represents the arguments for reading from the whiteboard
type ReadWhiteboardArgs struct {
	// No arguments needed - just read current content
}

// ReadWhiteboardTool implements the Tool interface for reading whiteboard content
type ReadWhiteboardTool struct {
	store    WhiteboardReader
	threadID string
}

// NewReadWhiteboardTool creates a new read whiteboard tool
func NewReadWhiteboardTool() *ReadWhiteboardTool {
	return &ReadWhiteboardTool{}
}

// SetWhiteboardStore sets the whiteboard store and thread ID for this tool instance.
// This should be called before the tool is used.
func (r *ReadWhiteboardTool) SetWhiteboardStore(store WhiteboardReader, threadID string) {
	r.store = store
	r.threadID = threadID
}

// SetContentGetter is deprecated but kept for backward compatibility.
// Use SetWhiteboardStore instead.
func (r *ReadWhiteboardTool) SetContentGetter(fn func() string) {
	// Wrap the function in an adapter
	r.store = &funcWhiteboardReader{fn: fn}
	r.threadID = "" // Not needed for function-based reader
}

// funcWhiteboardReader adapts a function to the WhiteboardReader interface
type funcWhiteboardReader struct {
	fn func() string
}

func (f *funcWhiteboardReader) Get(threadID string) string {
	return f.fn()
}

func (r *ReadWhiteboardTool) Name() string {
	return "read_whiteboard"
}

func (r *ReadWhiteboardTool) Description() string {
	return "Read the current content of the whiteboard. Returns the complete whiteboard markdown content. Use this to review what has been written, check previous answers, or understand the current state of the conversation."
}

func (r *ReadWhiteboardTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []string{},
	}
}

func (r *ReadWhiteboardTool) Invoke(ctx context.Context, arguments string) (string, error) {
	if r.store == nil {
		return "", nil // Return empty if no store is set
	}

	content := r.store.Get(r.threadID)

	result := map[string]any{
		"success": true,
		"content": content,
		"message": "Whiteboard content retrieved successfully",
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(resultJSON), nil
}
