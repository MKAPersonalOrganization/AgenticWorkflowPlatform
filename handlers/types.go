package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"school-with-ai/domain"
)

// writeSSEEvent writes an SSE event to the response writer.
func writeSSEEvent(w io.Writer, event any) error {
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", eventBytes)

	return err
}

// ThreadResponse represents a thread with its messages.
type ThreadResponse struct {
	*domain.Thread

	WhiteboardContent string `json:"whiteboardContent,omitempty"`
}
