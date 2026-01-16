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

// AgentProvider provides agent configurations by name.
type AgentProvider interface {
	GetConfig(name string) (core.AgentConfig, error)
}

// Engine handles the core LLM iteration and streaming logic in a generic way.
type Engine struct {
	log          *logger.Logger
	chatClient   ChatClient
	provider     AgentProvider
	threadMgmt   *threadmgmt.Manager
	reqCtxCapture **core.RequestContext // optional capture for testing
	toolTracer   core.ToolTracer
	toolExec     *ToolExecutionService
}

// New creates a new generic execution engine.
func New(
	log *logger.Logger,
	chatClient ChatClient,
	threadMgmt *threadmgmt.Manager,
	provider AgentProvider,
	reqCtxCapture **core.RequestContext,
	toolTracer core.ToolTracer,
) *Engine {
	return &Engine{
		log:           log,
		chatClient:    chatClient,
		provider:      provider,
		threadMgmt:    threadMgmt,
		reqCtxCapture: reqCtxCapture,
		toolTracer:    toolTracer,
		toolExec:      NewToolExecutionService(toolTracer),
	}
}

// Run processes the LLM response using an AgentThread and yields events.
func (e *Engine) Run(ctx context.Context, agentName string, runCtx core.RunContext) iter.Seq2[core.Event, error] {
	return func(yield func(core.Event, error) bool) {
		execState := &executionState{
			ctx:            ctx,
			threadID:       runCtx.ThreadID,
			messageID:      runCtx.MessageID,
			thread:         runCtx.Thread,
			options:        runCtx.Options,
			yield:          yield,
			agentName:      agentName,
		}

		// Initialize initial agent components
		config, err := e.provider.GetConfig(agentName)
		if err != nil {
			yield(nil, err)
			return
		}
		execState.llmCtx = NewLLMContextManager(config, e.threadMgmt, e.reqCtxCapture)
		execState.stream = NewStreamProcessor(config.Name)
		execState.events = NewEventFactory(config.Name)


		// Configure State Machine
		execState.sm = stateless.NewStateMachine(stateStart)

		execState.sm.Configure(stateStart).
			OnEntry(func(_ context.Context, _ ...any) error {
				return execState.sm.Fire(triggerStart)
			}).
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
			Permit(triggerToolsHandoff, stateSwitch)

		execState.sm.Configure(stateSwitch).
			OnEntry(func(_ context.Context, args ...any) error {
				targetAgent := args[0].(string)
				return e.handleSwitch(execState, targetAgent)
			}).
			Permit(triggerSwitchDone, stateStart)

		// Start execution
		if err := execState.sm.Fire(triggerStart); err != nil {
			yield(nil, err)
		}
	}
}

// executionState holds the state and context for an execution run.
type executionState struct {
	ctx   context.Context
	yield func(core.Event, error) bool

	// Run context
	threadID  string
	messageID string
	thread    core.AgentThread
	options   core.RunOptions
	agentName string

	// Components (Mutable per agent)
	llmCtx *LLMContextManager
	stream *StreamProcessor
	events *EventFactory

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
	if !s.emit(s.events.AssistantStarted(s.threadID, s.messageID)) {
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
	prepareResult, err := s.llmCtx.Prepare(s.ctx, prepareCtx)
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
	res, err := s.stream.Process(s.ctx, streamIter, ProcessParams{
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
	updatedThread, err := s.llmCtx.AddAssistantMessage(s.ctx, s.threadID, s.messageID, s.streamResult.Content, toolCalls)
	if err != nil {
		return err
	}
	s.thread = updatedThread

	if !s.emit(s.events.AssistantCompleted(s.threadID, s.messageID, s.streamResult.Content, toolCalls)) {
		return context.Canceled
	}

	if s.streamResult.HasToolCalls() {
		return s.sm.Fire(triggerToolsFound)
	}
	return s.sm.Fire(triggerStreamDone)
}

func (e *Engine) handleTools(s *executionState) error {
	var handoffTarget string

	for _, tc := range s.streamResult.ToolCalls {
		if !s.emit(s.events.ToolCallCompleted(s.threadID, tc.ID)) {
			return context.Canceled
		}

		result := e.toolExec.Execute(tc, s.executor)

		toolResultID := core.GenerateID()
		updatedThread, err := s.llmCtx.AddToolResultMessage(s.ctx, s.threadID, toolResultID, result.ToolCallID, result.Result)
		if err != nil {
			return err
		}
		s.thread = updatedThread

		if !s.emit(s.events.ToolCallExecuted(
			s.threadID, toolResultID, result.ToolCallID,
			result.ToolName, result.Arguments, result.Result, s.messageID,
		)) {
			return context.Canceled
		}

		// Check for routing command
		if result.Command != nil && result.Command.Goto != "" {
			handoffTarget = result.Command.Goto
		}
	}

	if handoffTarget != "" {
		return s.sm.Fire(triggerToolsHandoff, handoffTarget)
	}

	s.messageID = core.GenerateID()
	return s.sm.Fire(triggerToolsContinue)
}

func (e *Engine) handleSwitch(s *executionState, targetAgent string) error {
	if !s.emit(s.events.AgentHandoff(s.threadID, targetAgent)) {
		return context.Canceled
	}

	newConfig, err := e.provider.GetConfig(targetAgent)
	if err != nil {
		return err
	}

	s.agentName = targetAgent
	s.llmCtx = NewLLMContextManager(newConfig, e.threadMgmt, e.reqCtxCapture)
	s.stream = NewStreamProcessor(newConfig.Name)
	s.events = NewEventFactory(newConfig.Name)
	
	// Start next iteration with fresh message ID
	s.messageID = core.GenerateID()
	
	return s.sm.Fire(triggerSwitchDone)
}

// State machine states
type state int

const (
	stateStart state = iota
	stateStream
	stateTools
	stateSwitch
	stateDone
	stateError
)

const (
	triggerStart           = "Start"
	triggerToolsFound      = "ToolsFound"
	triggerStreamDone      = "StreamDone"
	triggerToolsContinue   = "ToolsContinue"
	triggerToolsHandoff    = "ToolsHandoff"
	triggerSwitchDone      = "SwitchDone"
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




