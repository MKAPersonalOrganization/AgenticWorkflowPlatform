package engine

import (
	"context"
	"fmt"
	"iter"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
	"school-with-ai/foundation/logger"

	"github.com/qmuntal/stateless"
	"golang.org/x/sync/errgroup"
)

// ChatClient is the LLM client interface.
type ChatClient = llm.ChatClient

// AgentProvider provides agent configurations by name.
type AgentProvider interface {
	GetConfig(ctx context.Context, name string) (core.AgentConfig, error)
}

// Engine handles the core LLM iteration and streaming logic in a generic way.
type Engine struct {
	log           *logger.Logger
	chatClient    ChatClient
	provider      AgentProvider
	reqCtxCapture **core.RequestContext // optional capture for testing
	toolTracer    core.ToolTracer
	toolExec      *ToolExecutionService
}

// New creates a new generic execution engine.
func New(
	log *logger.Logger,
	chatClient ChatClient,
	provider AgentProvider,
	reqCtxCapture **core.RequestContext,
	toolTracer core.ToolTracer,
) *Engine {
	return &Engine{
		log:           log,
		chatClient:    chatClient,
		provider:      provider,
		reqCtxCapture: reqCtxCapture,
		toolTracer:    toolTracer,
		toolExec:      NewToolExecutionService(toolTracer),
	}
}

// GetGraph returns the DOT graph representation of the engine state machine.
func (e *Engine) GetGraph() string {
	// Create a dummy execution state to configure the state machine
	exec := &executionState{
		engine: e,
		// We don't need real dependencies for graph generation
		safety:   &SafetyUnit{},
		observer: &ObserverUnit{},
		tools:    &ToolProcessor{},
	}

	// We can use any start state, setup will configure all of them
	_ = exec.setup(StateAwaitingExecution)

	return exec.sm.ToGraph()
}

// Run processes the LLM response using an AgentThread and yields events.
func (e *Engine) Run(ctx context.Context, agentName string, runCtx core.RunContext) iter.Seq2[core.Event, error] {
	return func(yield func(core.Event, error) bool) {
		e.log.Info(ctx, "engine run started", "agent", agentName, "thread_id", runCtx.ThreadID)
		type result struct {
			evt core.Event
			err error
		}
		ch := make(chan result, 100)
		ctx, cancel := context.WithCancel(ctx)

		wrappedYield := func(evt core.Event, err error) bool {
			select {
			case ch <- result{evt: evt, err: err}:
				return true
			case <-ctx.Done():
				return false
			}
		}

		go func() {
			defer close(ch)
			defer cancel()
			defer e.log.Info(ctx, "engine background goroutine finished")

			exec, err := e.newExecutionState(ctx, agentName, runCtx, wrappedYield)
			if err != nil {
				wrappedYield(nil, err)
				return
			}

			// Initialize executor before state machine runs
			exec.prepareExecutor()

			// Start with AwaitingExecution
			if err := exec.setup(StateAwaitingExecution); err != nil {
				wrappedYield(nil, err)
				return
			}

			// Handle user message submission before state machine runs
			if err := exec.handleUserMessageSubmission(); err != nil {
				wrappedYield(nil, fmt.Errorf("failed to handle user message: %w", err))
				return
			}

			if err := exec.run(); err != nil {
				if err != errStopIteration {
					exec.handleExecutionError(ctx, err)
					wrappedYield(nil, err)
				}
				return
			}

			// Finalize and yield internal completion event with termination reason
			exec.finalizeResult()
			wrappedYield(runCompletedEvent{Reason: exec.observer.result.Reason}, nil)
		}()

		for res := range ch {
			if !yield(res.evt, res.err) {
				e.log.Info(ctx, "engine run stop requested by consumer")
				cancel()
				return
			}
		}
		e.log.Info(ctx, "engine run completed normal exit", "thread_id", runCtx.ThreadID)
	}
}

// handleExecutionError updates the result and completion metadata based on error type.
func (exec *executionState) handleExecutionError(ctx context.Context, err error) {
	// Update result based on error type, but only if not already set to a specific reason
	if exec.observer.result.Reason == TerminationGoalReached {
		if ctx.Err() != nil {
			exec.observer.result.Reason = TerminationUserAborted
			exec.completionReason = CompletionReasonUserAborted
		} else {
			exec.observer.result.Reason = TerminationError
			exec.observer.result.Error = err
			exec.completionReason = CompletionReasonError
		}
		exec.observer.result.Success = false
	} else {
		// Completion reason already set (e.g., MaxIterations, LoopDetected)
		// Map the existing termination reason to completion reason
		exec.completionReason = terminationReasonToCompletionReason(exec.observer.result.Reason)
	}
	// Fire transition to completed state to trigger event emission
	_ = exec.sm.Fire(TrigExecutionFailed)
}

// terminationReasonToCompletionReason converts TerminationReason to CompletionReason.
func terminationReasonToCompletionReason(tr TerminationReason) CompletionReason {
	switch tr {
	case TerminationGoalReached:
		return CompletionReasonSuccess
	case TerminationMaxIterations:
		return CompletionReasonMaxIterations
	case TerminationLoopDetected:
		return CompletionReasonLoopDetected
	case TerminationTimeout:
		return CompletionReasonTimeout
	case TerminationUserAborted:
		return CompletionReasonUserAborted
	case TerminationError:
		return CompletionReasonError
	default:
		return CompletionReasonError
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
		engine:      e,
		ctx:         ctx,
		threadID:    runCtx.ThreadID,
		messageID:   runCtx.MessageID,
		thread:      runCtx.Thread,
		options:     runCtx.Options,
		agent:       instance,
		history:     runCtx.InitialHistory,
		userContent: runCtx.UserContent,
		userID:      runCtx.UserID,
		observer: &ObserverUnit{
			yield:     yield,
			startTime: timeNowNano(),
			threadID:  runCtx.ThreadID,
			events:    instance.Events,
			result: &ExecutionResult{
				Success:    true,
				Reason:     TerminationGoalReached,
				FinalAgent: agentName,
			},
		},
		tools: &ToolProcessor{},
	}

	exec.safety = &SafetyUnit{
		iteration:    runCtx.Iteration,
		total:        runCtx.Total,
		maxTotal:     DefaultMaxTotalIterations,
		loopDetector: NewLoopDetector(DefaultLoopDetectorConfig()),
		maxAgent:     instance.Config.MaxIterations,
	}

	if exec.safety.maxAgent <= 0 {
		exec.safety.maxAgent = DefaultMaxAgentIterations
	}

	if runCtx.LoopState != "" {
		if err := exec.safety.loopDetector.Load(runCtx.LoopState); err != nil {
			e.log.Error(ctx, "failed to load loop detector state", "error", err)
		}
	}

	return exec, nil
}

// prepareExecutor initializes the tool executor with base tools.
// This is called before the state machine runs to ensure the executor
// is available regardless of which execution path is taken.
func (s *executionState) prepareExecutor() {
	// Use the agent's base tools from configuration
	// When the LLM is actually called, middleware will be applied
	// and a new executor with effective tools will be created.
	// This provides a fallback executor for state resumption scenarios.
	tools := s.agent.Config.Tools
	if tools == nil {
		tools = []core.Tool{}
	}

	s.agent.Executor = s.engine.toolExec.CreateExecutor(
		s.ctx,
		tools,
		s.threadID,
		s.messageID,
		nil, // No RequestContext yet - will be set during LLM call
	)
}

var errStopIteration = fmt.Errorf("stop iteration")

// emit is deprecated, logic moved to wrapYield or handled by caller.
func (e *Engine) emit(ctx context.Context, evt core.Event) error {
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

	// User message submission (if this is a new user message)
	userContent string
	userID      string

	// agent-specific state (swappable during handoff)
	agent *AgentInstance

	// Cohesive Units
	safety   *SafetyUnit
	observer *ObserverUnit
	tools    *ToolProcessor

	// Current Turn State
	sm           *stateless.StateMachine
	streamResult *StreamResult
	resolution   *RouteResolver

	// State metadata - tracks WHY we're in a particular state
	pauseReason      PauseReason      // Why paused (input/auth)
	completionReason CompletionReason // Why completed (success/error type)

	// Tool state machines for tracking individual tool execution states
	toolStateMachines map[string]*domain.ToolStateMachine
}

func (s *executionState) generateAgentResponse() error {
	if err := s.safety.Guard(); err != nil {
		s.observer.Fail(TerminationMaxIterations, err)
		return err
	}
	// Yield state update
	if !s.observer.yield(s.safety.Report(s.threadID), nil) {
		return errStopIteration
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
		AgentName:         s.agent.Name,
		ToolStateMachines: s.toolStateMachines,
		Yield:             s.observer.yield,
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
	isInput, isAuth, err := s.checkToolWaitingStates()
	if err != nil {
		s.observer.Fail(TerminationError, err)
		s.completionReason = CompletionReasonError
		return s.sm.Fire(TrigExecutionFailed)
	}

	if isInput {
		s.observer.result.Reason = TerminationAwaitingInput
		s.pauseReason = PauseReasonInput
		return s.sm.Fire(TrigAwaitUserInput)
	}
	if isAuth {
		s.observer.result.Reason = TerminationAwaitingAuth
		s.pauseReason = PauseReasonAuth
		return s.sm.Fire(TrigAwaitUserInput)
	}

	return s.nextStep()
}

// checkToolWaitingStates checks if any tools are in specific waiting states.
// Returns (isAwaitingInput, isAwaitingAuth, error)
func (s *executionState) checkToolWaitingStates() (bool, bool, error) {
	hasInput := false
	hasAuth := false
	for _, tsm := range s.toolStateMachines {
		if tsm.IsAwaitingInput() {
			hasInput = true
		}
		if tsm.IsAwaitingAuth() {
			hasAuth = true
		}
	}

	if hasInput && hasAuth {
		return false, false, fmt.Errorf("invalid state: simultaneous input and authorization requirements detected")
	}

	return hasInput, hasAuth, nil
}

func (s *executionState) refreshState(history []core.Message) {
	s.history = history
}

// nextStep determines the appropriate state transition based on current context.
// It simplifies the logic by explicitly checking conditions for the current state,
// removing the need for a generic transition evaluator.
func (s *executionState) nextStep() error {
	currentState := s.sm.MustState()

	switch currentState {
	case StateInvokingLLM:
		// Transition based on whether tools were called
		if s.streamResult != nil && s.streamResult.HasToolCalls() {
			return s.sm.Fire(TrigToolsDetected)
		}
		// No tools called, execution complete successfully
		s.completionReason = CompletionReasonSuccess
		s.observer.result.Reason = TerminationGoalReached
		return s.sm.Fire(TrigExecutionComplete)

	case StateExecutingTools:
		// Tool waiting states (Input/Auth) are handled in executeRequiredActions.
		// If we reach here, we are ready for the next turn.

		// Prepare for next iteration: Generate new message ID for the NEXT turn
		// This happens AFTER the current turn (ExecutingTools) is finished
		s.messageID = core.GenerateID()
		s.streamResult = nil
		s.resolution = nil
		return s.sm.Fire(TrigToolsCompleted)

	default:
		// Default termination for safety
		s.completionReason = CompletionReasonSuccess
		s.observer.result.Reason = TerminationGoalReached
		return s.sm.Fire(TrigExecutionComplete)
	}
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

func (s *SafetyUnit) Report(threadID string) *domain.ExecutionStateReported {
	return &domain.ExecutionStateReported{
		ThreadID:     threadID,
		Iteration:    s.iteration,
		Total:        s.total,
		LoopStateHex: s.loopDetector.Serialize(),
		Timestamp:    time.Now(),
	}
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
		LLM:    NewLLMContextManager(config, e.reqCtxCapture),
		Events: events,
		Stream: NewStreamProcessor(events),
	}, nil
}

// PersistenceUnit is deprecated.

// ToolProcessor handles the execution of tools and determination of next steps.
type ToolProcessor struct{}

// ToolProcessorContext provides the dependencies needed for tool processing.
type ToolProcessorContext struct {
	Ctx               context.Context
	ThreadID          string
	MessageID         string
	StreamResult      *StreamResult
	Safety            *SafetyUnit
	AgentName         string
	ToolStateMachines map[string]*domain.ToolStateMachine
	Yield             func(core.Event, error) bool
}

func (p *ToolProcessor) ProcessActions(tctx ToolProcessorContext) (*RouteResolver, []core.Message, error) {
	var route RouteResolver
	var newMessages []core.Message

	// 1. Parallel execution using errgroup
	g, _ := errgroup.WithContext(tctx.Ctx)

	for _, tc := range tctx.StreamResult.ToolCalls {
		tsm := tctx.ToolStateMachines[tc.ID]

		if info := tctx.Safety.RecordToolCall(tc); info != nil {
			return nil, nil, fmt.Errorf("loop detected: %s", info.Description)
		}
		// Yield state update
		if !tctx.Yield(tctx.Safety.Report(tctx.ThreadID), nil) {
			return nil, nil, fmt.Errorf("yield interrupted")
		}

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
		tsm := tctx.ToolStateMachines[tc.ID]
		result := tsm.Result()
		if tsm.IsAwaitingInteraction() {
			route.StopIteration = true
		}

		if result == nil {
			continue
		}

		if info := tctx.Safety.RecordToolResult(result.ToolName, result.Result, result.IsError); info != nil {
			// FailWithLoop requires ObserverUnit, but we can return error directly
			return nil, nil, fmt.Errorf("loop detected: %s", info.Description)
		}
		// Yield state update
		if !tctx.Yield(tctx.Safety.Report(tctx.ThreadID), nil) {
			return nil, nil, fmt.Errorf("yield interrupted")
		}

		// Yield ToolResultAdded event (durable)
		toolResultEvent := &domain.ToolResultAdded{
			ThreadID:   tctx.ThreadID,
			MessageID:  core.GenerateID(),
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			Result:     result.Result,
			AgentName:  tctx.AgentName,
			Timestamp:  time.Now(),
		}
		if !tctx.Yield(toolResultEvent, nil) {
			return nil, nil, errStopIteration
		}

		// Create message for internal history
		newMessages = append(newMessages, core.Message{
			ID:         toolResultEvent.MessageID,
			Role:       "tool",
			Content:    result.Result,
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			IsError:    result.IsError,
			AgentName:  tctx.AgentName,
			Timestamp:  toolResultEvent.Timestamp,
		})

		if res := result.Command; res != nil {
			if res.StopIteration {
				route.StopIteration = true
			}
		}
	}

	return &route, newMessages, nil
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
		ThreadID:       s.threadID,
		UserID:         s.userID,
		MessageID:      s.messageID,
		Thread:         s.thread,
		InitialHistory: s.history,
		Options:        s.options,
	}

	s.engine.log.Debug(s.ctx, "preparing LLM context", "historyCount", len(s.history))
	for i, msg := range s.history {
		s.engine.log.Debug(s.ctx, "history message", "index", i, "role", msg.Role, "contentLength", len(msg.Content), "toolCallsCount", len(msg.ToolCalls), "toolCallID", msg.ToolCallID, "toolName", msg.ToolName)
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

// handleUserMessageSubmission yields a UserMessageAdded event.
func (s *executionState) handleUserMessageSubmission() error {
	// Only process if this is a new user message submission
	if s.userContent == "" {
		return nil
	}

	// Generate unique message ID for the user message
	userMessageID := core.GenerateID()

	// Yield UserMessageAdded event (durable)
	userMsgEvent := &domain.UserMessageAdded{
		ThreadID:  s.threadID,
		UserID:    s.userID,
		MessageID: userMessageID,
		Content:   s.userContent,
		Timestamp: time.Now(),
	}
	if !s.observer.yield(userMsgEvent, nil) {
		return errStopIteration
	}

	// Update internal history
	s.history = append(s.history, core.Message{
		ID:        userMessageID,
		Role:      "user",
		Content:   s.userContent,
		Timestamp: userMsgEvent.Timestamp,
	})

	// Emit RunStarted event (transient)
	if !s.observer.yield(s.observer.events.RunStarted(s.threadID, s.messageID), nil) {
		return errStopIteration
	}

	return nil
}

func (s *executionState) setup(initialState state) error {
	s.sm = stateless.NewStateMachine(initialState)

	// Log and persist state changes
	s.sm.OnTransitioned(func(ctx context.Context, trans stateless.Transition) {
		srcState := trans.Source.(state)
		dstState := trans.Destination.(state)
		trigger := trans.Trigger

		s.engine.log.Info(ctx, "state transition",
			"thread_id", s.threadID,
			"message_id", s.messageID,
			"agent", s.agent.Name,
			"from_state", srcState.String(),
			"to_state", dstState.String(),
			"trigger", trigger,
			"pause_reason", s.pauseReason.String(),
			"completion_reason", s.completionReason.String(),
		)

		s.trackStateChange(dstState)
	})

	// Configure StateAwaitingExecution (replaces StateAwaitingTurn)
	s.sm.Configure(StateAwaitingExecution).
		OnEntry(func(ctx context.Context, _ ...any) error {
			s.engine.log.Info(ctx, "entering StateAwaitingExecution",
				"thread_id", s.threadID,
				"message_id", s.messageID,
				"agent", s.agent.Name,
			)
			return nil
		}).
		Permit(TrigBeginExecution, StateInvokingLLM).
		Permit(TrigExecutionFailed, StateCompleted)

	// Configure StateInvokingLLM (replaces StateInvokingLLMResponse)
	s.sm.Configure(StateInvokingLLM).
		OnEntry(func(ctx context.Context, _ ...any) error {
			s.engine.log.Info(ctx, "entering StateInvokingLLM - calling LLM",
				"thread_id", s.threadID,
				"message_id", s.messageID,
				"agent", s.agent.Name,
			)
			s.observer.TrackStarted(s.messageID)
			return nil
		}).
		OnExit(func(ctx context.Context, _ ...any) error {
			if s.streamResult != nil {
				s.engine.log.Info(ctx, "exiting StateInvokingLLM - LLM response received",
					"thread_id", s.threadID,
					"message_id", s.messageID,
					"agent", s.agent.Name,
					"has_tool_calls", s.streamResult.HasToolCalls(),
					"tool_count", len(s.streamResult.ToolCalls),
				)
				s.observer.TrackCompleted(s.messageID, s.streamResult.Content, toDomainToolCalls(s.streamResult.ToolCalls))
			}
			return nil
		}).
		PermitReentry(TrigBeginExecution).
		Permit(TrigToolsDetected, StateExecutingTools).
		Permit(TrigExecutionComplete, StateCompleted).
		Permit(TrigExecutionFailed, StateCompleted)

	// Configure StateExecutingTools (replaces StatePerformingActions)
	s.sm.Configure(StateExecutingTools).
		OnEntry(func(ctx context.Context, _ ...any) error {
			toolNames := []string{}
			if s.streamResult != nil {
				for _, tc := range s.streamResult.ToolCalls {
					toolNames = append(toolNames, tc.Name)
				}
			}
			s.engine.log.Info(ctx, "entering StateExecutingTools - executing tools",
				"thread_id", s.threadID,
				"message_id", s.messageID,
				"agent", s.agent.Name,
				"tool_count", len(toolNames),
				"tools", toolNames,
			)

			// Emit ActionsDetected event when entering this state
			if s.streamResult != nil && s.streamResult.HasToolCalls() {
				toolCalls := toDomainToolCalls(s.streamResult.ToolCalls)
				if !s.observer.yield(s.agent.Events.ActionsDetected(s.threadID, s.messageID, toolCalls), nil) {
					return errStopIteration
				}
			}
			return nil
		}).
		PermitReentry(TrigBeginExecution).
		Permit(TrigAwaitUserInput, StateAwaitingUserInput).
		Permit(TrigToolsCompleted, StateInvokingLLM).
		Permit(TrigExecutionFailed, StateCompleted)

	// Configure StateAwaitingUserInput (unifies StateAwaitingInput + StateAwaitingAuth)
	s.sm.Configure(StateAwaitingUserInput).
		OnEntry(func(ctx context.Context, _ ...any) error {
			s.engine.log.Info(ctx, "entering StateAwaitingUserInput - paused for user interaction",
				"thread_id", s.threadID,
				"message_id", s.messageID,
				"agent", s.agent.Name,
				"pause_reason", s.pauseReason.String(),
			)
			return nil
		}).
		Permit(TrigBeginExecution, StateInvokingLLM).
		Permit(TrigExecutionFailed, StateCompleted)

	// Configure StateCompleted (terminal state for both success and failure)
	s.sm.Configure(StateCompleted).
		OnEntry(func(ctx context.Context, _ ...any) error {
			s.engine.log.Info(ctx, "entering StateCompleted - execution finished",
				"thread_id", s.threadID,
				"message_id", s.messageID,
				"agent", s.agent.Name,
				"completion_reason", s.completionReason.String(),
				"is_success", s.completionReason.IsSuccess(),
				"termination_reason", s.observer.result.Reason,
				"iterations", s.safety.total,
			)

			// Emit events based on completion reason (metadata)
			if s.completionReason.IsSuccess() {
				// Successful completion: emit turn finished and run finished
				if !s.observer.yield(s.agent.Events.TurnFinished(s.threadID, s.messageID), nil) {
					return errStopIteration
				}
				if !s.observer.yield(s.observer.events.RunFinished(s.threadID, s.messageID), nil) {
					return errStopIteration
				}
			} else {
				// Error termination: emit run error with context
				msg := "Unknown error"
				if s.observer.result.Error != nil {
					msg = s.observer.result.Error.Error()
				}
				s.engine.log.Error(ctx, "execution failed",
					"thread_id", s.threadID,
					"message_id", s.messageID,
					"agent", s.agent.Name,
					"completion_reason", s.completionReason.String(),
					"error", msg,
				)
				if !s.observer.yield(s.observer.events.RunError(s.threadID, msg, "RUN_ERROR"), nil) {
					return errStopIteration
				}
			}
			return nil
		}).
		Permit(TrigBeginExecution, StateInvokingLLM) // Allow restart from completed state

	return nil
}

func (s *executionState) run() error {
	// Start the state machine
	if err := s.sm.Fire(TrigBeginExecution); err != nil {
		return err
	}

	// Execution loop: continue until we hit a terminal or paused state
	for {
		currentState := s.sm.MustState().(state)

		switch currentState {
		case StateInvokingLLM:
			if err := s.generateAgentResponse(); err != nil {
				return err
			}
			// nextStep() within generateAgentResponse will trigger transition to ExecutingTools or Completed
		case StateExecutingTools:
			if err := s.executeRequiredActions(); err != nil {
				return err
			}
			// nextStep() within executeRequiredActions will trigger transition to InvokingLLM or AwaitingUserInput
		case StateAwaitingUserInput, StateCompleted:
			// Terminal or paused states for this run
			return nil
		default:
			return fmt.Errorf("unexpected engine state: %s", currentState.String())
		}

		// Safety break to prevent infinite loops if fire fails to change state
		if s.sm.MustState().(state) == currentState && currentState != StateAwaitingUserInput && currentState != StateCompleted {
			return fmt.Errorf("state machine stuck in state %s", currentState.String())
		}
	}
}

func (s *executionState) streamAndProcessLLMResponse(llmContext *LLMContext) error {
	// Update executor with middleware-processed effective tools
	// This replaces the base executor created in prepareExecutor()
	// with one that has the full context and middleware-applied tools
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
		// Yield state update
		if !s.observer.yield(s.safety.Report(s.threadID), nil) {
			return errStopIteration
		}
	}
	return nil
}

func (s *executionState) saveAgentMessageAndEmitEvents() error {
	toolCalls := toDomainToolCalls(s.streamResult.ToolCalls)

	// Yield AssistantMessageAdded event (durable)
	assistantMsgEvent := &domain.AssistantMessageAdded{
		ThreadID:  s.threadID,
		MessageID: s.messageID,
		Content:   s.streamResult.Content,
		ToolCalls: toolCalls,
		AgentName: s.agent.Name,
		Timestamp: time.Now(),
	}
	if !s.observer.yield(assistantMsgEvent, nil) {
		return errStopIteration
	}

	// Update internal history
	s.history = append(s.history, core.Message{
		ID:        s.messageID,
		Role:      "assistant",
		Content:   s.streamResult.Content,
		ToolCalls: s.streamResult.ToolCalls,
		AgentName: s.agent.Name,
		Timestamp: assistantMsgEvent.Timestamp,
	})

	return nil
}

func (s *executionState) trackStateChange(newState state) {
	if !s.observer.yield(domain.EngineStateChanged{
		ThreadID:  s.threadID,
		NewState:  newState.String(),
		Timestamp: time.Now(),
	}, nil) {
		// We can't return error from here easily if it's a callback,
		// but trackStateChange is called from SM callback.
	}
}

func parseState(str string) state {
	switch str {
	case "AwaitingExecution":
		return StateAwaitingExecution
	case "InvokingLLM":
		return StateInvokingLLM
	case "ExecutingTools":
		return StateExecutingTools
	case "AwaitingUserInput":
		return StateAwaitingUserInput
	case "Completed":
		return StateCompleted
	default:
		// Default to initial state if unknown
		return StateAwaitingExecution
	}
}

// Safety and iteration limits
const (
	DefaultMaxTotalIterations = 20
	DefaultMaxAgentIterations = 5
)

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
	// Emit specific events based on state transitions
	switch newState {
	case domain.ToolStateExecuting:
		// Emit ToolStart when entering executing state from idle
		if oldState == domain.ToolStateIdle {
			evt := e.events.ToolStart(
				e.threadID,
				e.messageID,
				toolCallID,
				toolName,
				"", // Arguments will be added if available
			)
			if result != nil {
				evt.ToolArguments = result.Arguments
			}
			if !e.yield(evt, nil) {
				return
			}
		}

	case domain.ToolStateAwaitingInput:
		if elicitation != nil {
			awaitEvt := e.events.ToolNeedsInput(
				e.threadID,
				e.messageID,
				toolCallID,
				toolName,
				elicitation,
			)
			if !e.yield(awaitEvt, nil) {
				return
			}
		}

	case domain.ToolStateAwaitingAuth:
		// ToolNeedsAuth will be emitted when authorization spec is available
		// For now handled by the tool state machine observer pattern

	case domain.ToolStateCompleted:
		if result != nil {
			evt := e.events.ToolSuccess(
				e.threadID,
				e.messageID,
				toolCallID,
				toolName,
				result.Result,
				result.Arguments,
			)
			if !e.yield(evt, nil) {
				return
			}
		}

	case domain.ToolStateError:
		if result != nil {
			errorMsg := result.Result
			if result.IsError {
				evt := e.events.ToolError(
					e.threadID,
					e.messageID,
					toolCallID,
					toolName,
					errorMsg,
					result.Arguments,
				)
				if !e.yield(evt, nil) {
					return
				}
			}
		}
	}
}

// Verify ToolStateEventEmitter implements ToolStateObserver.
var _ domain.ToolStateObserver = (*ToolStateEventEmitter)(nil)
