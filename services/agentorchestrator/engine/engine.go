package engine

import (
	"context"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/agentorchestrator/types"
	"school-with-ai/services/threadmgmt"


	"github.com/qmuntal/stateless"
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
func (e *Engine) Run(ctx context.Context, runCtx core.RunContext) iter.Seq2[core.Event, error] {
	return func(yield func(core.Event, error) bool) {
		// State variables
		var (
			// Context
			threadID  = runCtx.ThreadID
			messageID = runCtx.MessageID
			thread    = runCtx.Thread
			options   = runCtx.Options

			// Iteration state
			streamResult *StreamResult
			executor     core.ToolExecutor
			
			// State Machine
			sm *stateless.StateMachine
		)

		// State machine states
		type State int
		const (
			StateStart State = iota
			StateStream
			StateTools
			StateDone
			StateError
		)

		const (
			TriggerStart           = "Start"
			TriggerStreamProcessed = "StreamProcessed"
			TriggerToolsExecuted   = "ToolsExecuted"
		)

		// Helpers
		emit := func(event core.Event) bool {
			if !yield(event, nil) {
				return false
			}
			return true
		}

		type transitionData struct {
			Terminated bool
			HasTools   bool
			Stop       bool
		}

		// Handlers
		handleStart := func(ctx context.Context, args ...interface{}) error {
			if !emit(e.events.AssistantStarted(threadID, messageID)) {
				return context.Canceled
			}
			return nil
		}

		handleStream := func(ctx context.Context, args ...interface{}) error {
			// Prepare
			prepareCtx := core.RunContext{
				ThreadID:  threadID,
				MessageID: messageID,
				Thread:    thread,
				Options:   options,
			}
			prepareResult, err := e.llmCtx.Prepare(ctx, prepareCtx)
			if err != nil {
				return err
			}
			if prepareResult.Terminated {
				return sm.Fire(TriggerStreamProcessed, transitionData{Terminated: true})
			}

			// Stream
			llmContext := prepareResult.Context
			executor = e.toolExec.CreateExecutor(
				ctx, llmContext.GetEffectiveTools(),
				threadID, messageID, llmContext.RequestCtx,
			)
			streamIter := e.chatClient.GetStreamingResponse(ctx, llmContext.Messages, llmContext.Options)
			res, err := e.stream.Process(ctx, streamIter, ProcessParams{
				ThreadID:  threadID,
				MessageID: messageID,
			}, yield)
			if err != nil {
				return err
			}
			if res == nil {
				// Verify if we should consider this terminated or just done
				return sm.Fire(TriggerStreamProcessed, transitionData{Terminated: true})
			}
			streamResult = res

			// Persist and emit completed
			toolCalls := toDomainToolCalls(streamResult.ToolCalls)
			updatedThread, err := e.llmCtx.AddAssistantMessage(ctx, threadID, messageID, streamResult.Content, toolCalls)
			if err != nil {
				return err
			}
			thread = updatedThread

			if !emit(e.events.AssistantCompleted(threadID, messageID, streamResult.Content, toolCalls)) {
				return context.Canceled
			}

			return sm.Fire(TriggerStreamProcessed, transitionData{
				HasTools: streamResult.HasToolCalls(),
			})
		}

		handleTools := func(ctx context.Context, args ...interface{}) error {
			shouldStop := false

			for _, tc := range streamResult.ToolCalls {
				if !emit(e.events.ToolCallCompleted(threadID, tc.ID)) {
					return context.Canceled
				}

				result := e.toolExec.Execute(tc, executor)

				toolResultID := core.GenerateID()
				updatedThread, err := e.llmCtx.AddToolResultMessage(ctx, threadID, toolResultID, result.ToolCallID, result.Result)
				if err != nil {
					return err
				}
				thread = updatedThread

				if !emit(e.events.ToolCallExecuted(
					threadID, toolResultID, result.ToolCallID,
					result.ToolName, result.Arguments, result.Result, messageID,
				)) {
					return context.Canceled
				}

				if result.StopIteration {
					shouldStop = true
				}
			}

			return sm.Fire(TriggerToolsExecuted, transitionData{Stop: shouldStop})
		}

		// Configure State Machine
		sm = stateless.NewStateMachine(StateStart)

		sm.Configure(StateStart).
			OnExit(handleStart).
			Permit(TriggerStart, StateStream)

		sm.Configure(StateStream).
			OnEntry(handleStream).
			PermitDynamic(TriggerStreamProcessed, func(ctx context.Context, args ...interface{}) (stateless.State, error) {
				data := args[0].(transitionData)
				if data.Terminated {
					return StateDone, nil
				}
				if data.HasTools {
					return StateTools, nil
				}
				return StateDone, nil
			})

		sm.Configure(StateTools).
			OnEntry(handleTools).
			PermitDynamic(TriggerToolsExecuted, func(ctx context.Context, args ...interface{}) (stateless.State, error) {
				data := args[0].(transitionData)
				if data.Stop {
					return StateDone, nil
				}
				messageID = core.GenerateID()
				return StateStream, nil
			})

		// Start execution
		if err := sm.Fire(TriggerStart); err != nil {
			yield(nil, err)
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




