package engine

import (
	"context"
	"fmt"
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
	streamResult  *StreamResult
	executor      core.ToolExecutor
	iteration     int // Current iteration count within this agent's cycle
	maxIterations int // Maximum allowed iterations (from agent config)

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

	// Set iteration limit from config (default to 5 if not set)
	s.maxIterations = config.MaxIterations
	if s.maxIterations <= 0 {
		s.maxIterations = 5
	}
	s.iteration = 0 // Reset iteration counter for new agent

	return nil
}

func (s *executionState) setup(initialState state) error {
	// Configure State Machine
	s.sm = stateless.NewStateMachine(initialState)

	s.sm.Configure(StateInitial).
		OnExit(func(_ context.Context, _ ...any) error {
			return s.handleStart()
		}).
		Permit(TriggerBegin, StateDeterminingNextStep)

	s.sm.Configure(StateDeterminingNextStep).
		OnEntry(func(_ context.Context, _ ...any) error {
			return s.handleStream()
		}).
		PermitReentry(TriggerBegin). // Permit if reconciled here
		Permit(TriggerActionRequired, StateExecutingActions).
		Permit(TriggerStepComplete, StateDone)

	s.sm.Configure(StateExecutingActions).
		OnEntry(func(_ context.Context, _ ...any) error {
			return s.handleTools()
		}).
		PermitReentry(TriggerBegin). // Permit if reconciled here
		Permit(TriggerActionDone, StateDeterminingNextStep).
		Permit(TriggerHandoffSwitch, StateTransferring).
		Permit(TriggerHumanInputRequired, StateAwaitingHumanInput)

	s.sm.Configure(StateTransferring).
		OnEntry(func(_ context.Context, args ...any) error {
			targetAgent := args[0].(string)
			return s.handleSwitch(targetAgent)
		}).
		Permit(TriggerTransferComplete, StateInitial)

	s.sm.Configure(StateAwaitingHumanInput).
		PermitReentry(TriggerBegin) // Self-transition if run starts here

	return nil
}

func (s *executionState) run() error {
	for {
		// State Reconciliation: Infer starting state from thread history
		startState := s.inferState()

		if err := s.setup(startState); err != nil {
			return err
		}

		if err := s.sm.Fire(TriggerBegin); err != nil {
			return err
		}

		// If we are back in StateInitial, it means an agent handoff occurred.
		// We loop to run the next agent's cycle.
		state, _ := s.sm.State(s.ctx)
		if state != StateInitial {
			break
		}
	}

	return nil
}

func (s *executionState) inferState() state {
	messages, err := s.thread.RetrieveFullConversationHistory(s.ctx)
	if err != nil || len(messages) == 0 {
		return StateInitial
	}

	// 1. Find the last assistant message with tool calls
	var lastAssistantWithTools *core.Message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" && len(messages[i].ToolCalls) > 0 {
			lastAssistantWithTools = &messages[i]
			break
		}
	}

	if lastAssistantWithTools == nil {
		return StateInitial
	}

	// 2. Check if all tool calls of that message have results
	completedCalls := make(map[string]bool)
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "tool" {
			completedCalls[messages[i].ToolCallID] = true
		}
	}

	var pendingTC *core.ToolCall
	allCompleted := true
	for _, tc := range lastAssistantWithTools.ToolCalls {
		if !completedCalls[tc.ID] {
			allCompleted = false
			pendingTC = &tc
			break
		}
	}

	if allCompleted {
		return StateInitial
	}

	// 3. Resumption Logic: Check for human response
	if pendingTC.Function.Name == "transfer_to_agent" {
		var args struct {
			AgentName string `json:"agent_name"`
		}
		if err := core.UnmarshalArgs(pendingTC.Function.Arguments, &args); err == nil && args.AgentName == "human" {
			// If we have a user message after this assistant message, it's the response
			lastMsg := messages[len(messages)-1]
			if lastMsg.Role == "user" {
				// Pair: Fulfill the tool call with the user message
				toolResultID := core.GenerateID()
				agg, err := s.engine.threadMgmt.AddToolResultMessage(s.ctx, s.threadID, toolResultID, pendingTC.ID, lastMsg.Content, s.agentName)
				if err == nil {
					s.thread = s.engine.threadMgmt.ToAgentThread(agg)
					// Successfully paired, now determine next step
					return StateDeterminingNextStep
				}
			}
			// Still waiting for user input
			return StateAwaitingHumanInput
		}
	}

	// Populate streamResult for traditional tool execution
	s.streamResult = &StreamResult{
		Content:   lastAssistantWithTools.Content,
		ToolCalls: lastAssistantWithTools.ToolCalls,
	}
	return StateExecutingActions
}

func (s *executionState) handleStart() error {
	if !s.emit(s.events.AssistantStarted(s.threadID, s.messageID)) {
		return context.Canceled
	}
	return nil
}

func (s *executionState) handleStream() error {
	// Check iteration limit (must check BEFORE making LLM call)
	if s.iteration >= s.maxIterations {
		return fmt.Errorf("agent %q exceeded maximum iterations (%d)", s.agentName, s.maxIterations)
	}
	s.iteration++

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
		return s.sm.Fire(TriggerStepComplete)
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
		return s.sm.Fire(TriggerStepComplete)
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
		return s.sm.Fire(TriggerActionRequired)
	}
	return s.sm.Fire(TriggerStepComplete)
}

func (s *executionState) handleTools() error {
	var handoffTarget string

	// Process all tool calls sequentially (OpenAI requires results for all tool_call_ids)
	for _, tc := range s.streamResult.ToolCalls {
		if !s.emit(s.events.ToolCallCompleted(s.threadID, tc.ID)) {
			return context.Canceled
		}

		result := s.engine.toolExec.Execute(tc, s.executor)

		if result.ToolCallID == "" {
			result.ToolCallID = tc.ID // Fallback
		}

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

		// Check for routing command (handoff)
		if result.Command != nil && result.Command.Goto != "" {
			handoffTarget = result.Command.Goto

			// Special case: Human intervention (Factor 7)
			if handoffTarget == "human" {
				var params struct {
					Question string `json:"question"`
				}
				core.UnmarshalArgs(result.Result, &params)
				if params.Question == "" {
					params.Question = "Please provide the requested information."
				}
				if !s.emit(s.events.HumanInputRequested(s.threadID, s.messageID, tc.ID, params.Question)) {
					return context.Canceled
				}
			}
		}
	}

	if handoffTarget == "human" {
		return s.sm.Fire(TriggerHumanInputRequired)
	}

	if handoffTarget != "" {
		return s.sm.Fire(TriggerHandoffSwitch, handoffTarget)
	}

	s.messageID = core.GenerateID()
	return s.sm.Fire(TriggerActionDone)
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

	return s.sm.Fire(TriggerTransferComplete)
}

// State machine states
type state int

const (
	StateInitial state = iota
	StateDeterminingNextStep
	StateExecutingActions
	StateTransferring
	StateAwaitingHumanInput
	StateDone
	StateError
)

const (
	TriggerBegin            = "Begin"
	TriggerActionRequired   = "ActionRequired"
	TriggerStepComplete     = "StepComplete"
	TriggerActionDone       = "ActionDone"
	TriggerHandoffSwitch    = "HandoffSwitch"
	TriggerHumanInputRequired = "HumanInputRequired"
	TriggerTransferComplete = "TransferComplete"
)




