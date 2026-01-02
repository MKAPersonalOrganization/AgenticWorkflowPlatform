package tools

import (
	"context"
	"encoding/json"
)

// ReadWhiteboardArgs represents the arguments for reading from the whiteboard
type ReadWhiteboardArgs struct {
	// No arguments needed - just read current content
}

// ReadWhiteboardTool implements the Tool interface for reading whiteboard content
type ReadWhiteboardTool struct {
	getContentFunc func() string
}

// NewReadWhiteboardTool creates a new read whiteboard tool
func NewReadWhiteboardTool() *ReadWhiteboardTool {
	return &ReadWhiteboardTool{}
}

// SetContentGetter sets the function to retrieve whiteboard content
func (r *ReadWhiteboardTool) SetContentGetter(fn func() string) {
	r.getContentFunc = fn
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
	if r.getContentFunc == nil {
		return "", nil // Return empty if no getter is set
	}

	content := r.getContentFunc()

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
