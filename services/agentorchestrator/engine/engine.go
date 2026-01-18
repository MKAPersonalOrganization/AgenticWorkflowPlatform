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
		exec, err := e.newExecutionState(ctx, agentName, runCtx, yield)
		if err != nil {
			yield(nil, err)
			return
		}

		startState := exec.inferState()
		if err := exec.setup(startState); err != nil {
			yield(nil, err)
			return
		}

		if err := exec.run(); err != nil {
			// Update result based on error type, but only if not already set to a specific reason
			if exec.observer.result.Reason == TerminationGoalReached {
				if ctx.Err() != nil {
					exec.observer.result.Reason = TerminationUserAborted
				} else {
					exec.observer.result.Reason = TerminationError
					exec.observer.result.Error = err
				}
				exec.observer.result.Success = false
			}
			yield(nil, err)
		}

		// Finalize execution result
		res := exec.observer.result
		res.FinalAgent = exec.agentName
		res.Iterations = exec.safety.iteration
		res.ToolsCalled = exec.observer.toolNames
		res.Duration = timeSince(exec.observer.startTime)
	}
}

func (e *Engine) newExecutionState(ctx context.Context, agentName string, runCtx core.RunContext, yield func(core.Event, error) bool) (*executionState, error) {
	exec := &executionState{
		engine:    e,
		ctx:       ctx,
		threadID:  runCtx.ThreadID,
		messageID: runCtx.MessageID,
		thread:    runCtx.Thread,
		options:   runCtx.Options,
		agentName: agentName,
		history:   runCtx.InitialHistory,
		observer: &ObserverUnit{
			yield:     yield,
			startTime: timeNowNano(),
			threadID:  runCtx.ThreadID,
			result: &ExecutionResult{
				Success:    true,
				Reason:     TerminationGoalReached,
				FinalAgent: agentName,
			},
		},
		safety: &SafetyUnit{
			maxTotal:     20,
			loopDetector: NewLoopDetector(DefaultLoopDetectorConfig()),
		},
		workers: &WorkersUnit{},
		persistence: &PersistenceUnit{
			mgmt: e.threadMgmt,
		},
		tools: &ToolProcessor{},
	}
	exec.persistence.state = exec
	exec.tools.state = exec

	if err := exec.initAgentComponents(agentName); err != nil {
		exec.observer.result.Success = false
		exec.observer.result.Reason = TerminationError
		exec.observer.result.Error = err
		return nil, err
	}

	return exec, nil
}

// timeNowNano returns the current time in nanoseconds (extracted for testing).
var timeNowNano = func() int64 {
	return time.Now().UnixNano()
}

// timeSince calculates duration since a start time in nanoseconds.
func timeSince(startNano int64) time.Duration {
	return time.Duration(timeNowNano() - startNano)
}

// executionState coordinates the agent's turn.
type executionState struct {
	engine *Engine
	ctx    context.Context

	// Context of the current turn
	threadID  string
	messageID string
	agentName string
	thread    core.AgentThread
	options   core.RunOptions
	history   []core.Message

	// Cohesive Units
	safety      *SafetyUnit
	observer    *ObserverUnit
	workers     *WorkersUnit
	persistence *PersistenceUnit
	tools       *ToolProcessor

	// Current Turn State
	sm           *stateless.StateMachine
	streamResult *StreamResult
}

// HistoryReconciler analyzes the conversation history to determine the resumption state.
type HistoryReconciler struct {
	history   []core.Message
	agentName string
}

// ResumptionPlan describes an action to take to pair a message during state inference.
type ResumptionPlan struct {
	ToolCallID string
	Content    string
}

// SafetyUnit enforces execution limits and detects infinite loops.
type SafetyUnit struct {
	iteration    int
	total        int
	maxAgent     int
	maxTotal     int
	loopDetector *LoopDetector
}

func (s *SafetyUnit) Guard() error {
	if s.iteration >= s.maxAgent || s.total >= s.maxTotal {
		if s.total >= s.maxTotal {
			return fmt.Errorf("run exceeded total iteration limit (%d)", s.maxTotal)
		}
		return fmt.Errorf("agent exceeded maximum iterations (%d)", s.maxAgent)
	}
	s.iteration++
	s.total++
	return nil
}

func (s *SafetyUnit) RecordContent(content string) *LoopInfo {
	return s.loopDetector.RecordContent(content)
}

func (s *SafetyUnit) RecordToolCall(tc core.ToolCall) *LoopInfo {
	return s.loopDetector.RecordToolCall(tc)
}

func (s *SafetyUnit) RecordToolResult(name, result string, isErr bool) *LoopInfo {
	return s.loopDetector.RecordToolResult(name, result, isErr)
}

func (s *SafetyUnit) ResetAgentLimits() {
	s.iteration = 0
	s.loopDetector.Reset()
}

// ObserverUnit handles event emission and result reporting.
type ObserverUnit struct {
	yield     func(core.Event, error) bool
	result    *ExecutionResult
	startTime int64
	toolNames []string
	events    *EventFactory
	threadID  string
}

func (o *ObserverUnit) TrackStarted(messageID string) bool {
	return o.yield(o.events.AssistantStarted(o.threadID, messageID), nil)
}

func (o *ObserverUnit) TrackCompleted(messageID, content string, toolCalls []domain.ToolCall) bool {
	return o.yield(o.events.AssistantCompleted(o.threadID, messageID, content, toolCalls), nil)
}

func (o *ObserverUnit) TrackToolCall(tcID string) bool {
	return o.yield(o.events.ToolCallCompleted(o.threadID, tcID), nil)
}

func (o *ObserverUnit) TrackToolExecuted(toolResultID, tcID, name, args, res, messageID string) bool {
	return o.yield(o.events.ToolCallExecuted(o.threadID, toolResultID, tcID, name, args, res, messageID), nil)
}

func (o *ObserverUnit) TrackHandoff(target string) bool {
	return o.yield(o.events.AgentHandoff(o.threadID, target), nil)
}

func (o *ObserverUnit) TrackHumanInput(messageID, tcID, question string, schema map[string]any) bool {
	return o.yield(o.events.HumanInputRequested(o.threadID, messageID, tcID, question, schema), nil)
}

func (o *ObserverUnit) Fail(reason TerminationReason, err error) {
	o.result.Success = false
	o.result.Reason = reason
	o.result.Error = err
}

func (o *ObserverUnit) FailWithLoop(info *LoopInfo) error {
	o.Fail(TerminationLoopDetected, fmt.Errorf("loop detected: %s (%d occurrences)", info.Description, info.Occurrences))
	o.result.LoopInfo = info
	return o.result.Error
}

// WorkersUnit provides the specialized tools and processors for the turn.
type WorkersUnit struct {
	llm      *LLMContextManager
	stream   *StreamProcessor
	events   *EventFactory
	executor core.ToolExecutor
}

// PersistenceUnit ensures atomic updates between the database and in-memory state.
type PersistenceUnit struct {
	mgmt  *threadmgmt.Manager
	state *executionState
}

func (p *PersistenceUnit) AddAssistantMessage(ctx context.Context, content string, toolCalls []domain.ToolCall) error {
	agg, err := p.mgmt.AddAssistantMessage(ctx, p.state.threadID, p.state.messageID, content, toolCalls, p.state.agentName)
	if err != nil {
		return err
	}
	p.refreshState(agg)
	return nil
}

func (p *PersistenceUnit) AddToolResultMessage(ctx context.Context, toolCallID, result string) error {
	toolResultID := core.GenerateID()
	agg, err := p.mgmt.AddToolResultMessage(ctx, p.state.threadID, toolResultID, toolCallID, result, p.state.agentName)
	if err != nil {
		return err
	}
	p.refreshState(agg)
	return nil
}

func (p *PersistenceUnit) refreshState(agg *domain.ThreadAggregate) {
	p.state.thread = p.mgmt.ToAgentThread(agg)
	p.state.history = p.mgmt.ToCoreMessages(agg)
}

// ToolProcessor handles the execution of tools and determination of next steps.
type ToolProcessor struct {
	state *executionState
}

func (p *ToolProcessor) ProcessActions() (*RouteResolver, error) {
	var route RouteResolver
	for _, tc := range p.state.streamResult.ToolCalls {
		p.state.observer.toolNames = append(p.state.observer.toolNames, tc.Function.Name)

		if info := p.state.safety.RecordToolCall(tc); info != nil {
			return nil, p.state.observer.FailWithLoop(info)
		}

		if !p.state.observer.TrackToolCall(tc.ID) {
			return nil, context.Canceled
		}

		result := p.state.engine.toolExec.Execute(tc, p.state.workers.executor)
		if info := p.state.safety.RecordToolResult(result.ToolName, result.Result, result.IsError); info != nil {
			return nil, p.state.observer.FailWithLoop(info)
		}

		if err := p.state.persistence.AddToolResultMessage(p.state.ctx, tc.ID, result.Result); err != nil {
			return nil, err
		}

		toolResultID := core.GenerateID()
		if !p.state.observer.TrackToolExecuted(toolResultID, tc.ID, result.ToolName, result.Arguments, result.Result, p.state.messageID) {
			return nil, context.Canceled
		}

		if res := result.Command; res != nil {
			if res.Goto != "" {
				route.HandoffAgent = res.Goto
			}
			if res.Elicitation != nil {
				route.Elicitation = res.Elicitation
				route.ToolCallID = tc.ID
			}
		}
	}
	return &route, nil
}

// RouteResolver determines the next state based on tool results.
type RouteResolver struct {
	HandoffAgent string
	Elicitation  *core.ElicitationSpec
	ToolCallID   string
}

func (r *RouteResolver) HasAction() bool {
	return r.HandoffAgent != "" || r.Elicitation != nil
}

func (s *executionState) prepareContextForLLM() (*PrepareResult, error) {
	runCtx := core.RunContext{
		ThreadID:  s.threadID,
		MessageID: s.messageID,
		Thread:    s.thread,
		Options:   s.options,
	}
	return s.workers.llm.Prepare(s.ctx, runCtx, s.history)
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
	s.workers.llm = NewLLMContextManager(config, s.engine.threadMgmt, s.engine.reqCtxCapture)
	s.workers.events = NewEventFactory(config.Name)
	s.workers.stream = NewStreamProcessor(s.workers.events)

	s.observer.events = s.workers.events

	s.safety.maxAgent = config.MaxIterations
	if s.safety.maxAgent <= 0 {
		s.safety.maxAgent = 5
	}
	s.safety.iteration = 0
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
	reconciler := &HistoryReconciler{
		history:   s.history,
		agentName: s.agentName,
	}
	st, res, plan := reconciler.Reconcile()

	if plan != nil {
		if err := s.persistence.AddToolResultMessage(s.ctx, plan.ToolCallID, plan.Content); err != nil {
			return StateError // Or handle error appropriately
		}
	}

	s.streamResult = res
	if s.streamResult != nil {
		s.workers.events = NewEventFactory(s.agentName)
	}
	return st
}

func (r *HistoryReconciler) Reconcile() (state, *StreamResult, *ResumptionPlan) {
	if len(r.history) == 0 {
		return StateAwaitingTurn, nil, nil
	}

	// 1. Find the last assistant message with tool calls
	lastAssistantWithTools := r.findLastAssistantWithTools()
	if lastAssistantWithTools == nil {
		return StateAwaitingTurn, nil, nil
	}

	// 2. Check if all tool calls of that message have results
	pendingTC := r.findPendingToolCall(lastAssistantWithTools)
	if pendingTC == nil {
		return StateAwaitingTurn, nil, nil
	}

	// 3. Resumption Logic: Check for Elicitation response
	if pendingTC.Function.Name == middleware.ElicitInputToolName {
		return r.reconcileElicitation(pendingTC)
	}

	// Populate streamResult for traditional tool execution
	res := &StreamResult{
		Content:   lastAssistantWithTools.Content,
		ToolCalls: lastAssistantWithTools.ToolCalls,
	}
	return StatePerformingActions, res, nil
}

func (r *HistoryReconciler) findLastAssistantWithTools() *core.Message {
	for i := len(r.history) - 1; i >= 0; i-- {
		if r.history[i].Role == "assistant" && len(r.history[i].ToolCalls) > 0 {
			return &r.history[i]
		}
	}
	return nil
}

func (r *HistoryReconciler) findPendingToolCall(msg *core.Message) *core.ToolCall {
	completedCalls := make(map[string]bool)
	for i := len(r.history) - 1; i >= 0; i-- {
		if r.history[i].Role == "tool" {
			completedCalls[r.history[i].ToolCallID] = true
		}
	}

	for _, tc := range msg.ToolCalls {
		if !completedCalls[tc.ID] {
			return &tc
		}
	}
	return nil
}

func (r *HistoryReconciler) reconcileElicitation(tc *core.ToolCall) (state, *StreamResult, *ResumptionPlan) {
	lastMsg := r.history[len(r.history)-1]

	if lastMsg.Role == "user" {
		// Plan to automate Pairing: Fulfill the tool call with the user message content
		return StateGeneratingResponse, nil, &ResumptionPlan{
			ToolCallID: tc.ID,
			Content:    lastMsg.Content,
		}
	}

	if lastMsg.Role == "tool" && lastMsg.ToolCallID == tc.ID {
		return StateGeneratingResponse, nil, nil
	}

	return StateWaitingForHuman, nil, nil
}

func (s *executionState) generateAgentResponse() error {
	if err := s.safety.Guard(); err != nil {
		s.observer.Fail(TerminationMaxIterations, err)
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

func (s *executionState) streamAndProcessLLMResponse(llmContext *LLMContext) error {
	if !s.observer.TrackStarted(s.messageID) {
		return context.Canceled
	}

	s.workers.executor = s.engine.toolExec.CreateExecutor(
		s.ctx, llmContext.GetEffectiveTools(),
		s.threadID, s.messageID, llmContext.RequestCtx,
	)

	streamIter := s.engine.chatClient.GetStreamingResponse(s.ctx, llmContext.Messages, llmContext.Options)
	res, err := s.workers.stream.Process(s.ctx, streamIter, ProcessParams{
		ThreadID:  s.threadID,
		MessageID: s.messageID,
	}, s.observer.yield)

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
		if info := s.safety.RecordContent(s.streamResult.Content); info != nil {
			return s.observer.FailWithLoop(info)
		}
	}
	return nil
}

func (s *executionState) saveAgentMessageAndEmitEvents() error {
	toolCalls := toDomainToolCalls(s.streamResult.ToolCalls)
	if err := s.persistence.AddAssistantMessage(s.ctx, s.streamResult.Content, toolCalls); err != nil {
		return err
	}

	if !s.observer.TrackCompleted(s.messageID, s.streamResult.Content, toolCalls) {
		return context.Canceled
	}
	return nil
}

func (s *executionState) executeRequiredActions() error {
	resolution, err := s.tools.ProcessActions()
	if err != nil {
		return err
	}

	if resolution.Elicitation != nil {
		return s.pauseWaitingForHumanInput(resolution.ToolCallID, resolution.Elicitation)
	}

	if resolution.HandoffAgent != "" {
		return s.transferControlToAgent(resolution.HandoffAgent)
	}

	return s.prepareForNextMessageTurn()
}

func (s *executionState) failWithLoop(reason TerminationReason, info *LoopInfo) error {
	s.observer.result.Reason = reason
	s.observer.result.LoopInfo = info
	s.observer.result.Success = false
	return fmt.Errorf("loop detected: %s (%d occurrences)", info.Description, info.Occurrences)
}


func (s *executionState) pauseWaitingForHumanInput(toolID string, spec *core.ElicitationSpec) error {
	s.observer.result.Reason = TerminationElicitation
	if !s.observer.TrackHumanInput(s.messageID, toolID, spec.Question, spec.Schema) {
		return context.Canceled
	}
	return s.sm.Fire(TrigHumanInputNeeded)
}

func (s *executionState) transferControlToAgent(targetAgent string) error {
	// If already in StateSwitchingAgent, we are finishing the switch
	state, _ := s.sm.State(s.ctx)
	if state == StateSwitchingAgent {
		s.observer.result.FinalAgent = targetAgent
		s.observer.result.HandoffTarget = "" // Clear transient target
		return s.sm.Fire(TrigAgentSwitched)
	}

	// Otherwise, we are initiating the switch
	s.observer.result.Reason = TerminationHandoff
	s.observer.result.HandoffTarget = targetAgent

	if !s.observer.TrackHandoff(targetAgent) {
		return context.Canceled
	}

	if err := s.initAgentComponents(targetAgent); err != nil {
		return err
	}

	s.safety.ResetAgentLimits()
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
