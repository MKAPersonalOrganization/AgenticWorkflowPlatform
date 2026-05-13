package adkgo

import (
	"aicompass/modules/chat/thread"
	"encoding/json"
	"strings"

	"google.golang.org/adk/session"
)

// ──────────────────────────────────────────────────────────────────────────────
// State Delta Translation
//
// ADK-Go accumulates state changes in EventActions.StateDelta. These are
// transactional — applied when the event is committed. We translate non-internal
// state changes to thread.StateChangeReported events.
// ──────────────────────────────────────────────────────────────────────────────

func translateStateDelta(evt *session.Event, ctx EventContext) []thread.Event {
	if evt == nil || len(evt.Actions.StateDelta) == 0 {
		return nil
	}
	// A2A sub-agents encode file / artifact metadata as "__a2a_*" markers
	// in the state delta (see buildRemoteAgentNode in workflow.go). These
	// short-circuit the normal state-change emission path so the frontend
	// sees them as dedicated events, not as opaque state writes.
	if fileRaw, ok := evt.Actions.StateDelta["__a2a_file__"]; ok {
		return a2aFileEvent(fileRaw, evt, ctx)
	}
	if artifactRaw, ok := evt.Actions.StateDelta["__a2a_artifact__"]; ok {
		return a2aArtifactEvent(artifactRaw, evt, ctx)
	}
	return stateChangeEvents(evt, ctx)
}

// a2aFileEvent decodes an "__a2a_file__" marker into a
// FileAttachmentReceived event, or returns nil when the marker is
// malformed or empty (never promoted into a regular state change).
func a2aFileEvent(raw any, evt *session.Event, ctx EventContext) []thread.Event {
	fileJSON, ok := raw.(string)
	if !ok || fileJSON == "" {
		return nil
	}
	var info struct {
		FileURL  string `json:"fileURL"`
		FileName string `json:"fileName"`
		MimeType string `json:"mimeType"`
	}
	if err := json.Unmarshal([]byte(fileJSON), &info); err != nil || info.FileURL == "" {
		return nil
	}
	return []thread.Event{&thread.FileAttachmentReceived{
		ThreadID:  ctx.ThreadID,
		MessageID: evt.ID,
		AgentName: ctx.AgentName,
		FileURL:   info.FileURL,
		FileName:  info.FileName,
		MimeType:  info.MimeType,
		Timestamp: coalesceTime(evt.Timestamp),
	}}
}

// a2aArtifactEvent decodes an "__a2a_artifact__" marker into an
// A2AArtifactReceived event, or returns nil when malformed.
func a2aArtifactEvent(raw any, evt *session.Event, ctx EventContext) []thread.Event {
	artifactJSON, ok := raw.(string)
	if !ok || artifactJSON == "" {
		return nil
	}
	var info struct {
		ArtifactID string `json:"artifactId"`
		Title      string `json:"title"`
		Content    string `json:"content"`
	}
	if err := json.Unmarshal([]byte(artifactJSON), &info); err != nil || info.Content == "" {
		return nil
	}
	return []thread.Event{&thread.A2AArtifactReceived{
		ThreadID:   ctx.ThreadID,
		MessageID:  evt.ID,
		AgentName:  ctx.AgentName,
		ArtifactID: info.ArtifactID,
		Title:      info.Title,
		Content:    info.Content,
		Timestamp:  coalesceTime(evt.Timestamp),
	}}
}

// stateChangeEvents emits one StateChangeReported per user-visible key.
// Internal markers ("__" prefix) and ephemeral ADK scope ("temp:" prefix)
// are suppressed; session- / app: / user:-scoped keys are reported.
func stateChangeEvents(evt *session.Event, ctx EventContext) []thread.Event {
	var events []thread.Event
	now := coalesceTime(evt.Timestamp)
	for k, v := range evt.Actions.StateDelta {
		if strings.HasPrefix(k, "__") || strings.HasPrefix(k, "temp:") {
			continue
		}
		events = append(events, &thread.StateChangeReported{
			ThreadID:  ctx.ThreadID,
			AgentName: ctx.AgentName,
			Key:       k,
			Value:     v,
			Timestamp: now,
		})
	}
	return events
}

// ──────────────────────────────────────────────────────────────────────────────
// Artifact Delta Translation
//
// ADK-Go tracks artifact saves in EventActions.ArtifactDelta (map[string]int64).
// Each entry maps an artifact filename to the version that was saved during this
// event. We translate these to ArtifactVersionSaved domain events so the
// frontend can display artifact cards with version info.
// ──────────────────────────────────────────────────────────────────────────────

func translateArtifactDelta(evt *session.Event, ctx EventContext) []thread.Event {
	if evt == nil || len(evt.Actions.ArtifactDelta) == 0 {
		return nil
	}
	var events []thread.Event
	now := coalesceTime(evt.Timestamp)
	for fileName, version := range evt.Actions.ArtifactDelta {
		events = append(events, &thread.ArtifactVersionSaved{
			ThreadID:  ctx.ThreadID,
			AgentName: ctx.AgentName,
			FileName:  fileName,
			Version:   version,
			Timestamp: now,
		})
	}
	return events
}
