package engine

import (
	"context"
	"fmt"
	"iter"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/agentorchestrator/types"
	"school-with-ai/services/threadmgmt"

	"github.com/qmuntal/stateless"
	"golang.org/x/sync/errgroup"
)

// Use central types
type ChatClient = types.ChatClient

// AgentProvider provides agent configurations by name.
type AgentProvider interface {
	GetConfig(ctx context.Context, name string) (core.AgentConfig, error)
}

// EventEmitter defines the interface for real-time event dispatching.
type EventEmitter interface {
	Dispatch(ctx context.Context, event core.Event) error
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
	emitter       EventEmitter
}

// New creates a new generic execution engine.
func New(
	log *logger.Logger,
	chatClient ChatClient,
	threadMgmt *threadmgmt.Manager,
	provider AgentProvider,
	reqCtxCapture **core.RequestContext,
	toolTracer core.ToolTracer,
	emitter EventEmitter,
) *Engine {
	return &Engine{
		log:           log,
		chatClient:    chatClient,
		provider:      provider,
		threadMgmt:    threadMgmt,
		reqCtxCapture: reqCtxCapture,
		toolTracer:    toolTracer,
		toolExec:      NewToolExecutionService(toolTracer),
		emitter:       emitter,
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
			exec.handleExecutionError(ctx, err)
			yield(nil, err)
			return
		}

		// Finalize and yield internal completion event with termination reason
		exec.finalizeResult()
		yield(runCompletedEvent{Reason: exec.observer.result.Reason}, nil)
	}
}

// handleExecutionError updates the result based on error type.
func (exec *executionState) handleExecutionError(ctx context.Context, err error) {
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
}

// finalizeResult completes the execution result with final metrics.
func (exec *executionState) finalizeResult() {
	res := exec.observer.result
	res.FinalAgent = exec.agent.Name
	res.Iterations = exec.safety.iteration
	res.ToolsCalled = exec.observer.toolNames
	res.Duration = timeSince(exec.observer.startTime)
}

func (e *Engine) newExecutionState(ctx context.Context, agentName string, runCtx core.RunContext, yield func(core.Event, error) bool) (*executionState, error) {
	instance, err := e.newAgentInstance(ctx, agentName)
	if err != nil {
		return nil, err
	}

	exec := &executionState{
		engine:    e,
		ctx:       ctx,
		threadID:  runCtx.ThreadID,
		messageID: runCtx.MessageID,
		thread:    runCtx.Thread,
		options:   runCtx.Options,
		agent:     instance,
		history:   runCtx.InitialHistory,
		observer: &ObserverUnit{
			yield:     e.wrapYield(ctx, &PersistenceUnit{mgmt: e.threadMgmt}, yield),
			startTime: timeNowNano(),
			threadID:  runCtx.ThreadID,
			events:    instance.Events,
			result: &ExecutionResult{
				Success:    true,
				Reason:     TerminationGoalReached,
				FinalAgent: agentName,
			},
		},
		safety: &SafetyUnit{
			maxTotal:     DefaultMaxTotalIterations,
			loopDetector: NewLoopDetector(DefaultLoopDetectorConfig()),
			maxAgent:     instance.Config.MaxIterations,
		},
		persistence: &PersistenceUnit{
			mgmt: e.threadMgmt,
		},
		tools: &ToolProcessor{},
	}

	if exec.safety.maxAgent <= 0 {
		exec.safety.maxAgent = DefaultMaxAgentIterations
	}

	return exec, nil
}

// wrapYield creates a yield function that automatically handles persistence and emission.
func (e *Engine) wrapYield(ctx context.Context, p *PersistenceUnit, yield func(core.Event, error) bool) func(core.Event, error) bool {
	return func(evt core.Event, err error) bool {
		if err != nil || evt == nil {
			return yield(evt, err)
		}

		if err := e.emit(ctx, evt, p); err != nil {
			e.log.Error(ctx, "failed to emit event", "error", err, "type", evt.EventType())
		}

		return yield(evt, err)
	}
}

// emit handles the dual path of event delivery:
// 1. Persistent events are saved to the EventStore (and then broadcast by the projection system).
// 2. Transient events are dispatched directly to the EventRouter for real-time streaming.
func (e *Engine) emit(ctx context.Context, evt core.Event, p *PersistenceUnit) error {
	if evt == nil {
		return nil
	}

	// 1. Handle special persistence cases (Token Usage, Tool Events)
	// These are events produced by the Engine that must be durable.
	persistent := false
	switch e := evt.(type) {
	case *domain.TokenUsageReported:
		if p != nil {
			if _, err := p.ReportTokenUsage(ctx, *e); err != nil {
				return err
			}
		}
		persistent = true
	case *domain.ToolAwaitingInputEvent,
		*domain.ToolAwaitingAuthEvent,
		*domain.ToolInputProvidedEvent,
		*domain.ToolAuthGrantedEvent,
		*domain.ToolStateChangedEvent:
		if p != nil {
			if _, err := p.ReportToolEvent(ctx, evt); err != nil {
				return err
			}
		}
		persistent = true
	case *domain.MessageAdded, *domain.ThreadCreated, *domain.ThreadTitleUpdatedEvent:
		// These are already persisted by the thread management calls.
		// We treat them as persistent so we don't dispatch them directly (avoiding duplicates).
		persistent = true
	}

	// 2. Dispatch transient events directly to the emitter for real-time UI updates
	if !persistent && e.emitter != nil {
		return e.emitter.Dispatch(ctx, evt)
	}

	return nil
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
	// turn-specific state
	threadID  string
	messageID string
	thread    core.AgentThread
	options   core.RunOptions
	history   []core.Message

	// agent-specific state (swappable during handoff)
	agent *AgentInstance

	// Cohesive Units
	safety      *SafetyUnit
	observer    *ObserverUnit
	persistence *PersistenceUnit
	tools       *ToolProcessor

	// Current Turn State
	sm               *stateless.StateMachine
	streamResult     *StreamResult
	resolution       *RouteResolver
	stateConfigCache []StateDefinition // Cached to avoid recreation in nextStep()

	// Tool state machines for tracking individual tool execution states
	toolStateMachines map[string]*domain.ToolStateMachine
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
		return s.nextStep()
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

	return s.nextStep()
}

func (s *executionState) executeRequiredActions() error {
	// Create tool state machines for each tool call
	s.toolStateMachines = make(map[string]*domain.ToolStateMachine)

	// Initialize executor if needed (e.g., when resuming from history)
	if s.agent.Executor == nil {
		tools := s.agent.Config.Tools
		if tools == nil {
			tools = []core.Tool{}
		}
		s.agent.Executor = s.engine.toolExec.CreateExecutor(
			s.ctx, tools, s.threadID, s.messageID, nil,
		)
	}

	// Create state observer to emit events
	stateObserver := NewToolStateEventEmitter(
		s.threadID,
		s.messageID,
		s.observer.yield,
		s.agent.Events,
	)

	// Create state machines and register observers
	for _, tc := range s.streamResult.ToolCalls {
		tsm := domain.NewToolStateMachine(s.ctx, &tc, s.agent.Executor)
		tsm.RegisterObserver(stateObserver)
		s.toolStateMachines[tc.ID] = tsm
	}

	// Execute all tools using their state machines
	tctx := ToolProcessorContext{
		Ctx:               s.ctx,
		ThreadID:          s.threadID,
		MessageID:         s.messageID,
		StreamResult:      s.streamResult,
		Safety:            s.safety,
		Observer:          s.observer,
		Persistence:       s.persistence,
		AgentName:         s.agent.Name,
		ToolStateMachines: s.toolStateMachines,
	}
	resolution, agg, err := s.tools.ProcessActions(tctx)
	if err != nil {
		return err
	}
	if agg != nil {
		s.refreshState(agg)
	}
	s.resolution = resolution

	// Check if any tool is awaiting interaction
	if s.hasToolsAwaitingInteraction() {
		s.observer.result.Reason = TerminationAwaitingInput
		return s.sm.Fire(TrigStopIteration)
	}

	// Set termination reason when a tool requests to stop (e.g., elicitation)
	if resolution.StopIteration {
		s.observer.result.Reason = TerminationAwaitingInput
	}

	return s.nextStep()
}

// hasToolsAwaitingInteraction returns true if any tool is in a waiting state.
func (s *executionState) hasToolsAwaitingInteraction() bool {
	for _, tsm := range s.toolStateMachines {
		if tsm.IsAwaitingInteraction() {
			return true
		}
	}
	return false
}

func (s *executionState) refreshState(agg *domain.ThreadAggregate) {
	s.thread = s.engine.threadMgmt.ToAgentThread(agg)
	s.history = s.engine.threadMgmt.ToCoreMessages(agg)
}

// nextStep determines the appropriate state transition based on declarative guards.
// The actual transition logic is now encoded in the state configuration.
func (s *executionState) nextStep() error {
	currentState := s.sm.MustState()

	// Find the state definition to access guards (using cached config)
	for _, stateDef := range s.stateConfigCache {
		if stateDef.State != currentState {
			continue
		}

		// Evaluate transitions with guards in priority order
		for _, trans := range stateDef.Transitions {
			if trans.Guard == nil {
				continue // Skip unconditional transitions for now
			}

			// Check if guard condition is met
			if trans.Guard(s) {
				return s.sm.Fire(trans.Trigger)
			}
		}

		// Fall through to default transitions based on state
		break
	}

	// Default behavior for each state
	switch currentState {
	case StateGeneratingResponse:
		return s.sm.Fire(TrigTurnFinished)
	case StatePerformingActions:
		// Prepare for next iteration
		s.messageID = core.GenerateID()
		s.streamResult = nil
		s.resolution = nil
		return s.sm.Fire(TrigActionsCompleted)
	default:
		return s.sm.Fire(TrigTurnFinished)
	}
}

// HistoryReconciler analyzes the conversation history to determine the resumption state.
type HistoryReconciler struct {
	history   []core.Message
	agentName string
}

// ResumptionPlan describes an action to take to pair a message during state inference.
type ResumptionPlan struct {
	ToolCallID string
	MessageID  string
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

// AgentInstance holds everything specific to the agent currently in control.
type AgentInstance struct {
	Name     string
	Config   core.AgentConfig
	LLM      *LLMContextManager
	Stream   *StreamProcessor
	Events   *EventFactory
	Executor core.ToolExecutor
}

func (e *Engine) newAgentInstance(ctx context.Context, name string) (*AgentInstance, error) {
	config, err := e.provider.GetConfig(ctx, name)
	if err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	events := NewEventFactory(config.Name)
	return &AgentInstance{
		Name:   config.Name,
		Config: config,
		LLM:    NewLLMContextManager(config, e.threadMgmt, e.reqCtxCapture),
		Events: events,
		Stream: NewStreamProcessor(events),
	}, nil
}

// PersistenceUnit ensures atomic updates between the database and in-memory state.
type PersistenceUnit struct {
	mgmt *threadmgmt.Manager
}

func (p *PersistenceUnit) AddAssistantMessage(
	ctx context.Context,
	threadID, messageID string,
	content string,
	toolCalls []domain.ToolCall,
	agentName string,
) (*domain.ThreadAggregate, error) {
	return p.mgmt.AddAssistantMessage(ctx, threadID, messageID, content, toolCalls, agentName)
}

func (p *PersistenceUnit) AddToolResultMessage(
	ctx context.Context,
	threadID string,
	toolCallID string,
	result string,
	agentName string,
) (*domain.ThreadAggregate, error) {
	toolResultID := core.GenerateID()
	return p.mgmt.AddToolResultMessage(ctx, threadID, toolResultID, toolCallID, result, agentName)
}

func (p *PersistenceUnit) ReportTokenUsage(
	ctx context.Context,
	event domain.TokenUsageReported,
) (*domain.ThreadAggregate, error) {
	return p.mgmt.AddTokenUsage(ctx, event)
}

func (p *PersistenceUnit) ReportToolEvent(
	ctx context.Context,
	event core.Event,
) (*domain.ThreadAggregate, error) {
	return p.mgmt.RecordToolEvent(ctx, event.AggregateID(), event)
}

// ToolProcessor handles the execution of tools and determination of next steps.
type ToolProcessor struct{}

// ToolProcessorContext provides the dependencies needed for tool processing.
type ToolProcessorContext struct {
	Ctx               context.Context
	ThreadID          string
	MessageID         string
	StreamResult      *StreamResult
	Safety            *SafetyUnit
	Observer          *ObserverUnit
	Persistence       *PersistenceUnit
	AgentName         string
	ToolStateMachines map[string]*domain.ToolStateMachine
}

func (p *ToolProcessor) ProcessActions(tctx ToolProcessorContext) (*RouteResolver, *domain.ThreadAggregate, error) {
	var route RouteResolver
	var latestAgg *domain.ThreadAggregate

	// 1. Parallel execution using errgroup
	g, _ := errgroup.WithContext(tctx.Ctx)

	for _, tc := range tctx.StreamResult.ToolCalls {
		tc := tc // Shadow for closure
		tsm := tctx.ToolStateMachines[tc.ID]

		g.Go(func() error {
			return tsm.Start()
		})
	}

	// Wait for all tools to finish (or first error)
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	// 2. Sequential processing of results to maintain order and thread-safety
	for _, tc := range tctx.StreamResult.ToolCalls {
		tctx.Observer.toolNames = append(tctx.Observer.toolNames, tc.Function.Name)

		if info := tctx.Safety.RecordToolCall(tc); info != nil {
			return nil, nil, tctx.Observer.FailWithLoop(info)
		}

		tsm := tctx.ToolStateMachines[tc.ID]
		result := tsm.Result()
		if tsm.IsAwaitingInteraction() {
			route.StopIteration = true
		}

		if result == nil {
			continue
		}

		if info := tctx.Safety.RecordToolResult(result.ToolName, result.Result, result.IsError); info != nil {
			return nil, nil, tctx.Observer.FailWithLoop(info)
		}

		agg, err := tctx.Persistence.AddToolResultMessage(
			tctx.Ctx,
			tctx.ThreadID,
			tc.ID,
			result.Result,
			tctx.AgentName,
		)
		if err != nil {
			return nil, nil, err
		}
		latestAgg = agg

		if res := result.Command; res != nil {
			if res.StopIteration {
				route.StopIteration = true
			}
		}
	}

	return &route, latestAgg, nil
}

// RouteResolver determines the next state based on tool results.
type RouteResolver struct {
	StopIteration bool // Tool requested to stop iteration (e.g., elicitation)
}

func (r *RouteResolver) HasAction() bool {
	return r.StopIteration
}

func (s *executionState) prepareContextForLLM() (*PrepareResult, error) {
	runCtx := core.RunContext{
		ThreadID:  s.threadID,
		MessageID: s.messageID,
		Thread:    s.thread,
		Options:   s.options,
	}
	return s.agent.LLM.Prepare(s.ctx, runCtx, s.history)
}

func (s *executionState) initAgentComponents(name string) error {
	instance, err := s.engine.newAgentInstance(s.ctx, name)
	if err != nil {
		return err
	}
	s.agent = instance
	s.observer.events = instance.Events
	s.safety.ResetAgentLimits()
	return nil
}

func (s *executionState) setup(initialState state) error {
	s.sm = stateless.NewStateMachine(initialState)

	// Cache state configuration for efficient lookup in nextStep()
	s.stateConfigCache = s.stateConfiguration()

	// Build state machine from declarative configuration
	for _, stateDef := range s.stateConfigCache {
		config := s.sm.Configure(stateDef.State)

		// Attach lifecycle hooks
		if stateDef.OnEntry != nil {
			config.OnEntry(func(_ context.Context, args ...any) error {
				return stateDef.OnEntry(s)
			})
		}
		if stateDef.OnExit != nil {
			config.OnExit(func(_ context.Context, _ ...any) error {
				return stateDef.OnExit(s)
			})
		}

		// Configure transitions
		for _, trans := range stateDef.Transitions {
			if trans.Reentry {
				config.PermitReentry(trans.Trigger)
			} else {
				// Note: Guards are evaluated in nextStep() using CanFire
				// The stateless library will check them via CanFire before Fire
				config.Permit(trans.Trigger, trans.Target)
			}
		}
	}

	return nil
}

// stateConfiguration provides the complete declarative state machine specification.
func (s *executionState) stateConfiguration() []StateDefinition {
	return []StateDefinition{
		{
			State: StateAwaitingTurn,
			Transitions: []TransitionRule{
				{Trigger: TrigStartExecution, Target: StateGeneratingResponse},
			},
		},
		{
			State: StateGeneratingResponse,
			OnEntry: func(s *executionState) error {
				s.observer.TrackStarted(s.messageID)
				return s.generateAgentResponse()
			},
			OnExit: func(s *executionState) error {
				if s.streamResult != nil {
					s.observer.TrackCompleted(s.messageID, s.streamResult.Content, toDomainToolCalls(s.streamResult.ToolCalls))
				}
				return nil
			},
			Transitions: []TransitionRule{
				{Trigger: TrigStartExecution, Target: StateGeneratingResponse, Reentry: true},
				// Conditional transitions based on response content
				{
					Trigger: TrigActionsDetected,
					Target:  StatePerformingActions,
					Guard: func(s *executionState) bool {
						return s.streamResult != nil && s.streamResult.HasToolCalls()
					},
				},
				{Trigger: TrigTurnFinished, Target: StateDone},
			},
		},
		{
			State: StatePerformingActions,
			OnEntry: func(s *executionState) error {
				return s.executeRequiredActions()
			},
			Transitions: []TransitionRule{
				{Trigger: TrigStartExecution, Target: StatePerformingActions, Reentry: true},
				// Conditional transitions based on tool results

				{
					Trigger: TrigStopIteration,
					Target:  StateDone,
					Guard: func(s *executionState) bool {
						// Only stop for StopIteration if NOT a handoff
						return s.resolution != nil && s.resolution.StopIteration
					},
				},
				{Trigger: TrigActionsCompleted, Target: StateGeneratingResponse},
			},
		},

		{
			State:       StateDone,
			Transitions: []TransitionRule{}, // Terminal state
		},
	}
}

func (s *executionState) run() error {
	return s.sm.Fire(TrigStartExecution)
}

func (s *executionState) inferState() state {
	reconciler := &HistoryReconciler{
		history:   s.history,
		agentName: s.agent.Name,
	}
	st, res, plan := reconciler.Reconcile()

	if plan != nil {
		// Emit tool input provided event
		s.observer.yield(s.observer.events.ToolInputProvided(
			s.threadID,
			plan.MessageID,
			plan.ToolCallID,
			plan.Content,
		), nil)

		agg, err := s.persistence.AddToolResultMessage(
			s.ctx,
			s.threadID,
			plan.ToolCallID,
			plan.Content,
			s.agent.Name,
		)
		if err != nil {
			return StateError
		}
		s.refreshState(agg)
	}

	s.streamResult = res
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

	// 3. Resumption Logic: Check for active tool states
	// We check the persisted state metadata to see if any tool is waiting for input or auth
	if states, ok := lastAssistantWithTools.Metadata["tool_states"].(map[string]domain.ToolCallState); ok {
		if state, ok := states[pendingTC.ID]; ok {
			if state.Status == "AwaitingInput" || state.Status == "AwaitingAuth" {
				return r.reconcileElicitation(pendingTC, lastAssistantWithTools.ID)
			}
		}
	}

	// Populate streamResult for traditional tool execution (only pending ones)
	res := &StreamResult{
		Content: lastAssistantWithTools.Content,
	}

	completedCalls := make(map[string]bool)
	for i := len(r.history) - 1; i >= 0; i-- {
		if r.history[i].Role == "tool" {
			completedCalls[r.history[i].ToolCallID] = true
		}
	}

	for _, tc := range lastAssistantWithTools.ToolCalls {
		if !completedCalls[tc.ID] {
			res.ToolCalls = append(res.ToolCalls, tc)
		}
	}

	if len(res.ToolCalls) == 0 {
		return StateAwaitingTurn, nil, nil
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

func (r *HistoryReconciler) reconcileElicitation(tc *core.ToolCall, parentMessageID string) (state, *StreamResult, *ResumptionPlan) {
	lastMsg := r.history[len(r.history)-1]

	if lastMsg.Role == "user" {
		// Plan to automate Pairing: Fulfill the tool call with the user message content
		return StateGeneratingResponse, nil, &ResumptionPlan{
			ToolCallID: tc.ID,
			MessageID:  parentMessageID,
			Content:    lastMsg.Content,
		}
	}

	if lastMsg.Role == "tool" && lastMsg.ToolCallID == tc.ID {
		return StateGeneratingResponse, nil, nil
	}

	// Still waiting for user input - return Done (paused state)
	return StateDone, nil, nil
}

func (s *executionState) streamAndProcessLLMResponse(llmContext *LLMContext) error {
	s.agent.Executor = s.engine.toolExec.CreateExecutor(
		s.ctx, llmContext.GetEffectiveTools(),
		s.threadID, s.messageID, llmContext.RequestCtx,
	)

	streamIter := s.engine.chatClient.GetStreamingResponse(s.ctx, llmContext.Messages, llmContext.Options)

	res, err := s.agent.Stream.Process(s.ctx, streamIter, ProcessParams{
		ThreadID:  s.threadID,
		MessageID: s.messageID,
		Provider:  s.engine.chatClient.Provider(),
		Model:     string(llmContext.Options.ModelID),
	}, s.observer.yield) // Use the already wrapped yield
	if err != nil {
		return err
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
	agg, err := s.persistence.AddAssistantMessage(
		s.ctx,
		s.threadID,
		s.messageID,
		s.streamResult.Content,
		toolCalls,
		s.agent.Name,
	)
	if err != nil {
		return err
	}
	s.refreshState(agg)
	return nil
}

// State machine states
type state int

const (
	StateAwaitingTurn state = iota
	StateGeneratingResponse
	StatePerformingActions

	StateDone
	StateError
)

// String provides human-readable state names for debugging.
func (s state) String() string {
	switch s {
	case StateAwaitingTurn:
		return "AwaitingTurn"
	case StateGeneratingResponse:
		return "GeneratingResponse"
	case StatePerformingActions:
		return "PerformingActions"

	case StateDone:
		return "Done"
	case StateError:
		return "Error"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

const (
	TrigStartExecution   = "StartExecution"
	TrigActionsDetected  = "ActionsDetected"
	TrigTurnFinished     = "TurnFinished"
	TrigActionsCompleted = "ActionsCompleted"

	TrigStopIteration = "StopIteration" // Tool requested to stop (e.g., elicitation)
)

// Safety and iteration limits
const (
	DefaultMaxTotalIterations = 20
	DefaultMaxAgentIterations = 5
)

// StateDefinition declaratively defines a state's behavior and transitions.
type StateDefinition struct {
	State       state
	OnEntry     func(*executionState) error
	OnExit      func(*executionState) error
	Transitions []TransitionRule
}

// TransitionRule declaratively defines when and how a state transition occurs.
type TransitionRule struct {
	Trigger   string
	Target    state
	Guard     func(*executionState) bool // Optional condition for transition
	Reentry   bool                       // If true, allows transition to same state
	WithParam bool                       // If true, transition requires a parameter
}

// ToolStateEventEmitter emits domain events for tool state changes.
type ToolStateEventEmitter struct {
	threadID  string
	messageID string
	yield     func(core.Event, error) bool
	events    *EventFactory
}

// NewToolStateEventEmitter creates a new event emitter for tool state changes.
func NewToolStateEventEmitter(
	threadID string,
	messageID string,
	yield func(core.Event, error) bool,
	events *EventFactory,
) *ToolStateEventEmitter {
	return &ToolStateEventEmitter{
		threadID:  threadID,
		messageID: messageID,
		yield:     yield,
		events:    events,
	}
}

// OnToolStateChanged handles tool state transitions and emits appropriate events.
func (e *ToolStateEventEmitter) OnToolStateChanged(
	toolCallID string,
	toolName string,
	oldState domain.ToolState,
	newState domain.ToolState,
	result *domain.ToolExecutionResult,
	elicitation *core.ElicitationSpec,
) {
	// Emit generic state changed event
	evt := e.events.ToolStateChanged(
		e.threadID,
		e.messageID,
		toolCallID,
		toolName,
		oldState.String(),
		newState.String(),
		result,
	)
	e.yield(evt, nil)

	// Emit specific events based on new state
	switch newState {
	case domain.ToolStateAwaitingInput:
		if elicitation != nil {
			awaitEvt := e.events.ToolAwaitingInput(
				e.threadID,
				e.messageID,
				toolCallID,
				toolName,
				elicitation,
			)
			e.yield(awaitEvt, nil)
		}

	case domain.ToolStateAwaitingAuth:
		// Authorization events will be added in future
		// For now, just emit state change

	case domain.ToolStateCompleted:
		// Tool completion is already tracked via ToolCallExecuted event
		// The state change event provides additional granularity

	case domain.ToolStateError:
		// Errors are already tracked via ToolCallExecuted event with IsError=true
		// The state change event provides additional granularity
	}
}

// Verify ToolStateEventEmitter implements ToolStateObserver.
var _ domain.ToolStateObserver = (*ToolStateEventEmitter)(nil)
