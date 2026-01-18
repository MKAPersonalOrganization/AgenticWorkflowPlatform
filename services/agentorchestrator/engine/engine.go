package engine

import (
	"context"
	"fmt"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/agentorchestrator/types"
	"school-with-ai/services/threadmgmt"
	"time"

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
			engine:         e,
			ctx:            ctx,
			yield:          yield,
			threadID:       runCtx.ThreadID,
			messageID:      runCtx.MessageID,
			thread:         runCtx.Thread,
			options:        runCtx.Options,
			agentName:      agentName,
			loopDetector:   NewLoopDetector(DefaultLoopDetectorConfig()),
			totalIterations: 0,
			totalToolCalls:  make([]string, 0),
			startTime:       timeNowNano(),
			history:         runCtx.InitialHistory,
			result: &ExecutionResult{
				Success:    true,
				Reason:     TerminationGoalReached,
				FinalAgent: agentName,
			},
		}

		if err := exec.initAgentComponents(agentName); err != nil {
			exec.result.Success = false
			exec.result.Reason = TerminationError
			exec.result.Error = err
			yield(nil, err)
			return
		}

		startState := exec.inferState()
		if err := exec.setup(startState); err != nil {
			yield(nil, err)
			return
		}

		if err := exec.run(); err != nil {
			// Update result based on error type
			if ctx.Err() != nil {
				exec.result.Reason = TerminationUserAborted
			} else {
				exec.result.Reason = TerminationError
				exec.result.Error = err
			}
			exec.result.Success = false
			yield(nil, err)
		}

		// Finalize execution result
		exec.result.FinalAgent = exec.agentName
		exec.result.Iterations = exec.iteration
		exec.result.ToolsCalled = exec.totalToolCalls
		exec.result.Duration = timeSince(exec.startTime)
	}
}

// timeNowNano returns the current time in nanoseconds (extracted for testing).
var timeNowNano = func() int64 {
	return time.Now().UnixNano()
}

// timeSince calculates duration since a start time in nanoseconds.
func timeSince(startNano int64) time.Duration {
	return time.Duration(timeNowNano() - startNano)
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
	iteration       int // Iterations for the current agent
	totalIterations int // Total iterations across ALL agents in this run
	maxIterations   int // Per-agent iteration limit
	maxTotalLimit   int // Global safety limit

	// State Machine
	sm *stateless.StateMachine

	// Execution tracking (enhanced)
	loopDetector   *LoopDetector
	result         *ExecutionResult
	totalToolCalls []string
	startTime      int64 // Unix nano timestamp
	history        []core.Message
}

func (s *executionState) emit(event core.Event) bool {
	return s.yield(event, nil)
}

func (s *executionState) updateState(agg *domain.ThreadAggregate) {
	s.thread = s.engine.threadMgmt.ToAgentThread(agg)
	s.history = s.engine.threadMgmt.ToCoreMessages(agg)
}

func (s *executionState) initAgentComponents(name string) error {
	config, err := s.engine.provider.GetConfig(name)
	if err != nil {
		return err
	}
	if err := config.Validate(); err != nil {
		return err
	}

	s.agentName = name
	s.llmCtx = NewLLMContextManager(config, s.engine.threadMgmt, s.engine.reqCtxCapture)
	s.events = NewEventFactory(config.Name)
	s.stream = NewStreamProcessor(s.events)
	s.maxIterations = config.MaxIterations
	if s.maxIterations <= 0 {
		s.maxIterations = 5
	}
	s.maxTotalLimit = 20
	s.iteration = 0
	return nil
}

func (s *executionState) setup(initialState state) error {
	s.sm = stateless.NewStateMachine(initialState)

	s.sm.Configure(StateAwaitingTurn).
		Permit(TrigStartExecution, StateGeneratingResponse)

	s.sm.Configure(StateGeneratingResponse).
		OnEntry(func(_ context.Context, _ ...any) error { return s.generateAgentResponse() }).
		PermitReentry(TrigStartExecution).
		Permit(TrigActionsDetected, StatePerformingActions).
		Permit(TrigTurnFinished, StateDone)

	s.sm.Configure(StatePerformingActions).
		OnEntry(func(_ context.Context, _ ...any) error { return s.executeRequiredActions() }).
		PermitReentry(TrigStartExecution).
		Permit(TrigActionsCompleted, StateGeneratingResponse).
		Permit(TrigHandoffInitiated, StateSwitchingAgent).
		Permit(TrigHumanInputNeeded, StateWaitingForHuman)

	s.sm.Configure(StateSwitchingAgent).
		OnEntry(func(_ context.Context, args ...any) error { return s.transferControlToAgent(args[0].(string)) }).
		Permit(TrigAgentSwitched, StateGeneratingResponse)

	s.sm.Configure(StateWaitingForHuman).
		PermitReentry(TrigStartExecution)

	return nil
}

func (s *executionState) run() error {
	return s.sm.Fire(TrigStartExecution)
}

func (s *executionState) inferState() state {
	if len(s.history) == 0 {
		return StateAwaitingTurn
	}

	messages := s.history

	// 1. Find the last assistant message with tool calls
	var lastAssistantWithTools *core.Message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" && len(messages[i].ToolCalls) > 0 {
			lastAssistantWithTools = &messages[i]
			break
		}
	}

	if lastAssistantWithTools == nil {
		return StateAwaitingTurn
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
		return StateAwaitingTurn
	}

	// 3. Resumption Logic: Check for Elicitation response
	if tc := pendingTC; tc != nil {
		if tc.Function.Name == middleware.ElicitInputToolName {
			// Check if the last message is a user response that hasn't been paired yet
			lastMsg := messages[len(messages)-1]
			if lastMsg.Role == "user" {
				// Automate Pairing: Fulfill the tool call with the user message content
				toolResultID := core.GenerateID()
				agg, err := s.engine.threadMgmt.AddToolResultMessage(s.ctx, s.threadID, toolResultID, tc.ID, lastMsg.Content, s.agentName)
				if err == nil {
					// Refresh state and transition to generating response
					s.updateState(agg)
					return StateGeneratingResponse
				}
			}

			if lastMsg.Role == "tool" && lastMsg.ToolCallID == tc.ID {
				// Tool result exists, so we can determine next step
				return StateGeneratingResponse
			}
			// Still waiting for user input
			return StateWaitingForHuman
		}
	}

	// Populate streamResult for traditional tool execution
	s.streamResult = &StreamResult{
		Content:   lastAssistantWithTools.Content,
		ToolCalls: lastAssistantWithTools.ToolCalls,
	}
	return StatePerformingActions
}

func (s *executionState) generateAgentResponse() error {
	if err := s.ensureUnderSafetyLimits(); err != nil {
		return err
	}

	prepped, err := s.prepareContextForLLM()
	if err != nil {
		return err
	}
	if prepped.Terminated {
		return s.sm.Fire(TrigTurnFinished)
	}

	if err := s.streamAndProcessLLMResponse(prepped.Context); err != nil {
		return err
	}

	if err := s.validateAgentOutput(); err != nil {
		return err
	}

	if err := s.saveAgentMessageAndEmitEvents(); err != nil {
		return err
	}

	if s.streamResult.HasToolCalls() {
		return s.sm.Fire(TrigActionsDetected)
	}
	return s.sm.Fire(TrigTurnFinished)
}

func (s *executionState) ensureUnderSafetyLimits() error {
	if s.iteration >= s.maxIterations || s.totalIterations >= s.maxTotalLimit {
		s.result.Reason = TerminationMaxIterations
		s.result.Success = false
		if s.totalIterations >= s.maxTotalLimit {
			return fmt.Errorf("run exceeded total iteration limit (%d)", s.maxTotalLimit)
		}
		return fmt.Errorf("agent %q exceeded maximum iterations (%d)", s.agentName, s.maxIterations)
	}
	s.iteration++
	s.totalIterations++
	return nil
}

func (s *executionState) prepareContextForLLM() (*PrepareResult, error) {
	runCtx := core.RunContext{
		ThreadID:  s.threadID,
		MessageID: s.messageID,
		Thread:    s.thread,
		Options:   s.options,
	}
	return s.llmCtx.Prepare(s.ctx, runCtx)
}

func (s *executionState) streamAndProcessLLMResponse(llmContext *LLMContext) error {
	if !s.emit(s.events.AssistantStarted(s.threadID, s.messageID)) {
		return context.Canceled
	}

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
		return s.sm.Fire(TrigTurnFinished)
	}
	s.streamResult = res
	return nil
}

func (s *executionState) validateAgentOutput() error {
	if s.streamResult.Content != "" {
		if info := s.loopDetector.RecordContent(s.streamResult.Content); info != nil {
			return s.failWithLoop(TerminationLoopDetected, info)
		}
	}
	return nil
}

func (s *executionState) saveAgentMessageAndEmitEvents() error {
	toolCalls := toDomainToolCalls(s.streamResult.ToolCalls)
	agg, err := s.engine.threadMgmt.AddAssistantMessage(s.ctx, s.threadID, s.messageID, s.streamResult.Content, toolCalls, s.agentName)
	if err != nil {
		return err
	}
	s.updateState(agg)

	if !s.emit(s.events.AssistantCompleted(s.threadID, s.messageID, s.streamResult.Content, toolCalls)) {
		return context.Canceled
	}
	return nil
}

func (s *executionState) executeRequiredActions() error {
	var route toolRoute
	if err := s.executeAllScheduledTools(&route); err != nil {
		return err
	}

	return s.handleSpecificRoutingTriggers(route)
}

func (s *executionState) failWithLoop(reason TerminationReason, info *LoopInfo) error {
	s.result.Reason = reason
	s.result.LoopInfo = info
	s.result.Success = false
	return fmt.Errorf("loop detected: %s (%d occurrences)", info.Description, info.Occurrences)
}

func (s *executionState) executeAllScheduledTools(route *toolRoute) error {
	for _, tc := range s.streamResult.ToolCalls {
		s.totalToolCalls = append(s.totalToolCalls, tc.Function.Name)

		if info := s.loopDetector.RecordToolCall(tc); info != nil {
			return s.failWithLoop(TerminationLoopDetected, info)
		}

		if !s.emit(s.events.ToolCallCompleted(s.threadID, tc.ID)) {
			return context.Canceled
		}

		result := s.engine.toolExec.Execute(tc, s.executor)
		if info := s.loopDetector.RecordToolResult(result.ToolName, result.Result, result.IsError); info != nil {
			return s.failWithLoop(TerminationLoopDetected, info)
		}

		toolResultID := core.GenerateID()
		agg, err := s.engine.threadMgmt.AddToolResultMessage(s.ctx, s.threadID, toolResultID, tc.ID, result.Result, s.agentName)
		if err != nil {
			return err
		}
		s.updateState(agg)

		if !s.emit(s.events.ToolCallExecuted(s.threadID, toolResultID, tc.ID, result.ToolName, result.Arguments, result.Result, s.messageID)) {
			return context.Canceled
		}

		if res := result.Command; res != nil {
			if res.Goto != "" {
				route.handoff = res.Goto
			}
			if res.Elicitation != nil {
				route.elicitation = res.Elicitation
				route.toolID = tc.ID
			}
		}
	}
	return nil
}

type toolRoute struct {
	handoff     string
	elicitation *core.ElicitationSpec
	toolID      string
}

func (s *executionState) handleSpecificRoutingTriggers(route toolRoute) error {
	if route.elicitation != nil {
		return s.pauseWaitingForHumanInput(route.toolID, route.elicitation)
	}

	if route.handoff != "" {
		return s.transferControlToAgent(route.handoff)
	}

	return s.prepareForNextMessageTurn()
}

func (s *executionState) pauseWaitingForHumanInput(toolID string, spec *core.ElicitationSpec) error {
	s.result.Reason = TerminationElicitation
	if !s.emit(s.events.HumanInputRequested(s.threadID, s.messageID, toolID, spec.Question, spec.Schema)) {
		return context.Canceled
	}
	return s.sm.Fire(TrigHumanInputNeeded)
}

func (s *executionState) transferControlToAgent(targetAgent string) error {
	// If already in StateSwitchingAgent, we are finishing the switch
	state, _ := s.sm.State(s.ctx)
	if state == StateSwitchingAgent {
		s.result.FinalAgent = targetAgent
		s.result.HandoffTarget = "" // Clear transient target
		return s.sm.Fire(TrigAgentSwitched)
	}

	// Otherwise, we are initiating the switch
	s.result.Reason = TerminationHandoff
	s.result.HandoffTarget = targetAgent

	if !s.emit(s.events.AgentHandoff(s.threadID, targetAgent)) {
		return context.Canceled
	}

	if err := s.initAgentComponents(targetAgent); err != nil {
		return err
	}

	s.loopDetector.Reset()
	s.messageID = core.GenerateID()

	return s.sm.Fire(TrigHandoffInitiated, targetAgent)
}

func (s *executionState) prepareForNextMessageTurn() error {
	s.messageID = core.GenerateID()
	return s.sm.Fire(TrigActionsCompleted)
}

// State machine states
type state int

const (
	StateAwaitingTurn state = iota
	StateGeneratingResponse
	StatePerformingActions
	StateSwitchingAgent
	StateWaitingForHuman
	StateDone
	StateError
)

const (
	TrigStartExecution   = "StartExecution"
	TrigActionsDetected  = "ActionsDetected"
	TrigTurnFinished     = "TurnFinished"
	TrigActionsCompleted = "ActionsCompleted"
	TrigHandoffInitiated = "HandoffInitiated"
	TrigHumanInputNeeded = "HumanInputNeeded"
	TrigAgentSwitched    = "AgentSwitched"
)
