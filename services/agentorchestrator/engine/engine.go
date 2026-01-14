package engine

import (
	"context"
	"fmt"
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
	notifier      core.EventNotifier
	config        core.AgentConfig
	reqCtxCapture **core.RequestContext
	toolTracer    core.ToolTracer
}

// New creates a new execution engine.
func New(
	log *logger.Logger,
	chatClient ChatClient,
	threadMgmt *threadmgmt.Manager,
	notifier core.EventNotifier,
	config core.AgentConfig,
	reqCtxCapture **core.RequestContext,
	toolTracer core.ToolTracer,
) *Engine {
	return &Engine{
		log:           log,
		chatClient:    chatClient,
		threadMgmt:    threadMgmt,
		notifier:      notifier,
		config:        config,
		reqCtxCapture: reqCtxCapture,
		toolTracer:    toolTracer,
	}
}

// RunWithThread processes the LLM streaming response using an AgentThread.
func (e *Engine) RunWithThread(
	ctx context.Context,
	threadID string,
	messageID string,
	thread core.AgentThread,
	runOpts core.RunOptions,
) error {
	return e.RunIteration(ctx, threadID, messageID, thread, runOpts)
}

// RunIteration processes a single LLM iteration and recursively continues if tools are called.
func (e *Engine) RunIteration(
	ctx context.Context,
	threadID string,
	messageID string,
	thread core.AgentThread,
	runOpts core.RunOptions,
) error {
	messages := thread.GetMessages()
	select {
	case <-ctx.Done():
		return fmt.Errorf("client disconnected: %w", ctx.Err())
	default:
	}

	startEvent := &domain.AssistantMessageStartedEvent{
		ThreadID:  threadID,
		MessageID: messageID,
		Timestamp: time.Now(),
	}
	if e.notifier != nil {
		if err := e.notifier.HandleDomainEvent(startEvent); err != nil {
			return err
		}
	}

	llmMsgs := convertCoreMsgsToLLMMsgs(messages)
	chatMessages := llm.FromDomainMessages(llmMsgs)

	chatOptions, reqCtx, err := core.ToChatOptionsWithMiddleware(ctx, e.config, chatMessages, threadID, runOpts)
	if err != nil {
		return fmt.Errorf("failed to apply middleware: %w", err)
	}

	if e.reqCtxCapture != nil {
		*e.reqCtxCapture = reqCtx
	}

	if reqCtx.Terminate {
		return nil
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
				if e.notifier != nil {
					evt := &domain.AssistantMessageCompletedEvent{
						ThreadID:  threadID,
						MessageID: messageID,
						Content:   completeContent,
						AgentName: e.config.Name,
						Timestamp: time.Now(),
					}
					if err := e.notifier.HandleDomainEvent(evt); err != nil {
						e.log.Error(ctx, "failed to emit assistant message completed event on disconnect",
							"error", err, "threadID", threadID, "messageID", messageID)
					}
				}
			}
			return fmt.Errorf("client disconnected during streaming: %w", ctx.Err())
		default:
		}

		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		textDelta := update.Text()
		if textDelta != "" {
			completeContent += textDelta

			contentEvent := &domain.AssistantContentStreamedEvent{
				ThreadID:  threadID,
				MessageID: messageID,
				Delta:     textDelta,
				Timestamp: time.Now(),
			}
			if e.notifier != nil {
				if err := e.notifier.HandleDomainEvent(contentEvent); err != nil {
					return err
				}
			}
		}

		tcUpdate, hasTool := toolCalls.ProcessUpdate(update)
		if hasTool {
			if tcUpdate.IsNew {
				toolStartEvent := &domain.ToolCallStartedEvent{
					ThreadID:        threadID,
					ToolCallID:      tcUpdate.ID,
					ToolCallName:    tcUpdate.Name,
					ParentMessageID: messageID,
					AgentName:       e.config.Name,
					Timestamp:       time.Now(),
				}
				if e.notifier != nil {
					if err := e.notifier.HandleDomainEvent(toolStartEvent); err != nil {
						return err
					}
				}
			}

			if tcUpdate.ArgsDelta != "" {
				argsEvent := &domain.ToolCallArgumentsStreamedEvent{
					ThreadID:   threadID,
					ToolCallID: tcUpdate.ID,
					Delta:      tcUpdate.ArgsDelta,
					Timestamp:  time.Now(),
				}
				if e.notifier != nil {
					if err := e.notifier.HandleDomainEvent(argsEvent); err != nil {
						return err
					}
				}
			}
		}
	}

	toolCallsList := toolCalls.GetAll()

	for _, tc := range toolCallsList {
		completedEvent := &domain.ToolCallCompletedEvent{
			ThreadID:   threadID,
			ToolCallID: tc.ID,
			Timestamp:  time.Now(),
		}
		if e.notifier != nil {
			if err := e.notifier.HandleDomainEvent(completedEvent); err != nil {
				return err
			}
		}
	}

	if toolCalls.HasToolCalls() {
		domainToolCalls := toDomainToolCalls(toolCallsList)
		agg, err := e.threadMgmt.AddAssistantMessage(ctx, threadID, messageID, completeContent, domainToolCalls)
		if err != nil {
			return fmt.Errorf("failed to add assistant message: %w", err)
		}

		// Update local thread state with consistent state from repository
		thread = e.threadMgmt.ToAgentThread(agg)

		assistantEvent := &domain.AssistantMessageCompletedEvent{
			ThreadID:  threadID,
			MessageID: messageID,
			Content:   completeContent,
			ToolCalls: domainToolCalls,
			AgentName: e.config.Name,
			Timestamp: time.Now(),
		}
		if e.notifier != nil {
			if err := e.notifier.HandleDomainEvent(assistantEvent); err != nil {
				return err
			}
		}

		// Execute tools and check for Commands
		shouldContinue := true
		for _, toolCall := range toolCallsList {
			execResult, err := e.executeToolCallWithExecutor(threadID, toolCall, dynamicExecutor)
			var result string
			if err != nil {
				result = fmt.Sprintf(`{"error": "%v"}`, err)
			} else {
				result = execResult.Result
			}

			// Check if this tool wants to stop iteration
			if execResult.StopIteration {
				shouldContinue = false
			}

			toolResultID := core.GenerateID()
			agg, err = e.threadMgmt.AddToolResultMessage(ctx, threadID, toolResultID, toolCall.ID, result)
			if err != nil {
				return fmt.Errorf("failed to add tool result message: %w", err)
			}

			// Update local thread state with consistent state from repository
			thread = e.threadMgmt.ToAgentThread(agg)

			toolEvent := &domain.ToolCallExecutedEvent{
				ThreadID:        threadID,
				MessageID:       toolResultID,
				ToolCallID:      toolCall.ID,
				ToolName:        toolCall.Function.Name,
				ToolArguments:   toolCall.Function.Arguments,
				ToolResult:      result,
				ParentMessageID: messageID,
				AgentName:       e.config.Name,
				Timestamp:       time.Now(),
			}
			if e.notifier != nil {
				if err := e.notifier.HandleDomainEvent(toolEvent); err != nil {
					return err
				}
			}
		}

		if !shouldContinue {
			return nil
		}

		// Continue with the already updated in-memory thread
		continuationMessageID := core.GenerateID()
		return e.RunIteration(ctx, threadID, continuationMessageID, thread, runOpts)
	}

	if _, err := e.threadMgmt.AddAssistantMessage(ctx, threadID, messageID, completeContent, nil); err != nil {
		return fmt.Errorf("failed to add final assistant message: %w", err)
	}

	assistantEvent := &domain.AssistantMessageCompletedEvent{
		ThreadID:  threadID,
		MessageID: messageID,
		Content:   completeContent,
		AgentName: e.config.Name,
		Timestamp: time.Now(),
	}
	if e.notifier != nil {
		if err := e.notifier.HandleDomainEvent(assistantEvent); err != nil {
			return err
		}
	}

	return nil
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
