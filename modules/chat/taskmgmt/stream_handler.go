package taskmgmt

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"school-with-ai/modules/chat/taskmanagement"

	"github.com/hibiken/asynq"
)

// StreamJobEvents streams SSE events for a specific job.
func (h *TaskManagementHandler) StreamJobEvents(w http.ResponseWriter, r *http.Request) {
	// Set CORS origin header early — even error responses need CORS
	// to be readable by the browser from the allowed origin.
	origin := h.allowedOrigin
	if origin == "" {
		origin = "self"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)

	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)
		return
	}

	// Override the server's WriteTimeout for this long-lived SSE connection
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		h.log.Error(r.Context(), "[API] Failed to disable write deadline", "error", err)
		// Continue anyway - connection might still work
	}

	// Setup SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Get the flusher to flush data immediately
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// 1. Get Task Info to find Thread ID
	taskInfo, err := h.taskManager.GetTaskInfo("default", jobID)
	if err != nil {
		h.log.Warn(r.Context(), "[API] Failed to get task info", "jobID", jobID, "error", err)
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// 2. Extract ThreadID from payload
	var payload taskmanagement.ProcessChatPayload
	if err := json.Unmarshal(taskInfo.Payload, &payload); err != nil {
		h.log.Error(r.Context(), "[API] Failed to unmarshal task payload", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	threadID := payload.ThreadID

	// 3. Subscribe BEFORE replaying history.
	//    Subscribing first means any live events published while we are
	//    draining the replay batch land in the channel buffer and are not
	//    lost. We deduplicate by SeqID afterwards.
	eventCh, cancel := h.broadcaster.Subscribe(threadID)
	defer cancel()

	h.log.Info(r.Context(), "[API] Starting SSE stream", "jobID", jobID, "threadID", threadID)

	// 4. Replay missed events when the client reconnects with ?lastEventId=.
	//    lastEventId is a Redis Stream entry ID (e.g. "1717000000000-0").
	lastEventID := r.URL.Query().Get("lastEventId")
	maxReplayedSeqID := ""

	if lastEventID != "" {
		replayed := h.broadcaster.Replay(threadID, lastEventID)
		for _, env := range replayed {
			aguiEvents := taskmanagement.ToAguiEvents(env.Event)
			for _, ae := range aguiEvents {
				if err := WriteSSEEvent(w, ae, env.SeqID); err != nil {
					return
				}
			}
			if maxReplayedSeqID == "" || !streamIDLE(env.SeqID, maxReplayedSeqID) {
				maxReplayedSeqID = env.SeqID
			}
		}
		if len(replayed) > 0 {
			flusher.Flush()
		}
		h.log.Info(r.Context(), "[API] Replayed events",
			"jobID", jobID, "count", len(replayed), "upToSeqID", maxReplayedSeqID)
	}

	// 5. Stream live events with heartbeat to detect dead connections.
	ctx := r.Context()
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()
	// Maximum connection duration to prevent indefinite goroutine leaks
	maxTTL := time.NewTimer(30 * time.Minute)
	defer maxTTL.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-maxTTL.C:
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case env, ok := <-eventCh:
			if !ok {
				// Channel was closed by the broadcaster (slow client).
				// The client's onerror handler will reconnect with
				// ?lastEventId= and receive any missed events via Replay.
				return
			}

			// Skip events already sent during the replay phase to avoid
			// duplicating delta events (e.g. TEXT_MESSAGE_CONTENT).
			if maxReplayedSeqID != "" && streamIDLE(env.SeqID, maxReplayedSeqID) {
				continue
			}

			aguiEvents := taskmanagement.ToAguiEvents(env.Event)
			for _, ae := range aguiEvents {
				if err := WriteSSEEvent(w, ae, env.SeqID); err != nil {
					return
				}
			}
			flusher.Flush()
		}
	}
}

// GetJobStatus retrieves the status of a task.
func (h *TaskManagementHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		http.Error(w, "taskId is required", http.StatusBadRequest)

		return
	}

	// Try to get task info from Asynq
	taskInfo, err := h.taskManager.GetTaskInfo("default", taskID)

	// Create Task response
	task := Task{
		ID:        taskID,
		ContextID: "", // Would need to be stored/retrieved separately
		Metadata:  make(map[string]any),
	}

	// Add stream URL to metadata
	task.Metadata["streamUrl"] = fmt.Sprintf("/v1/tasks/%s:subscribe", taskID)

	if err != nil {
		task.Status = NewTaskStatus(TaskStateUnknown)
	} else {
		// Map Asynq state to TaskState
		var state TaskState
		switch taskInfo.State {
		case asynq.TaskStatePending:
			state = TaskStateSubmitted
		case asynq.TaskStateActive:
			state = TaskStateWorking
		case asynq.TaskStateScheduled:
			state = TaskStateSubmitted
		case asynq.TaskStateRetry:
			state = TaskStateWorking
		case asynq.TaskStateArchived:
			state = TaskStateFailed
		case asynq.TaskStateCompleted:
			state = TaskStateCompleted
		default:
			state = TaskStateUnknown
		}

		task.Status = NewTaskStatus(state)

		// Add retry information to metadata
		if taskInfo.Retried > 0 {
			task.Metadata["retried"] = taskInfo.Retried
			task.Metadata["maxRetry"] = taskInfo.MaxRetry
		}
		if taskInfo.LastErr != "" {
			task.Metadata["lastError"] = taskInfo.LastErr
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)

		return
	}
}

// CancelJob cancels a pending or active job.
func (h *TaskManagementHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)

		return
	}

	// Use Asynq Inspector to cancel task
	err := h.taskManager.CancelTask(jobID)
	if err != nil {
		http.Error(w, "Failed to cancel job", http.StatusBadRequest)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{
		"jobId":  jobID,
		"status": "cancelled",
	})
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)

		return
	}
}
