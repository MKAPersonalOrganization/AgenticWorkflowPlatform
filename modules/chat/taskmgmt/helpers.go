package taskmgmt

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteSSEEvent writes an SSE event to the response writer.
// If seqID is non-empty it is written as the SSE "id:" field, which the
// browser stores as MessageEvent.lastEventId and forwards on reconnect.
func WriteSSEEvent(w io.Writer, event any, seqID string) error {
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if seqID != "" {
		_, err = fmt.Fprintf(w, "id: %s\ndata: %s\n\n", seqID, eventBytes)
	} else {
		_, err = fmt.Fprintf(w, "data: %s\n\n", eventBytes)
	}
	if err != nil {
		return fmt.Errorf("failed to write SSE event: %w", err)
	}
	return nil
}
