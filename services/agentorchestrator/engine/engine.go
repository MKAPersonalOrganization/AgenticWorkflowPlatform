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
type (
	ChatClient                = types.ChatClient
	ThreadStore               = types.ThreadStore
	ShouldContinueIterationFn = types.ShouldContinueIterationFn
)

// Engine handles the core LLM iteration and streaming logic.
type Engine struct {
	log                     *logger.Logger
	chatClient              ChatClient
	threadMgmt              *threadmgmt.Manager
	notifier                core.EventNotifier
	readModel               ThreadStore
	config                  core.AgentConfig
	shouldContinueIteration ShouldContinueIterationFn
	reqCtxCapture           **core.RequestContext
	toolTracer              core.ToolTracer
}

// New creates a new execution engine.
func New(
	log *logger.Logger,
	chatClient ChatClient,
	threadMgmt *threadmgmt.Manager,
	notifier core.EventNotifier,
	readModel ThreadStore,
	config core.AgentConfig,
	shouldContinueIteration ShouldContinueIterationFn,
	reqCtxCapture **core.RequestContext,
	toolTracer core.ToolTracer,
) *Engine {
	return &Engine{
		log:                     log,
		chatClient:              chatClient,
		threadMgmt:              threadMgmt,
		notifier:                notifier,
		readModel:               readModel,
		config:                  config,
		shouldContinueIteration: shouldContinueIteration,
		reqCtxCapture:           reqCtxCapture,
		toolTracer:              toolTracer,
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
	messages := thread.GetMessages()
	return e.RunIteration(ctx, threadID, messageID, messages, thread, runOpts)
}

// RunIteration processes a single LLM iteration and recursively continues if tools are called.
func (e *Engine) RunIteration(
	ctx context.Context,
	threadID string,
	messageID string,
	messages []domain.Message,
	thread core.AgentThread,
	runOpts core.RunOptions,
) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("client disconnected: %w", ctx.Err())
	default:
	}

	startEvent := &core.AssistantMessageStartedEvent{
		BaseEvent: core.BaseEvent{
			Type:      "AssistantMessageStartedEvent",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		MessageID: messageID,
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
					evt := &core.AssistantMessageCompletedEvent{
						BaseEvent: core.BaseEvent{
							Type:      "AssistantMessageCompletedEvent",
							Timestamp: time.Now(),
							ThreadID:  threadID,
						},
						MessageID: messageID,
						Content:   completeContent,
						AgentName: e.config.Name,
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

			contentEvent := &core.AssistantContentStreamedEvent{
				BaseEvent: core.BaseEvent{
					Type:      "AssistantContentStreamedEvent",
					Timestamp: time.Now(),
					ThreadID:  threadID,
				},
				MessageID: messageID,
				Delta:     textDelta,
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
				toolStartEvent := &core.ToolCallStartedEvent{
					BaseEvent: core.BaseEvent{
						Type:      "ToolCallStartedEvent",
						Timestamp: time.Now(),
						ThreadID:  threadID,
					},
					ToolCallID:      tcUpdate.ID,
					ToolCallName:    tcUpdate.Name,
					ParentMessageID: messageID,
					AgentName:       e.config.Name,
				}
				if e.notifier != nil {
					if err := e.notifier.HandleDomainEvent(toolStartEvent); err != nil {
						return err
					}
				}
			}

			if tcUpdate.ArgsDelta != "" {
				argsEvent := &core.ToolCallArgumentsStreamedEvent{
					BaseEvent: core.BaseEvent{
						Type:      "ToolCallArgumentsStreamedEvent",
						Timestamp: time.Now(),
						ThreadID:  threadID,
					},
					ToolCallID: tcUpdate.ID,
					Delta:      tcUpdate.ArgsDelta,
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
		completedEvent := &core.ToolCallCompletedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "ToolCallCompletedEvent",
				Timestamp: time.Now(),
				ThreadID:  threadID,
			},
			ToolCallID: tc.ID,
		}
		if e.notifier != nil {
			if err := e.notifier.HandleDomainEvent(completedEvent); err != nil {
				return err
			}
		}
	}

	if toolCalls.HasToolCalls() {
		err = e.threadMgmt.AddAssistantMessage(ctx, threadID, messageID, completeContent, toolCallsList)
		if err != nil {
			return fmt.Errorf("failed to add assistant message: %w", err)
		}

		assistantEvent := &core.AssistantMessageCompletedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "AssistantMessageCompletedEvent",
				Timestamp: time.Now(),
				ThreadID:  threadID,
			},
			MessageID: messageID,
			Content:   completeContent,
			ToolCalls: toolCallsList,
			AgentName: e.config.Name,
		}
		if e.notifier != nil {
			if err := e.notifier.HandleDomainEvent(assistantEvent); err != nil {
				return err
			}
		}

		for _, toolCall := range toolCallsList {
			result, err := e.executeToolCallWithExecutor(threadID, toolCall, dynamicExecutor)
			if err != nil {
				result = fmt.Sprintf(`{"error": "%v"}`, err)
			}

			toolResultID := core.GenerateID()
			err = e.threadMgmt.AddToolResultMessage(ctx, threadID, toolResultID, toolCall.ID, result)
			if err != nil {
				return fmt.Errorf("failed to add tool result message: %w", err)
			}

			toolEvent := &core.ToolCallExecutedEvent{
				BaseEvent: core.BaseEvent{
					Type:      "ToolCallExecutedEvent",
					Timestamp: time.Now(),
					ThreadID:  threadID,
				},
				MessageID:       toolResultID,
				ToolCallID:      toolCall.ID,
				ToolName:        toolCall.Function.Name,
				ToolArguments:   toolCall.Function.Arguments,
				ToolResult:      result,
				ParentMessageID: messageID,
				AgentName:       e.config.Name,
			}
			if e.notifier != nil {
				if err := e.notifier.HandleDomainEvent(toolEvent); err != nil {
					return err
				}
			}
		}

		if e.shouldContinueIteration != nil && !e.shouldContinueIteration(toolCallsList) {
			return nil
		}

		stateWithAccess, ok := e.readModel.(core.ThreadStateWithAgentAccess)
		if !ok {
			return fmt.Errorf("state does not implement ThreadStateWithAgentAccess")
		}

		updatedThread, exists := stateWithAccess.GetAgentThread(threadID)
		if !exists {
			var err error
			updatedThread, err = stateWithAccess.GetOrCreateAgentThread(ctx, threadID)
			if err != nil {
				return fmt.Errorf("failed to get/create thread %s after tool execution: %w", threadID, err)
			}
		}

		continuationMessageID := core.GenerateID()
		return e.RunIteration(ctx, threadID, continuationMessageID, updatedThread.GetMessages(), thread, runOpts)
	}

	err = e.threadMgmt.AddAssistantMessage(ctx, threadID, messageID, completeContent, nil)
	if err != nil {
		return fmt.Errorf("failed to add final assistant message: %w", err)
	}

	assistantEvent := &core.AssistantMessageCompletedEvent{
		BaseEvent: core.BaseEvent{
			Type:      "AssistantMessageCompletedEvent",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		MessageID: messageID,
		Content:   completeContent,
		AgentName: e.config.Name,
	}
	if e.notifier != nil {
		if err := e.notifier.HandleDomainEvent(assistantEvent); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) executeToolCallWithExecutor(threadID string, toolCall domain.ToolCall, executor core.ToolExecutor) (string, error) {
	result, err := executor.Execute(toolCall)
	if err != nil {
		return result, fmt.Errorf("failed to execute tool call %s: %w", toolCall.Function.Name, err)
	}
	return result, nil
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
