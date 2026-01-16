package engine

import (
	"context"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/agentorchestrator/types"
	"school-with-ai/services/threadmgmt"
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
	stream     *StreamProcessor
	events     *EventFactory
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
		stream:     NewStreamProcessor(config.Name),
		events:     NewEventFactory(config.Name),
	}
}

// Run processes the LLM response using an AgentThread and yields events.
func (e *Engine) Run(
	ctx context.Context,
	runCtx core.RunContext,
) iter.Seq2[core.Event, error] {
	return func(yield func(core.Event, error) bool) {
		// Emit start event once at the beginning
		if !yield(e.events.AssistantStarted(runCtx.ThreadID, runCtx.MessageID), nil) {
			return
		}

		// Main iteration loop - continues while there are tool calls to process
		for {
			prepareResult, err := e.llmCtx.Prepare(ctx, runCtx)
			if err != nil {
				yield(nil, err)
				return
			}
			if prepareResult.Terminated {
				return
			}

			llmContext := prepareResult.Context
			executor := e.toolExec.CreateExecutor(ctx, llmContext.GetEffectiveTools(), runCtx.ThreadID, runCtx.MessageID, llmContext.RequestCtx)

			streamIter := e.chatClient.GetStreamingResponse(ctx, llmContext.Messages, llmContext.Options)

			streamResult, err := e.stream.Process(ctx, streamIter, ProcessParams{
				ThreadID:  runCtx.ThreadID,
				MessageID: runCtx.MessageID,
			}, yield)
			if err != nil {
				yield(nil, err)
				return
			}
			if streamResult == nil {
				return
			}

			// Emit tool call completed events
			for _, tc := range streamResult.ToolCalls {
				if !yield(e.events.ToolCallCompleted(runCtx.ThreadID, tc.ID), nil) {
					return
				}
			}

			// No tool calls - persist final message and exit
			if !streamResult.HasToolCalls() {
				if _, err := e.llmCtx.AddAssistantMessage(ctx, runCtx.ThreadID, runCtx.MessageID, streamResult.Content, nil); err != nil {
					yield(nil, err)
					return
				}
				yield(e.events.AssistantCompleted(runCtx.ThreadID, runCtx.MessageID, streamResult.Content, nil), nil)
				return
			}

			// Has tool calls - persist, execute, and continue loop
			domainToolCalls := toDomainToolCalls(streamResult.ToolCalls)
			thread, err := e.llmCtx.AddAssistantMessage(ctx, runCtx.ThreadID, runCtx.MessageID, streamResult.Content, domainToolCalls)
			if err != nil {
				yield(nil, err)
				return
			}
			runCtx.Thread = thread

			if !yield(e.events.AssistantCompleted(runCtx.ThreadID, runCtx.MessageID, streamResult.Content, domainToolCalls), nil) {
				return
			}

			// Execute all tools
			execResults := e.toolExec.ExecuteAll(streamResult.ToolCalls, executor)

			// Persist results and emit events
			for _, result := range execResults.Results {
				toolResultID := core.GenerateID()
				thread, err = e.llmCtx.AddToolResultMessage(ctx, runCtx.ThreadID, toolResultID, result.ToolCallID, result.Result)
				if err != nil {
					yield(nil, err)
					return
				}
				runCtx.Thread = thread

				if !yield(e.events.ToolCallExecuted(runCtx.ThreadID, toolResultID, result.ToolCallID, result.ToolName, result.Arguments, result.Result, runCtx.MessageID), nil) {
					return
				}
			}

			// Check if tools signaled to stop
			if execResults.ShouldStop {
				return
			}

			// Continue loop with new message ID
			runCtx.MessageID = core.GenerateID()
		}
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


