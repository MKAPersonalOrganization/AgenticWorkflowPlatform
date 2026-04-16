package evaluation

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"aicompass/shared"

	"github.com/google/uuid"
)

// RunEvalInline handles POST /api/evaluation/runs/eval-stream
// Streams evaluation events inline using the same pattern as AGUI message streaming.
func RunEvalInline(svc *service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := shared.GetUserFromContext(r.Context())
		if user == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse request
		var req EvalRunCreate
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		req.UserID = user.ID

		// Set SSE headers (same as AGUI)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		// Enable full duplex
		rc := http.NewResponseController(w)
		if err := rc.EnableFullDuplex(); err != nil {
			slog.Info("EnableFullDuplex not supported", "error", err)
		}

		// Send SESSION_START (same as AGUI)
		_ = rc.SetWriteDeadline(time.Now().Add(15 * time.Second))
		runID := uuid.New()
		if err := writeSSEEvent(w, map[string]string{
			"type":      "SESSION_START",
			"contextId": runID.String(),
		}, ""); err != nil {
			return
		}
		flusher.Flush()

		// Execute evaluation and stream events inline
		for event, err := range svc.executeEvalStream(r.Context(), user.ID, req, runID) {
			writeErr := rc.SetWriteDeadline(time.Now().Add(15 * time.Second))
			if writeErr != nil || r.Context().Err() != nil {
				slog.Info("Client disconnected, aborting evaluation stream")
				return
			}

			if err != nil {
				slog.Error("Evaluation execution error", "error", err)
				_ = writeSSEEvent(w, map[string]any{
					"type":    "RUN_ERROR",
					"message": err.Error(),
				}, "")
				flusher.Flush()
				return
			}

			if err := writeSSEEvent(w, event, ""); err != nil {
				slog.Info("Write failed, client disconnected")
				return
			}
			flusher.Flush()
		}
	}
}
