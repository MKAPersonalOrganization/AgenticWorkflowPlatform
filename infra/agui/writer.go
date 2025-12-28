package agui

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// WriteSSEEvent writes an event to the response writer in SSE format
func WriteSSEEvent(w http.ResponseWriter, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	if err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}
