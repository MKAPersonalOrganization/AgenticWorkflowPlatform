package engine

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/agentorchestrator/types"
	"school-with-ai/services/threadmgmt"
	"time"
)

// Use central types
type ChatClient = types.ChatClient
// Engine handles the core LLM iteration and streaming logic.
type Engine struct {
	log           *logger.Logger
	chatClient    ChatClient
	threadMgmt    *threadmgmt.Manager
	config        core.AgentConfig
	reqCtxCapture **core.RequestContext
	toolExec      *ToolExecutionService
}

// New creates a new execution engine.
func New(
	log *logger.Logger,
	chatClient ChatClient,
	threadMgmt *threadmgmt.Manager,
	config core.AgentConfig,
	reqCtxCapture **core.RequestContext,
	toolTracer core.ToolTracer,
) *Engine {
	return &Engine{
		log:           log,
		chatClient:    chatClient,
		threadMgmt:    threadMgmt,
		config:        config,
		reqCtxCapture: reqCtxCapture,
		toolExec:      NewToolExecutionService(toolTracer),
	}
}

// setAgentName automatically sets the agent name on events that have an AgentName field.
// This helper reduces boilerplate and prevents bugs from forgetting to set the agent name.
func (e *Engine) setAgentName(event domain.Event) {
	// Use type switch to set AgentName on events that support it
	switch evt := event.(type) {
	case *domain.AssistantMessageCompletedEvent:
		if evt.AgentName == "" {
			evt.AgentName = e.config.Name
		}
	case *domain.ToolCallStartedEvent:
		if evt.AgentName == "" {
			evt.AgentName = e.config.Name
		}
	case *domain.ToolCallExecutedEvent:
		if evt.AgentName == "" {
			evt.AgentName = e.config.Name
		}
	}
}

// yieldEvent is a helper that auto-assigns the agent name before yielding an event.
// This ensures all events have the correct agent name without manual assignment.
func (e *Engine) yieldEvent(yield func(core.Event, error) bool, event core.Event, err error) bool {
	if event != nil {
		e.setAgentName(event)
	}
	return yield(event, err)
}

// Run processes the LLM response using an AgentThread and yields events.
func (e *Engine) Run(
	ctx context.Context,
	runCtx core.RunContext,
) iter.Seq2[core.Event, error] {
	return func(yield func(core.Event, error) bool) {
		startEvent := &domain.AssistantMessageStartedEvent{
			ThreadID:  runCtx.ThreadID,
			MessageID: runCtx.MessageID,
			Timestamp: time.Now(),
		}
		if !yield(startEvent, nil) {
			return
		}

		session, err := e.newSession(ctx, runCtx)
		if err != nil {
			yield(nil, err)
			return
		}
		if session == nil {
			// Session terminated by middleware
			return
		}

		executor := e.toolExec.CreateExecutor(ctx, session.ctx.GetEffectiveTools(), runCtx.ThreadID, runCtx.MessageID, session.ctx)

		streamIter := e.chatClient.GetStreamingResponse(ctx, session.messages, session.options)

		var completeContent string
		toolCalls := core.NewToolCallAccumulator()

		for update, err := range streamIter {
			// Check if stream finished early due to cancellation
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if completeContent != "" {
					evt := &domain.AssistantMessageCompletedEvent{
						ThreadID:  runCtx.ThreadID,
						MessageID: runCtx.MessageID,
						Content:   completeContent,
						Timestamp: time.Now(),
					}
					if !e.yieldEvent(yield, evt, nil) {
						return
					}
				}
				yield(nil, fmt.Errorf("client disconnected: %w", err))
				return
			}

			if err != nil {
				yield(nil, fmt.Errorf("stream error: %w", err))
				return
			}

			textDelta := update.Text()
			if textDelta != "" {
				completeContent += textDelta

				contentEvent := &domain.AssistantContentStreamedEvent{
					ThreadID:  runCtx.ThreadID,
					MessageID: runCtx.MessageID,
					Delta:     textDelta,
					Timestamp: time.Now(),
				}
				if !yield(contentEvent, nil) {
					return
				}
			}

			tcUpdate, hasTool := toolCalls.ProcessUpdate(update)
			if hasTool {
				if tcUpdate.IsNew {
					toolStartEvent := &domain.ToolCallStartedEvent{
						ThreadID:        runCtx.ThreadID,
						ToolCallID:      tcUpdate.ID,
						ToolCallName:    tcUpdate.Name,
						ParentMessageID: runCtx.MessageID,
						Timestamp:       time.Now(),
					}
					if !e.yieldEvent(yield, toolStartEvent, nil) {
						return
					}
				}

				if tcUpdate.ArgsDelta != "" {
					argsEvent := &domain.ToolCallArgumentsStreamedEvent{
						ThreadID:   runCtx.ThreadID,
						ToolCallID: tcUpdate.ID,
						Delta:      tcUpdate.ArgsDelta,
						Timestamp:  time.Now(),
					}
					if !yield(argsEvent, nil) {
						return
					}
				}
			}
		}

		toolCallsList := toolCalls.GetAll()

		for _, tc := range toolCallsList {
			completedEvent := &domain.ToolCallCompletedEvent{
				ThreadID:   runCtx.ThreadID,
				ToolCallID: tc.ID,
				Timestamp:  time.Now(),
			}
			if !yield(completedEvent, nil) {
				return
			}
		}

		if toolCalls.HasToolCalls() {
			domainToolCalls := toDomainToolCalls(toolCallsList)
			agg, err := e.threadMgmt.AddAssistantMessage(ctx, runCtx.ThreadID, runCtx.MessageID, completeContent, domainToolCalls)
			if err != nil {
				yield(nil, fmt.Errorf("failed to add assistant message: %w", err))
				return
			}

			// Update local thread state with consistent state from repository
			runCtx.Thread = e.threadMgmt.ToAgentThread(agg)

			assistantEvent := &domain.AssistantMessageCompletedEvent{
				ThreadID:  runCtx.ThreadID,
				MessageID: runCtx.MessageID,
				Content:   completeContent,
				ToolCalls: domainToolCalls,
				Timestamp: time.Now(),
			}
			if !e.yieldEvent(yield, assistantEvent, nil) {
				return
			}

			// Execute all tools
			execResults := e.toolExec.ExecuteAll(toolCallsList, executor)

			// Persist results and emit events
			for _, result := range execResults.Results {
				toolResultID := core.GenerateID()
				agg, err = e.threadMgmt.AddToolResultMessage(ctx, runCtx.ThreadID, toolResultID, result.ToolCallID, result.Result)
				if err != nil {
					yield(nil, fmt.Errorf("failed to add tool result message: %w", err))
					return
				}

				// Update local thread state with consistent state from repository
				runCtx.Thread = e.threadMgmt.ToAgentThread(agg)

				toolEvent := &domain.ToolCallExecutedEvent{
					ThreadID:        runCtx.ThreadID,
					MessageID:       toolResultID,
					ToolCallID:      result.ToolCallID,
					ToolName:        result.ToolName,
					ToolArguments:   result.Arguments,
					ToolResult:      result.Result,
					ParentMessageID: runCtx.MessageID,
					Timestamp:       time.Now(),
				}
				if !e.yieldEvent(yield, toolEvent, nil) {
					return
				}
			}

			if execResults.ShouldStop {
				return
			}

			// Continue with the already updated in-memory thread
			runCtx.MessageID = core.GenerateID()
			for event, err := range e.Run(ctx, runCtx) {
				if !yield(event, err) {
					return
				}
			}
			return
		}

		if _, err := e.threadMgmt.AddAssistantMessage(ctx, runCtx.ThreadID, runCtx.MessageID, completeContent, nil); err != nil {
			yield(nil, fmt.Errorf("failed to add final assistant message: %w", err))
			return
		}

		assistantEvent := &domain.AssistantMessageCompletedEvent{
			ThreadID:  runCtx.ThreadID,
			MessageID: runCtx.MessageID,
			Content:   completeContent,
			Timestamp: time.Now(),
		}
		e.yieldEvent(yield, assistantEvent, nil)
	}
}

func toDomainToolCalls(ctcs []core.ToolCall) []domain.ToolCall {
	if ctcs == nil {
		return nil
	}
	result := make([]domain.ToolCall, len(ctcs))
	for i, ctc := range ctcs {
		result[i] = domain.ToolCall{
			ID:   ctc.ID,
			Type: ctc.Type,
			Function: domain.ToolFunction{
				Name:      ctc.Function.Name,
				Arguments: ctc.Function.Arguments,
			},
		}
	}
	return result
}

// runSession encapsulates the prepared state for an LLM run.
type runSession struct {
	options  *llm.ChatOptions
	ctx      *core.RequestContext
	messages []llm.ChatMessage
}

// newSession initializes the conversation history, applies middleware, and returns the execution session.
// Returns (nil, nil) if the session was terminated by middleware.
func (e *Engine) newSession(ctx context.Context, runCtx core.RunContext) (*runSession, error) {
	messages, err := runCtx.Thread.RetrieveFullConversationHistory(ctx)
	if err != nil {
		return nil, err
	}

	chatMessages := mapFromCoreToLLMMessages(messages)

	chatOptions, reqCtx, err := core.ToChatOptionsWithMiddleware(ctx, e.config, chatMessages, runCtx.ThreadID, runCtx.Options)
	if err != nil {
		return nil, fmt.Errorf("failed to apply middleware: %w", err)
	}

	if e.reqCtxCapture != nil {
		*e.reqCtxCapture = reqCtx
	}

	if reqCtx.Terminate {
		return nil, nil
	}

	return &runSession{
		options:  chatOptions,
		ctx:      reqCtx,
		messages: chatMessages,
	}, nil
}

