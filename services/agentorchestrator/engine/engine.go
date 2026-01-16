package engine

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/agentorchestrator/types"
	"school-with-ai/services/threadmgmt"
	"time"
)

// Use central types
type ChatClient = types.ChatClient
// Engine handles the core LLM iteration and streaming logic.
type Engine struct {
	log        *logger.Logger
	chatClient ChatClient
	config     core.AgentConfig
	toolExec   *ToolExecutionService
	llmCtx     *LLMContextManager
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
		log:        log,
		chatClient: chatClient,
		config:     config,
		toolExec:   NewToolExecutionService(toolTracer),
		llmCtx:     NewLLMContextManager(config, threadMgmt, reqCtxCapture),
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

		prepareResult, err := e.llmCtx.Prepare(ctx, runCtx)
		if err != nil {
			yield(nil, err)
			return
		}
		if prepareResult.Terminated {
			// Middleware terminated early
			return
		}

		llmContext := prepareResult.Context
		executor := e.toolExec.CreateExecutor(ctx, llmContext.GetEffectiveTools(), runCtx.ThreadID, runCtx.MessageID, llmContext.RequestCtx)

		streamIter := e.chatClient.GetStreamingResponse(ctx, llmContext.Messages, llmContext.Options)

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
			thread, err := e.llmCtx.AddAssistantMessage(ctx, runCtx.ThreadID, runCtx.MessageID, completeContent, domainToolCalls)
			if err != nil {
				yield(nil, err)
				return
			}
			runCtx.Thread = thread

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
				thread, err = e.llmCtx.AddToolResultMessage(ctx, runCtx.ThreadID, toolResultID, result.ToolCallID, result.Result)
				if err != nil {
					yield(nil, err)
					return
				}
				runCtx.Thread = thread

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

		if _, err := e.llmCtx.AddAssistantMessage(ctx, runCtx.ThreadID, runCtx.MessageID, completeContent, nil); err != nil {
			yield(nil, err)
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


