package common

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// SSEConfig holds tunable parameters for Server-Sent Event connections.
// All values have safe production defaults via DefaultSSEConfig().
type SSEConfig struct {
	// WriteTimeout is the deadline applied to every individual SSE write.
	// If a write cannot complete within this window the connection is torn
	// down and the client can retry.
	WriteTimeout time.Duration
}

// DefaultSSEConfig returns production-safe defaults.
func DefaultSSEConfig() SSEConfig {
	return SSEConfig{
		WriteTimeout: 15 * time.Second,
	}
}

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

// ToolInSetResponse represents individual tools within a tool set.
type ToolInSetResponse struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	Category          string         `json:"category"`
	RequirementConfig map[string]any `json:"requirement_config,omitempty"` // Per-tool requirement config
}
