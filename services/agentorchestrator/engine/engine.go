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
func (e *Engine) Run(ctx context.Context, runCtx core.RunContext) iter.Seq2[core.Event, error] {
	return func(yield func(core.Event, error) bool) {
		m := &machine{
			engine:    e,
			ctx:       ctx,
			yield:     yield,
			threadID:  runCtx.ThreadID,
			messageID: runCtx.MessageID,
			thread:    runCtx.Thread,
			options:   runCtx.Options,
		}
		m.state = StateStart
		m.run()
	}
}

func (m *machine) emit(event core.Event) bool {
	if !m.yield(event, nil) {
		m.state = StateDone
		return false
	}
	return true
}





// State machine states
type State int

const (
	StateStart State = iota
	StateStream
	StateTools
	StateDone
	StateError
)

type TransitionData struct {
	Terminated bool
	HasTools   bool
	Stop       bool
}

// machine implements the engine state machine.
type machine struct {
	engine *Engine
	ctx    context.Context
	yield  func(core.Event, error) bool

	// Run context
	threadID  string
	messageID string
	thread    core.AgentThread
	options   core.RunOptions

	// Iteration state
	streamResult *StreamResult
	executor     core.ToolExecutor

	state State
	err   error
}

func (m *machine) run() {
	for m.state != StateDone && m.state != StateError {
		data, err := m.executeHandler()
		if err != nil {
			m.err = err
			m.state = StateError
			break
		}
		m.state = m.Transition(m.state, data)
	}

	if m.err != nil {
		m.yield(nil, m.err)
	}
}

func (m *machine) executeHandler() (TransitionData, error) {
	switch m.state {
	case StateStart:
		return TransitionData{}, m.HandleStart()
	case StateStream:
		terminated, hasTools, err := m.HandleStream()
		return TransitionData{Terminated: terminated, HasTools: hasTools}, err
	case StateTools:
		stop, err := m.HandleTools()
		return TransitionData{Stop: stop}, err
	default:
		return TransitionData{}, nil
	}
}

func (m *machine) Transition(currentState State, data TransitionData) State {
	switch currentState {
	case StateStart:
		return StateStream
	case StateStream:
		if data.Terminated {
			return StateDone
		}
		if data.HasTools {
			return StateTools
		}
		return StateDone
	case StateTools:
		if data.Stop {
			return StateDone
		}
		m.messageID = core.GenerateID()
		return StateStream
	default:
		return StateDone
	}
}

// HandleStart emits the AssistantStarted event.
func (m *machine) HandleStart() error {
	if !m.emit(m.engine.events.AssistantStarted(m.threadID, m.messageID)) {
		return context.Canceled
	}
	return nil
}

// HandleStream prepares context, streams LLM response, and emits events.
func (m *machine) HandleStream() (terminated bool, hasTools bool, err error) {
	// Prepare
	runCtx := core.RunContext{
		ThreadID:  m.threadID,
		MessageID: m.messageID,
		Thread:    m.thread,
		Options:   m.options,
	}
	prepareResult, err := m.engine.llmCtx.Prepare(m.ctx, runCtx)
	if err != nil {
		return false, false, err
	}
	if prepareResult.Terminated {
		return true, false, nil
	}

	// Stream
	llmContext := prepareResult.Context
	m.executor = m.engine.toolExec.CreateExecutor(
		m.ctx, llmContext.GetEffectiveTools(),
		m.threadID, m.messageID, llmContext.RequestCtx,
	)
	streamIter := m.engine.chatClient.GetStreamingResponse(m.ctx, llmContext.Messages, llmContext.Options)
	streamResult, err := m.engine.stream.Process(m.ctx, streamIter, ProcessParams{
		ThreadID:  m.threadID,
		MessageID: m.messageID,
	}, m.yield)
	if err != nil {
		return false, false, err
	}
	if streamResult == nil {
		// Verify if we should consider this terminated or just done
		return true, false, nil
	}
	m.streamResult = streamResult

	// Persist and emit completed
	toolCalls := toDomainToolCalls(m.streamResult.ToolCalls)
	thread, err := m.engine.llmCtx.AddAssistantMessage(m.ctx, m.threadID, m.messageID, m.streamResult.Content, toolCalls)
	if err != nil {
		return false, false, err
	}
	m.thread = thread

	if !m.emit(m.engine.events.AssistantCompleted(m.threadID, m.messageID, m.streamResult.Content, toolCalls)) {
		return false, false, context.Canceled
	}

	return false, m.streamResult.HasToolCalls(), nil
}

// HandleTools executes tool calls and emits results.
func (m *machine) HandleTools() (stop bool, err error) {
	shouldStop := false

	for _, tc := range m.streamResult.ToolCalls {
		if !m.emit(m.engine.events.ToolCallCompleted(m.threadID, tc.ID)) {
			return false, context.Canceled
		}

		result := m.engine.toolExec.Execute(tc, m.executor)

		toolResultID := core.GenerateID()
		thread, err := m.engine.llmCtx.AddToolResultMessage(m.ctx, m.threadID, toolResultID, result.ToolCallID, result.Result)
		if err != nil {
			return false, err
		}
		m.thread = thread

		if !m.emit(m.engine.events.ToolCallExecuted(
			m.threadID, toolResultID, result.ToolCallID,
			result.ToolName, result.Arguments, result.Result, m.messageID,
		)) {
			return false, context.Canceled
		}

		if result.StopIteration {
			shouldStop = true
		}
	}

	return shouldStop, nil
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


