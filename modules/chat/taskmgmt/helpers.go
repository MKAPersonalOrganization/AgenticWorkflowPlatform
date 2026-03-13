package taskmgmt

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
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

// streamIDLE reports whether Redis Stream ID a is less than or equal to b.
// Stream IDs have the format "{milliseconds}-{sequence}".
func streamIDLE(a, b string) bool {
	if a == b {
		return true
	}
	parseID := func(s string) (int64, int64) {
		idx := strings.LastIndex(s, "-")
		if idx < 0 {
			return 0, 0
		}
		ms, _ := strconv.ParseInt(s[:idx], 10, 64)
		seq, _ := strconv.ParseInt(s[idx+1:], 10, 64)
		return ms, seq
	}
	ams, aseq := parseID(a)
	bms, bseq := parseID(b)
	if ams != bms {
		return ams < bms
	}
	return aseq <= bseq
}
