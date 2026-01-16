package engine

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
	"time"
)

// StreamResult holds the accumulated output from processing an LLM stream.
type StreamResult struct {
	Content   string
	ToolCalls []core.ToolCall
}

// StreamProcessor handles LLM stream iteration and event emission.
// It accumulates content and tool calls while emitting streaming events.
type StreamProcessor struct {
	agentName string
}

// NewStreamProcessor creates a new StreamProcessor.
func NewStreamProcessor(agentName string) *StreamProcessor {
	return &StreamProcessor{
		agentName: agentName,
	}
}

// ProcessParams contains the parameters for processing a stream.
type ProcessParams struct {
	ThreadID  string
	MessageID string
}

// Process iterates over the LLM stream, accumulates content and tool calls,
// and emits streaming events via the yield function.
//
// Returns the accumulated result or an error. If the yield function returns false,
// processing stops and returns the partial result with no error.
func (p *StreamProcessor) Process(
	ctx context.Context,
	streamIter iter.Seq2[llm.ChatResponseUpdate, error],
	params ProcessParams,
	yield func(core.Event, error) bool,
) (*StreamResult, error) {
	var content string
	toolCalls := core.NewToolCallAccumulator()

	for update, err := range streamIter {
		// Handle cancellation
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// Emit partial completion if we have content
			if content != "" {
				evt := &domain.AssistantMessageCompletedEvent{
					ThreadID:  params.ThreadID,
					MessageID: params.MessageID,
					Content:   content,
					AgentName: p.agentName,
					Timestamp: time.Now(),
				}
				if !yield(evt, nil) {
					return &StreamResult{Content: content, ToolCalls: toolCalls.GetAll()}, nil
				}
			}
			return nil, fmt.Errorf("client disconnected: %w", err)
		}

		if err != nil {
			return nil, fmt.Errorf("stream error: %w", err)
		}

		// Process text content
		if textDelta := update.Text(); textDelta != "" {
			content += textDelta

			contentEvent := &domain.AssistantContentStreamedEvent{
				ThreadID:  params.ThreadID,
				MessageID: params.MessageID,
				Delta:     textDelta,
				Timestamp: time.Now(),
			}
			if !yield(contentEvent, nil) {
				return &StreamResult{Content: content, ToolCalls: toolCalls.GetAll()}, nil
			}
		}

		// Process tool calls
		tcUpdate, hasTool := toolCalls.ProcessUpdate(update)
		if hasTool {
			if tcUpdate.IsNew {
				toolStartEvent := &domain.ToolCallStartedEvent{
					ThreadID:        params.ThreadID,
					ToolCallID:      tcUpdate.ID,
					ToolCallName:    tcUpdate.Name,
					ParentMessageID: params.MessageID,
					AgentName:       p.agentName,
					Timestamp:       time.Now(),
				}
				if !yield(toolStartEvent, nil) {
					return &StreamResult{Content: content, ToolCalls: toolCalls.GetAll()}, nil
				}
			}

			if tcUpdate.ArgsDelta != "" {
				argsEvent := &domain.ToolCallArgumentsStreamedEvent{
					ThreadID:   params.ThreadID,
					ToolCallID: tcUpdate.ID,
					Delta:      tcUpdate.ArgsDelta,
					Timestamp:  time.Now(),
				}
				if !yield(argsEvent, nil) {
					return &StreamResult{Content: content, ToolCalls: toolCalls.GetAll()}, nil
				}
			}
		}
	}

	return &StreamResult{
		Content:   content,
		ToolCalls: toolCalls.GetAll(),
	}, nil
}

// HasToolCalls returns true if the result contains tool calls.
func (r *StreamResult) HasToolCalls() bool {
	return len(r.ToolCalls) > 0
}
