package taskmgmt

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteSSEEvent writes an SSE event to the response writer.
func WriteSSEEvent(w io.Writer, event any) error {
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", eventBytes)
	if err != nil {
		return fmt.Errorf("failed to write SSE event: %w", err)
	}

	return nil
}
