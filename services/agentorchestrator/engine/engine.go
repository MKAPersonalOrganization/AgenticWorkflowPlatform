package engine

import (
	"context"
	"iter"
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
	log           *logger.Logger
	chatClient    ChatClient
	provider      AgentProvider
	threadMgmt    *threadmgmt.Manager
	reqCtxCapture **core.RequestContext // optional capture for testing
	toolTracer    core.ToolTracer
	toolExec      *ToolExecutionService
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
		exec := &executionState{
			engine:    e,
			ctx:       ctx,
			yield:     yield,
			threadID:  runCtx.ThreadID,
			messageID: runCtx.MessageID,
			thread:    runCtx.Thread,
			options:   runCtx.Options,
			agentName: agentName,
		}

		if err := exec.initAgentComponents(agentName); err != nil {
			yield(nil, err)
			return
		}

		if err := exec.run(); err != nil {
			yield(nil, err)
		}
	}
}

// executionState holds the state and context for an execution run.
type executionState struct {
	engine *Engine
	ctx    context.Context
	yield  func(core.Event, error) bool

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
	return s.yield(event, nil)
}

func (s *executionState) initAgentComponents(agentName string) error {
	config, err := s.engine.provider.GetConfig(agentName)
	if err != nil {
		return err
	}

	s.agentName = agentName
	s.llmCtx = NewLLMContextManager(config, s.engine.threadMgmt, s.engine.reqCtxCapture)
	s.events = NewEventFactory(config.Name)
	s.stream = NewStreamProcessor(s.events)
	return nil
}

func (s *executionState) setup() error {
	// Configure State Machine
	s.sm = stateless.NewStateMachine(stateStart)

	s.sm.Configure(stateStart).
		OnEntry(func(_ context.Context, _ ...any) error {
			return s.sm.Fire(triggerStart)
		}).
		OnExit(func(_ context.Context, _ ...any) error {
			return s.handleStart()
		}).
		Permit(triggerStart, stateStream)

	s.sm.Configure(stateStream).
		OnEntry(func(_ context.Context, _ ...any) error {
			return s.handleStream()
		}).
		Permit(triggerToolsFound, stateTools).
		Permit(triggerStreamDone, stateDone)

	s.sm.Configure(stateTools).
		OnEntry(func(_ context.Context, _ ...any) error {
			return s.handleTools()
		}).
		Permit(triggerToolsContinue, stateStream).
		Permit(triggerToolsHandoff, stateSwitch)

	s.sm.Configure(stateSwitch).
		OnEntry(func(_ context.Context, args ...any) error {
			targetAgent := args[0].(string)
			return s.handleSwitch(targetAgent)
		}).
		Permit(triggerSwitchDone, stateStart)

	return nil
}

func (s *executionState) run() error {
	if s.sm == nil {
		if err := s.setup(); err != nil {
			return err
		}
	}
	return s.sm.Fire(triggerStart)
}

func (s *executionState) handleStart() error {
	if !s.emit(s.events.AssistantStarted(s.threadID, s.messageID)) {
		return context.Canceled
	}
	return nil
}

func (s *executionState) handleStream() error {
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
	s.executor = s.engine.toolExec.CreateExecutor(
		s.ctx, llmContext.GetEffectiveTools(),
		s.threadID, s.messageID, llmContext.RequestCtx,
	)
	streamIter := s.engine.chatClient.GetStreamingResponse(s.ctx, llmContext.Messages, llmContext.Options)
	res, err := s.stream.Process(s.ctx, streamIter, ProcessParams{
		ThreadID:  s.threadID,
		MessageID: s.messageID,
	}, s.yield)
	if err != nil {
		return err
	}
	if res == nil {
		return s.sm.Fire(triggerStreamDone)
	}
	s.streamResult = res

	// Persist and emit completed
	toolCalls := toDomainToolCalls(s.streamResult.ToolCalls)
	agg, err := s.engine.threadMgmt.AddAssistantMessage(s.ctx, s.threadID, s.messageID, s.streamResult.Content, toolCalls, s.agentName)
	if err != nil {
		return err
	}
	s.thread = s.engine.threadMgmt.ToAgentThread(agg)

	if !s.emit(s.events.AssistantCompleted(s.threadID, s.messageID, s.streamResult.Content, toolCalls)) {
		return context.Canceled
	}

	if s.streamResult.HasToolCalls() {
		return s.sm.Fire(triggerToolsFound)
	}
	return s.sm.Fire(triggerStreamDone)
}

func (s *executionState) handleTools() error {
	var handoffTarget string

	for _, tc := range s.streamResult.ToolCalls {
		if !s.emit(s.events.ToolCallCompleted(s.threadID, tc.ID)) {
			return context.Canceled
		}

		result := s.engine.toolExec.Execute(tc, s.executor)

		toolResultID := core.GenerateID()
		agg, err := s.engine.threadMgmt.AddToolResultMessage(s.ctx, s.threadID, toolResultID, result.ToolCallID, result.Result, s.agentName)
		if err != nil {
			return err
		}
		s.thread = s.engine.threadMgmt.ToAgentThread(agg)

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

func (s *executionState) handleSwitch(targetAgent string) error {
	if !s.emit(s.events.AgentHandoff(s.threadID, targetAgent)) {
		return context.Canceled
	}

	if err := s.initAgentComponents(targetAgent); err != nil {
		return err
	}

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
	triggerStart         = "Start"
	triggerToolsFound    = "ToolsFound"
	triggerStreamDone    = "StreamDone"
	triggerToolsContinue = "ToolsContinue"
	triggerToolsHandoff  = "ToolsHandoff"
	triggerSwitchDone    = "SwitchDone"
)




