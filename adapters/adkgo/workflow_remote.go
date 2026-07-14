package adkgo

import (
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	fagent "aicompass/foundation/agent"
)

// buildRemoteAgentNode creates a custom ADK-Go agent that delegates to an
// external A2A endpoint via the A2ASubAgentProvider port.
//
// Because ADK-Go's remoteagent package uses a different version of the a2a-go
// module (v0.3.3 vs our v2), we cannot use remoteagent.NewA2A() directly.
// Instead we build a plain agent.New() whose Run function calls our existing
// a2a.Service.SendMessage() infrastructure, keeping all auth and transport
// logic in one place.
func buildRemoteAgentNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo) (agent.Agent, error) {
	if node.A2AConnectorID == "" {
		return nil, fmt.Errorf("node %q: remote node requires a2a_connector_id", node.ID)
	}
	if wbc.adapter.a2aSubAgentProvider == nil {
		return nil, fmt.Errorf("node %q: A2ASubAgentProvider not configured", node.ID)
	}

	// Resolve connector metadata once at build time for name/description.
	info, err := wbc.adapter.a2aSubAgentProvider.GetConnectorInfo(wbc.ctx, node.A2AConnectorID, wbc.userID)
	if err != nil {
		return nil, fmt.Errorf("node %q: resolving A2A connector %q: %w", node.ID, node.A2AConnectorID, err)
	}

	name := node.Label
	if name == "" {
		name = info.Name
	}
	agentName := wbc.uniqueName(name)

	return agent.New(agent.Config{
		Name:        agentName,
		Description: info.Description,
		Run:         remoteRunFn(wbc.adapter.a2aSubAgentProvider, node.A2AConnectorID, wbc.userID, node.OutputKey, agentName),
	})
}

// remoteRunFn returns the Run closure that streams A2A chunks from a remote
// agent into ADK-Go session events.
//
// Text deltas are buffered one step behind: we emit the previous delta
// as a PARTIAL event (AssistantContentStreamed → live display) only once
// we know there is a following delta. The very last delta is emitted as
// the NON-PARTIAL event so ADK-Go persists the full accumulated text to
// the session — no separate final event needed, which avoids re-displaying
// the content a second time.
//
// Artifact chunks are emitted immediately as separate non-content events
// carrying the artifact data in StateDelta, which TranslateEvent converts
// to thread.A2AArtifactReceived for separate frontend display.
func remoteRunFn(provider A2ASubAgentProvider, connectorID, userID, outputKey, agentName string) func(agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(ictx agent.InvocationContext) iter.Seq2[*session.Event, error] {
		return func(yield func(*session.Event, error) bool) {
			message := extractUserMessage(ictx)

			var accumulated strings.Builder
			var pendingEvt *session.Event

			for chunk, streamErr := range provider.SendStreamingMessage(ictx, connectorID, userID, message) {
				if streamErr != nil {
					yield(nil, fmt.Errorf("remote agent %q: %w", agentName, streamErr))
					return
				}

				if chunk.IsFile {
					if !yield(newA2AFileEvent(ictx, agentName, chunk), nil) {
						return
					}
					continue
				}

				if chunk.IsArtifact {
					// Skip artifacts whose content duplicates the accumulated streaming
					// text. Python ADK always packages the complete response as a final
					// artifact even for plain conversational responses — showing it would
					// repeat the message the user already sees in the stream.
					// Only surface artifacts with genuinely distinct content (e.g. a
					// generated playlist document, file, or structured output).
					if chunk.Content == "" || chunk.Content == accumulated.String() {
						continue
					}
					// Note: pendingEvt is intentionally NOT flushed here — the normal
					// end-of-loop path emits the final NON-PARTIAL text event so the
					// message is persisted correctly regardless of artifact ordering.
					if !yield(newA2AArtifactEvent(ictx, agentName, chunk), nil) {
						return
					}
					continue
				}

				if chunk.Delta == "" {
					continue
				}

				// Flush the previous delta as PARTIAL now that we know it is
				// not the last chunk.
				if pendingEvt != nil {
					if !yield(pendingEvt, nil) {
						return
					}
				}

				accumulated.WriteString(chunk.Delta)
				pendingEvt = newA2APartialTextEvent(ictx, agentName, chunk.Delta)
			}

			// Emit the last buffered event as NON-PARTIAL with the full
			// accumulated text so the session service persists the message.
			if pendingEvt != nil {
				pendingEvt.Partial = false
				pendingEvt.Content = &genai.Content{
					Role:  "model",
					Parts: []*genai.Part{{Text: accumulated.String()}},
				}
				if outputKey != "" {
					pendingEvt.Actions.StateDelta = map[string]any{outputKey: accumulated.String()}
				}
				yield(pendingEvt, nil)
			}
		}
	}
}

// extractUserMessage pulls the first non-empty text Part from the invocation's
// UserContent — the message the remote agent should respond to.
func extractUserMessage(ictx agent.InvocationContext) string {
	uc := ictx.UserContent()
	if uc == nil {
		return ""
	}
	for _, part := range uc.Parts {
		if part != nil && part.Text != "" {
			return part.Text
		}
	}
	return ""
}

// newA2AFileEvent encodes A2A file metadata into StateDelta so TranslateEvent
// can emit a FileAttachmentReceived event for separate frontend display.
func newA2AFileEvent(ictx agent.InvocationContext, agentName string, chunk A2AStreamChunk) *session.Event {
	fileData, _ := json.Marshal(map[string]any{
		"fileURL":  chunk.FileURL,
		"fileName": chunk.FileName,
		"mimeType": chunk.MimeType,
	})
	evt := session.NewEvent(ictx, ictx.InvocationID())
	evt.Author = agentName
	evt.Actions.StateDelta = map[string]any{
		"__a2a_file__": string(fileData),
	}
	return evt
}

// newA2AArtifactEvent encodes artifact metadata into StateDelta so TranslateEvent
// can emit thread.A2AArtifactReceived without changing the session.Event structure.
func newA2AArtifactEvent(ictx agent.InvocationContext, agentName string, chunk A2AStreamChunk) *session.Event {
	artifactData, _ := json.Marshal(map[string]any{
		"artifactId": chunk.ArtifactID,
		"title":      chunk.Title,
		"content":    chunk.Content,
	})
	evt := session.NewEvent(ictx, ictx.InvocationID())
	evt.Author = agentName
	evt.Actions.StateDelta = map[string]any{
		"__a2a_artifact__": string(artifactData),
	}
	return evt
}

// newA2APartialTextEvent builds a PARTIAL text event carrying a single delta.
// The caller decides whether to keep it partial (flush mid-stream) or promote
// it to the final NON-PARTIAL event with accumulated text at stream end.
func newA2APartialTextEvent(ictx agent.InvocationContext, agentName, delta string) *session.Event {
	evt := session.NewEvent(ictx, ictx.InvocationID())
	evt.Author = agentName
	evt.Content = &genai.Content{
		Role:  "model",
		Parts: []*genai.Part{{Text: delta}},
	}
	evt.Partial = true
	return evt
}
