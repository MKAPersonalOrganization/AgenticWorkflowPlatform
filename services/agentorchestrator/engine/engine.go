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
		execState := &executionState{
			ctx:       ctx,
			threadID:  runCtx.ThreadID,
			messageID: runCtx.MessageID,
			thread:    runCtx.Thread,
			options:   runCtx.Options,
			yield:     yield,
		}

		// Configure State Machine
		execState.sm = stateless.NewStateMachine(stateStart)

		execState.sm.Configure(stateStart).
			OnExit(func(_ context.Context, _ ...any) error {
				return e.handleStart(execState)
			}).
			Permit(triggerStart, stateStream)

		execState.sm.Configure(stateStream).
			OnEntry(func(_ context.Context, _ ...any) error {
				return e.handleStream(execState)
			}).
			Permit(triggerToolsFound, stateTools).
			Permit(triggerStreamDone, stateDone)

		execState.sm.Configure(stateTools).
			OnEntry(func(_ context.Context, _ ...any) error {
				return e.handleTools(execState)
			}).
			Permit(triggerToolsContinue, stateStream).
			Permit(triggerToolsDone, stateDone)

		// Start execution
		if err := execState.sm.Fire(triggerStart); err != nil {
			yield(nil, err)
		}
	}
}

// executionState holds the state for a single execution run.
type executionState struct {
	ctx   context.Context
	yield func(core.Event, error) bool

	// Run context
	threadID  string
	messageID string
	thread    core.AgentThread
	options   core.RunOptions

	// Iteration state
	streamResult *StreamResult
	executor     core.ToolExecutor

	// State Machine
	sm *stateless.StateMachine
}

func (s *executionState) emit(event core.Event) bool {
	if !s.yield(event, nil) {
		return false
	}
	return true
}

func (e *Engine) handleStart(s *executionState) error {
	if !s.emit(e.events.AssistantStarted(s.threadID, s.messageID)) {
		return context.Canceled
	}
	return nil
}

func (e *Engine) handleStream(s *executionState) error {
	// Prepare
	prepareCtx := core.RunContext{
		ThreadID:  s.threadID,
		MessageID: s.messageID,
		Thread:    s.thread,
		Options:   s.options,
	}
	prepareResult, err := e.llmCtx.Prepare(s.ctx, prepareCtx)
	if err != nil {
		return err
	}
	if prepareResult.Terminated {
		return s.sm.Fire(triggerStreamDone)
	}

	// Stream
	llmContext := prepareResult.Context
	s.executor = e.toolExec.CreateExecutor(
		s.ctx, llmContext.GetEffectiveTools(),
		s.threadID, s.messageID, llmContext.RequestCtx,
	)
	streamIter := e.chatClient.GetStreamingResponse(s.ctx, llmContext.Messages, llmContext.Options)
	res, err := e.stream.Process(s.ctx, streamIter, ProcessParams{
		ThreadID:  s.threadID,
		MessageID: s.messageID,
	}, s.yield)
	if err != nil {
		return err
	}
	if res == nil {
		// Verify if we should consider this terminated or just done
		return s.sm.Fire(triggerStreamDone)
	}
	s.streamResult = res

	// Persist and emit completed
	toolCalls := toDomainToolCalls(s.streamResult.ToolCalls)
	updatedThread, err := e.llmCtx.AddAssistantMessage(s.ctx, s.threadID, s.messageID, s.streamResult.Content, toolCalls)
	if err != nil {
		return err
	}
	s.thread = updatedThread

	if !s.emit(e.events.AssistantCompleted(s.threadID, s.messageID, s.streamResult.Content, toolCalls)) {
		return context.Canceled
	}

	if s.streamResult.HasToolCalls() {
		return s.sm.Fire(triggerToolsFound)
	}
	return s.sm.Fire(triggerStreamDone)
}

func (e *Engine) handleTools(s *executionState) error {
	shouldStop := false

	for _, tc := range s.streamResult.ToolCalls {
		if !s.emit(e.events.ToolCallCompleted(s.threadID, tc.ID)) {
			return context.Canceled
		}

		result := e.toolExec.Execute(tc, s.executor)

		toolResultID := core.GenerateID()
		updatedThread, err := e.llmCtx.AddToolResultMessage(s.ctx, s.threadID, toolResultID, result.ToolCallID, result.Result)
		if err != nil {
			return err
		}
		s.thread = updatedThread

		if !s.emit(e.events.ToolCallExecuted(
			s.threadID, toolResultID, result.ToolCallID,
			result.ToolName, result.Arguments, result.Result, s.messageID,
		)) {
			return context.Canceled
		}

		if result.StopIteration {
			shouldStop = true
		}
	}

	if shouldStop {
		return s.sm.Fire(triggerToolsDone)
	}
	s.messageID = core.GenerateID()
	return s.sm.Fire(triggerToolsContinue)
}

// State machine states
type state int

const (
	stateStart state = iota
	stateStream
	stateTools
	stateDone
	stateError
)

const (
	triggerStart         = "Start"
	triggerToolsFound    = "ToolsFound"
	triggerStreamDone    = "StreamDone"
	triggerToolsContinue = "ToolsContinue"
	triggerToolsDone     = "ToolsDone"
)


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




