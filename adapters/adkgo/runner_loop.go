package adkgo

import (
	"aicompass/modules/chat/thread"
	"time"

	"google.golang.org/adk/session"
)

// ──────────────────────────────────────────────────────────────────────────────
// Run-loop translators
//
// The ADK event loop inside RunnerAdapter.Run used to be a single ~280-line
// for-range with nested switches and up-to-seven-deep branches. Every new
// event type piled another `case` + yield-and-check into the same function.
//
// This file pulls the per-event logic out into small pointer-receiver methods
// on runLoopState. Each method advances state and returns the thread.Events
// the outer iterator should yield. yields stay centralized in Run() (via
// yieldAll), so cancellation plumbing lives in one place instead of scattered
// across a dozen `if !yield(...) { return }` branches.
// ──────────────────────────────────────────────────────────────────────────────

// runLoopState is the mutable state that accumulates across iterations of the
// ADK event loop. Extracted so translator methods can read + mutate via one
// well-defined receiver rather than closing over a handful of locals.
type runLoopState struct {
	evtCtx         EventContext
	agentPositions map[string]AgentPosition // read-only; built at agent construction
	currentAuthor  string
	assistantMsgID string
	streamingMsgID string
	nodeStartedAt  time.Time
	loopIterations map[string]int
}

// isAgentTransition reports whether an ADK event's Author implies control has
// moved to a different sub-agent than the one we're currently tracking.
// User-authored events (function responses) are not transitions.
func isAgentTransition(evt *session.Event, currentAuthor string) bool {
	return evt.Author != "" && evt.Author != "user" && evt.Author != currentAuthor
}

// yieldAll yields events in order, returning false as soon as the receiver
// signals cancellation. Centralizes the `if !yield(...) { return }` pattern.
func yieldAll(yield func(thread.Event, error) bool, events []thread.Event) bool {
	for _, ev := range events {
		if !yield(ev, nil) {
			return false
		}
	}
	return true
}

// onAdkError emits cleanup events for when ADK reports an error mid-run: a
// failure completion for any tracked workflow node, and a message close if a
// stream was open. The caller is responsible for yielding the error itself.
func (s *runLoopState) onAdkError() []thread.Event {
	var out []thread.Event
	if _, tracked := s.agentPositions[s.currentAuthor]; tracked {
		out = append(out, &thread.WorkflowNodeCompleted{
			ThreadID:   s.evtCtx.ThreadID,
			AgentName:  s.currentAuthor,
			DurationMs: time.Since(s.nodeStartedAt).Milliseconds(),
			Outcome:    "error",
			Timestamp:  time.Now(),
		})
		s.nodeStartedAt = time.Now()
	}
	if s.streamingMsgID != "" {
		out = append(out, &thread.AssistantMessageCompleted{
			ThreadID:  s.evtCtx.ThreadID,
			MessageID: s.streamingMsgID,
			Timestamp: time.Now(),
		})
		s.streamingMsgID = ""
	}
	return out
}

// onTransition handles the switch from s.currentAuthor → newAuthor: close any
// open streaming message, mint a fresh assistantMsgID so each agent gets its
// own message bubble, emit WorkflowNodeCompleted(success) for the departing
// tracked sub-agent, then emit AgentTransferStarted enriched with workflow
// position (step/iteration context).
func (s *runLoopState) onTransition(newAuthor string) []thread.Event {
	var out []thread.Event

	if s.streamingMsgID != "" {
		out = append(out, &thread.AssistantMessageCompleted{
			ThreadID:  s.evtCtx.ThreadID,
			MessageID: s.streamingMsgID,
			Timestamp: time.Now(),
		})
		s.streamingMsgID = ""
	}

	s.assistantMsgID = generateID()

	fromAgent := s.currentAuthor
	s.currentAuthor = newAuthor
	s.evtCtx.AgentName = newAuthor

	transferEvt := &thread.AgentTransferStarted{
		ThreadID:  s.evtCtx.ThreadID,
		FromAgent: fromAgent,
		ToAgent:   newAuthor,
		Timestamp: time.Now(),
	}
	if pos, ok := s.agentPositions[newAuthor]; ok {
		transferEvt.WorkflowType = pos.WorkflowType
		transferEvt.ParentAgent = pos.ParentName
		transferEvt.StepIndex = pos.StepIndex
		transferEvt.StepCount = pos.StepCount
		transferEvt.MaxIterations = pos.MaxIterations
		if pos.WorkflowType == "loop" {
			// A return to stepIndex==0 within the same loop parent marks a
			// new iteration. First entry starts at 1.
			if _, seen := s.loopIterations[pos.ParentName]; !seen {
				s.loopIterations[pos.ParentName] = 1
			} else if pos.StepIndex == 0 {
				s.loopIterations[pos.ParentName]++
			}
			transferEvt.Iteration = s.loopIterations[pos.ParentName]
		}
	}

	if _, tracked := s.agentPositions[fromAgent]; tracked {
		out = append(out, &thread.WorkflowNodeCompleted{
			ThreadID:   s.evtCtx.ThreadID,
			AgentName:  fromAgent,
			DurationMs: time.Since(s.nodeStartedAt).Milliseconds(),
			Outcome:    "success",
			Timestamp:  time.Now(),
		})
	}
	s.nodeStartedAt = time.Now()

	out = append(out, transferEvt)
	return out
}

// onAssistantContentStreamed rewrites the per-delta UUID with the stable
// assistantMsgID and prefixes an AssistantMessageStarted on the first delta
// so the frontend can open its message bubble.
func (s *runLoopState) onAssistantContentStreamed(evt *thread.AssistantContentStreamed) []thread.Event {
	evt.MessageID = s.assistantMsgID
	var out []thread.Event
	if s.streamingMsgID == "" {
		s.streamingMsgID = s.assistantMsgID
		out = append(out, &thread.AssistantMessageStarted{
			ThreadID:  evt.ThreadID,
			MessageID: s.streamingMsgID,
			Timestamp: evt.Timestamp,
		})
	}
	out = append(out, evt)
	return out
}

// onAssistantMessageAdded ends the streaming lifecycle for a completed
// assistant message. If a stream was in progress, emit MessageCompleted to
// end the UI bubble. For non-streaming sources (A2A remote agents), synthesize
// the full start→delta→complete lifecycle so the frontend still renders the
// content. The durable MessageAdded event is always yielded last.
func (s *runLoopState) onAssistantMessageAdded(evt *thread.AssistantMessageAdded) []thread.Event {
	// A2A-sourced events may arrive with an empty session.Event.ID, which
	// would fail persistence. Back-fill with the stable assistantMsgID.
	if evt.MessageID == "" {
		evt.MessageID = s.assistantMsgID
	}

	var out []thread.Event
	switch {
	case s.streamingMsgID != "":
		out = append(out, &thread.AssistantMessageCompleted{
			ThreadID:  evt.ThreadID,
			MessageID: s.streamingMsgID,
			Content:   evt.Content,
			AgentName: evt.AgentName,
			ToolCalls: evt.ToolCalls,
			Timestamp: evt.Timestamp,
		})
		s.streamingMsgID = ""
	case evt.Content != "":
		// Non-streaming path: synthesize a full TEXT_MESSAGE lifecycle so the
		// frontend has something to render.
		out = append(out,
			&thread.AssistantMessageStarted{
				ThreadID:  evt.ThreadID,
				MessageID: evt.MessageID,
				Timestamp: evt.Timestamp,
			},
			&thread.AssistantContentStreamed{
				ThreadID:  evt.ThreadID,
				MessageID: evt.MessageID,
				Delta:     evt.Content,
				Timestamp: evt.Timestamp,
			},
			&thread.AssistantMessageCompleted{
				ThreadID:  evt.ThreadID,
				MessageID: evt.MessageID,
				Content:   evt.Content,
				AgentName: evt.AgentName,
				Timestamp: evt.Timestamp,
			},
		)
	}
	out = append(out, evt)
	return out
}

// onLoopEnd emits terminal cleanup after the ADK event stream is exhausted:
// completion for the last tracked sub-agent and closure of any dangling
// streaming message.
func (s *runLoopState) onLoopEnd() []thread.Event {
	var out []thread.Event
	if _, tracked := s.agentPositions[s.currentAuthor]; tracked {
		out = append(out, &thread.WorkflowNodeCompleted{
			ThreadID:   s.evtCtx.ThreadID,
			AgentName:  s.currentAuthor,
			DurationMs: time.Since(s.nodeStartedAt).Milliseconds(),
			Outcome:    "success",
			Timestamp:  time.Now(),
		})
	}
	if s.streamingMsgID != "" {
		out = append(out, &thread.AssistantMessageCompleted{
			ThreadID:  s.evtCtx.ThreadID,
			MessageID: s.streamingMsgID,
			Timestamp: time.Now(),
		})
	}
	return out
}
