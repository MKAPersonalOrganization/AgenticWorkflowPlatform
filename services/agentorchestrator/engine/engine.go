package engine

import (
	"context"
	"fmt"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/agentorchestrator/threadmgmt"
	"school-with-ai/services/agentorchestrator/types"
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
	toolTracer    core.ToolTracer
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
		toolTracer:    toolTracer,
	}
}

// Run processes the LLM response using an AgentThread and yields events.
func (e *Engine) Run(
	ctx context.Context,
	runCtx core.RunContext,
) iter.Seq2[core.Event, error] {
	return func(yield func(core.Event, error) bool) {
		messages := runCtx.Thread.GetMessages()
		select {
		case <-ctx.Done():
			yield(nil, fmt.Errorf("client disconnected: %w", ctx.Err()))
			return
		default:
		}
		startEvent := &domain.AssistantMessageStartedEvent{
			ThreadID:  runCtx.ThreadID,
			MessageID: runCtx.MessageID,
			Timestamp: time.Now(),
		}
		if !yield(startEvent, nil) {
			return
		}

		llmMsgs := convertCoreMsgsToLLMMsgs(messages)
		chatMessages := llm.FromDomainMessages(llmMsgs)

		chatOptions, reqCtx, err := core.ToChatOptionsWithMiddleware(ctx, e.config, chatMessages, runCtx.ThreadID, runCtx.Options)
		if err != nil {
			yield(nil, fmt.Errorf("failed to apply middleware: %w", err))
			return
		}

		if e.reqCtxCapture != nil {
			*e.reqCtxCapture = reqCtx
		}

		if reqCtx.Terminate {
			return
		}

		dynamicRegistry := core.NewToolRegistry()
		for _, tool := range reqCtx.GetEffectiveTools() {
			dynamicRegistry.Register(tool)
		}

		executorOpts := []core.ToolExecutorOption{}
		if e.toolTracer != nil {
			executorOpts = append(executorOpts, core.WithToolTracer(e.toolTracer))
		}
		dynamicExecutor := core.NewRegistryToolExecutor(ctx, dynamicRegistry, executorOpts...)

		streamIter := e.chatClient.GetStreamingResponse(ctx, chatMessages, chatOptions)

		var completeContent string
		toolCalls := core.NewToolCallAccumulator()

		for update, err := range streamIter {
			select {
			case <-ctx.Done():
				if completeContent != "" {
					evt := &domain.AssistantMessageCompletedEvent{
						ThreadID:  runCtx.ThreadID,
						MessageID: runCtx.MessageID,
						Content:   completeContent,
						AgentName: e.config.Name,
						Timestamp: time.Now(),
					}
					if !yield(evt, nil) {
						return
					}
				}
				yield(nil, fmt.Errorf("client disconnected during streaming: %w", ctx.Err()))
				return
			default:
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
						AgentName:       e.config.Name,
						Timestamp:       time.Now(),
					}
					if !yield(toolStartEvent, nil) {
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
				AgentName: e.config.Name,
				Timestamp: time.Now(),
			}
			if !yield(assistantEvent, nil) {
				return
			}

			// Execute tools and check for Commands
			shouldContinue := true
			for _, toolCall := range toolCallsList {
				execResult, err := e.executeToolCallWithExecutor(runCtx.ThreadID, toolCall, dynamicExecutor)
				var result string
				if err != nil {
					result = fmt.Sprintf(`{"error": "%s"}`, err)
				} else {
					result = execResult.Result
				}

				// Check if this tool wants to stop iteration
				if execResult.StopIteration {
					shouldContinue = false
				}

				toolResultID := core.GenerateID()
				agg, err = e.threadMgmt.AddToolResultMessage(ctx, runCtx.ThreadID, toolResultID, toolCall.ID, result)
				if err != nil {
					yield(nil, fmt.Errorf("failed to add tool result message: %w", err))
					return
				}

				// Update local thread state with consistent state from repository
				runCtx.Thread = e.threadMgmt.ToAgentThread(agg)

				toolEvent := &domain.ToolCallExecutedEvent{
					ThreadID:        runCtx.ThreadID,
					MessageID:       toolResultID,
					ToolCallID:      toolCall.ID,
					ToolName:        toolCall.Function.Name,
					ToolArguments:   toolCall.Function.Arguments,
					ToolResult:      result,
					ParentMessageID: runCtx.MessageID,
					AgentName:       e.config.Name,
					Timestamp:       time.Now(),
				}
				if !yield(toolEvent, nil) {
					return
				}
			}

			if !shouldContinue {
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
			AgentName: e.config.Name,
			Timestamp: time.Now(),
		}
		yield(assistantEvent, nil)
	}
}

func (e *Engine) executeToolCallWithExecutor(threadID string, toolCall core.ToolCall, executor core.ToolExecutor) (core.ToolExecutionResult, error) {
	execResult, err := executor.Execute(toolCall)
	if err != nil {
		return execResult, fmt.Errorf("failed to execute tool call %s: %w", toolCall.Function.Name, err)
	}
	return execResult, nil
}

func convertCoreMsgToLLMMsg(cm core.Message) llm.Message {
	msg := llm.Message{
		ID:         cm.ID,
		Role:       cm.Role,
		Content:    cm.Content,
		Timestamp:  cm.Timestamp,
		ToolCallID: cm.ToolCallID,
		AgentName:  cm.AgentName,
		Metadata:   cm.Metadata,
	}

	for _, tc := range cm.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, llm.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: llm.ToolFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return msg
}

func convertCoreMsgsToLLMMsgs(msgs []core.Message) []llm.Message {
	result := make([]llm.Message, len(msgs))
	for i, cm := range msgs {
		result[i] = convertCoreMsgToLLMMsg(cm)
	}
	return result
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
