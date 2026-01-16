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
			state:     stateStart,
		}
		m.run()
	}
}

// State machine states
type state int

const (
	stateStart state = iota
	stateStream
	stateTools
	stateDone
)

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

	state state
	err   error
}

func (m *machine) run() {
	for m.state != stateDone {
		m.step()
	}
	if m.err != nil {
		m.yield(nil, m.err)
	}
}

func (m *machine) step() {
	switch m.state {
	case stateStart:
		m.doStart()
	case stateStream:
		m.doStream()
	case stateTools:
		m.doTools()
	}
}

func (m *machine) emit(event core.Event) bool {
	if !m.yield(event, nil) {
		m.state = stateDone
		return false
	}
	return true
}

func (m *machine) fail(err error) {
	m.err = err
	m.state = stateDone
}

func (m *machine) done() {
	m.state = stateDone
}

// doStart emits AssistantStarted and transitions to Stream.
func (m *machine) doStart() {
	if m.emit(m.engine.events.AssistantStarted(m.threadID, m.messageID)) {
		m.state = stateStream
	}
}

// doStream prepares context, streams LLM response, and emits AssistantCompleted.
func (m *machine) doStream() {
	// Prepare
	runCtx := core.RunContext{
		ThreadID:  m.threadID,
		MessageID: m.messageID,
		Thread:    m.thread,
		Options:   m.options,
	}
	prepareResult, err := m.engine.llmCtx.Prepare(m.ctx, runCtx)
	if err != nil {
		m.fail(err)
		return
	}
	if prepareResult.Terminated {
		m.done()
		return
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
		m.fail(err)
		return
	}
	if streamResult == nil {
		m.done()
		return
	}
	m.streamResult = streamResult

	// Persist and emit completed
	toolCalls := toDomainToolCalls(m.streamResult.ToolCalls)
	thread, err := m.engine.llmCtx.AddAssistantMessage(m.ctx, m.threadID, m.messageID, m.streamResult.Content, toolCalls)
	if err != nil {
		m.fail(err)
		return
	}
	m.thread = thread

	if !m.emit(m.engine.events.AssistantCompleted(m.threadID, m.messageID, m.streamResult.Content, toolCalls)) {
		return
	}

	// Transition
	if m.streamResult.HasToolCalls() {
		m.state = stateTools
	} else {
		m.done()
	}
}

// doTools executes all tool calls and emits their results.
func (m *machine) doTools() {
	shouldStop := false

	for _, tc := range m.streamResult.ToolCalls {
		if !m.emit(m.engine.events.ToolCallCompleted(m.threadID, tc.ID)) {
			return
		}

		result := m.engine.toolExec.Execute(tc, m.executor)

		toolResultID := core.GenerateID()
		thread, err := m.engine.llmCtx.AddToolResultMessage(m.ctx, m.threadID, toolResultID, result.ToolCallID, result.Result)
		if err != nil {
			m.fail(err)
			return
		}
		m.thread = thread

		if !m.emit(m.engine.events.ToolCallExecuted(
			m.threadID, toolResultID, result.ToolCallID,
			result.ToolName, result.Arguments, result.Result, m.messageID,
		)) {
			return
		}

		if result.StopIteration {
			shouldStop = true
		}
	}

	// Transition
	if shouldStop {
		m.done()
	} else {
		m.messageID = core.GenerateID()
		m.state = stateStream
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


