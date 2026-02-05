package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"time"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/internal/llm"

	"golang.org/x/sync/errgroup"
)

// ChatClient is the LLM client interface (aliased for convenience).
type ChatClient = llm.LLM

// AgentProvider provides agent configurations by name.
type AgentProvider interface {
	GetConfig(ctx context.Context, name string) (core.AgentConfig, error)
}

// Engine handles the core LLM iteration and streaming logic in a generic way.
type Engine struct {
	chatClient    ChatClient
	provider      AgentProvider
	reqCtxCapture **core.RequestContext // optional capture for testing
	toolTracer    core.ToolTracer
	toolExec      *ToolExecutionService
}

// New creates a new generic execution engine.
func New(
	chatClient ChatClient,
	provider AgentProvider,
	reqCtxCapture **core.RequestContext,
	toolTracer core.ToolTracer,
) *Engine {
	return &Engine{
		chatClient:    chatClient,
		provider:      provider,
		reqCtxCapture: reqCtxCapture,
		toolTracer:    toolTracer,
		toolExec:      NewToolExecutionService(toolTracer),
	}
}

// GetGraph returns an empty string as the state machine has been replaced by a loop.
func (e *Engine) GetGraph() string {
	return ""
}

// Run processes the LLM response using an AgentThread and yields events.
func (e *Engine) Run(ctx context.Context, agentName string, runCtx core.RunContext) iter.Seq2[core.Event, error] {
	return func(yield func(core.Event, error) bool) {
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

			exec, err := e.newExecutionState(ctx, agentName, runCtx, wrappedYield)
			if err != nil {
				wrappedYield(nil, err)
				return
			}

			// Initialize executor before state machine runs
			exec.prepareExecutor()

			// Start turn
			exec.trackStateChange(StateInvokingLLM)

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
				cancel()
				return
			}
		}
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
		// Map the existing termination reason to completion reason
		exec.completionReason = terminationReasonToCompletionReason(exec.observer.result.Reason)
	}
	// Transition to completed state to trigger event emission
	exec.trackStateChange(StateCompleted)
	exec.emitCompletionEvents()
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
	res.FinalAgent = exec.agent.Name()
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
		_ = exec.safety.loopDetector.Load(runCtx.LoopState)
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
		tools = []agent.ExecutableTool{}
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
	streamResult *StreamResult
	resolution   *RouteResolver

	// State metadata - tracks WHY we're in a particular state
	pauseReason      PauseReason      // Why paused (input/auth)
	completionReason CompletionReason // Why completed (success/error type)

	// Tool state machines for tracking individual tool execution states
	toolStateMachines map[string]*core.ToolStateMachine
}

func (s *executionState) executeRequiredActions() error {
	// Create tool state machines for each tool call
	s.toolStateMachines = make(map[string]*core.ToolStateMachine)

	// Create state observer to emit events
	stateObserver := NewToolStateEventEmitter(
		s.threadID,
		s.messageID,
		s.observer.yield,
		s.agent.Events,
	)

	// Create state machines and register observers
	for _, tc := range s.streamResult.ToolCalls {
		tsm := core.NewToolStateMachine(&tc)
		tsm.AddObserver(stateObserver)
		s.toolStateMachines[tc.ID] = tsm
	}

	// Execute all tools using their state machines
	tctx := ToolProcessorContext{
		Ctx:               s.ctx,
		ThreadID:          s.threadID,
		MessageID:         s.messageID,
		StreamResult:      s.streamResult,
		Safety:            s.safety,
		AgentName:         s.agent.Name(),
		ToolStateMachines: s.toolStateMachines,
		Yield:             s.observer.yield,
		Executor:          s.agent.Executor,
		Middlewares:       s.agent.Config.FunctionMiddleware,
		Tools:             s.agent.Config.Tools,
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
		return err
	}

	if isInput {
		s.observer.result.Reason = TerminationAwaitingInput
		s.pauseReason = PauseReasonInput
		s.trackStateChange(StateAwaitingUserInput)
		return nil
	}
	if isAuth {
		s.observer.result.Reason = TerminationAwaitingAuth
		s.pauseReason = PauseReasonAuth
		s.trackStateChange(StateAwaitingUserInput)
		return nil
	}

	// Prepare for next iteration: Generate new message ID for the NEXT turn
	s.messageID = core.GenerateID()
	s.streamResult = nil
	s.resolution = nil

	return nil
}

// checkToolWaitingStates checks if any tools are in specific waiting states.
// Returns (isAwaitingInput, isAwaitingAuth, error)
func (s *executionState) checkToolWaitingStates() (bool, bool, error) {
	hasInput := false
	hasAuth := false
	for _, tsm := range s.toolStateMachines {
		if tsm.IsAwaitingInput(s.ctx) {
			hasInput = true
		}
		if tsm.IsAwaitingAuth(s.ctx) {
			hasAuth = true
		}
	}

	if hasInput && hasAuth {
		return false, false, fmt.Errorf("invalid state: simultaneous input and authorization requirements detected")
	}

	return hasInput, hasAuth, nil
}

func (s *executionState) refreshState(newMessages []core.Message) {
	s.history = append(s.history, newMessages...)
}

// emitCompletionEvents handles the emission of final events based on completion reason.
// Note: Run lifecycle events (RunFinished, RunError) are emitted at the service layer,
// not here. The engine only emits TurnFinished for successful completions.
func (s *executionState) emitCompletionEvents() {
	if s.completionReason.IsSuccess() {
		// Successful completion: emit turn finished only
		// RunFinished will be emitted by the service layer (RunnerAdapter)
		if !s.observer.yield(s.agent.Events.TurnFinished(s.threadID, s.messageID), nil) {
			return
		}
	}
	// On error: do nothing here
	// RunError will be emitted by the service layer (RunnerAdapter)
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

func (s *SafetyUnit) Report(threadID string) *core.ExecutionStateReported {
	return &core.ExecutionStateReported{
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

func (o *ObserverUnit) TrackCompleted(messageID, content string, toolCalls []core.ToolCall) bool {
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
	instanceName string
	Config       core.AgentConfig
	LLM          *LLMContextManager
	Stream       *StreamProcessor
	Events       *EventFactory
	Executor     core.ToolExecutor
}

func (ai *AgentInstance) ID() agent.ID { return agent.ID(ai.instanceName) }

func (ai *AgentInstance) Name() string { return ai.instanceName }

func (ai *AgentInstance) Description() string { return "" }

func (ai *AgentInstance) Run(ctx context.Context, inv *agent.Invocation) iter.Seq2[agent.Event, error] {
	return func(yield func(agent.Event, error) bool) {
		yield(nil, fmt.Errorf("Run not directly supported on AgentInstance; use Engine"))
	}
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
		instanceName: config.Name,
		Config:       config,
		LLM:          NewLLMContextManager(config, e.reqCtxCapture),
		Events:       events,
		Stream:       NewStreamProcessor(events),
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
	ToolStateMachines map[string]*core.ToolStateMachine
	Yield             func(core.Event, error) bool
	Executor          core.ToolExecutor
	Middlewares       []agent.Middleware
	Tools             []agent.ExecutableTool // Added for tool lookup
}

func (p *ToolProcessor) ProcessActions(tctx ToolProcessorContext) (*RouteResolver, []core.Message, error) {
	var route RouteResolver
	var newMessages []core.Message

	// Map tool names to actual agent.Tool if needed, though we only have tc.Name

	// 1. Parallel execution using errgroup
	g, _ := errgroup.WithContext(tctx.Ctx)

	for _, tc := range tctx.StreamResult.ToolCalls {
		tc := tc // capture loop variable
		tsm := tctx.ToolStateMachines[tc.ID]

		if info := tctx.Safety.RecordToolCall(tc); info != nil {
			return nil, nil, fmt.Errorf("loop detected: %s", info.Description)
		}
		// Yield state update
		if !tctx.Yield(tctx.Safety.Report(tctx.ThreadID), nil) {
			return nil, nil, fmt.Errorf("yield interrupted")
		}

		g.Go(func() error {
			// Start the state machine (transition to Executing)
			if err := tsm.Start(tctx.Ctx); err != nil {
				return err
			}

			var execResult core.ToolExecResult
			var execErr error

			// Parse arguments for middleware
			var argsMap map[string]any
			if tc.Arguments != "" {
				_ = json.Unmarshal([]byte(tc.Arguments), &argsMap)
			}

			// Resolve the actual tool if possible
			var tool agent.Tool
			for _, t := range tctx.Tools {
				if t.Name() == tc.Name {
					tool = t
					break
				}
			}

			// Actually execute the tool wrapped in middleware
			err := ExecuteFunctionMiddleware(
				tctx.Ctx,
				tctx.Middlewares,
				tool,
				argsMap,
				map[string]any{
					"agent_name": tctx.AgentName,
					"thread_id":  tctx.ThreadID,
					"message_id": tctx.MessageID,
					"tool_name":  tc.Name,
				},
				func(c *agent.Context) error {
					execResult, execErr = tctx.Executor.Execute(tc)
					c.Result = execResult
					return execErr
				},
			)

			// Handle potential error from ExecuteFunctionMiddleware itself (though unlikely)
			if err != nil && execErr == nil {
				execErr = err
			}

			// Prepare result for state machine (always, even for elicitation/auth)
			toolResult := &core.ToolExecutionResult{
				ToolCallID:    tc.ID,
				ToolName:      tc.Name,
				Arguments:     tc.Arguments,
				Result:        execResult.Result,
				IsError:       execErr != nil,
				StopIteration: execResult.StopIteration,
				Command:       execResult.Command,
			}

			// Handle elicitation (needs input)
			if execResult.Command != nil && execResult.Command.Elicitation != nil {
				return tsm.NeedsInput(execResult.Command.Elicitation, toolResult)
			}

			// Handle authorization (needs auth)
			if execResult.Command != nil && execResult.Command.Authorization != nil {
				return tsm.NeedsAuth(execResult.Command.Authorization, toolResult)
			}

			// Update state machine based on execution result
			if execErr != nil {
				return tsm.Error(toolResult)
			}
			return tsm.Success(toolResult)
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
		if tsm.IsAwaitingInteraction(tctx.Ctx) {
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
		toolResultEvent := &core.ToolResultAdded{
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

func (s *executionState) run() error {
	// Wrap entire agent run with AgentMiddleware
	if len(s.agent.Config.AgentMiddleware) > 0 {
		return ExecuteAgentMiddleware(
			s.ctx,
			s.agent.Config.AgentMiddleware,
			s.agent,
			nil, // messages (core.Message, not agent.Message)
			nil, // session  (not available in engine)
			false,
			map[string]any{
				"agent_name":    s.agent.Name(),
				"thread_id":     s.threadID,
				"message_id":    s.messageID,
				"user_id":       s.userID,
				"message_count": len(s.history),
			},
			func() error {
				return s.runCore()
			},
		)
	}
	return s.runCore()
}

func (s *executionState) runCore() error {
	// 1. Initial State
	s.trackStateChange(StateInvokingLLM)

	for i := 0; i < s.safety.maxAgent; i++ {
		// 3. Report Status
		if !s.observer.yield(s.safety.Report(s.threadID), nil) {
			return errStopIteration
		}

		// 4. Invoke LLM
		s.observer.TrackStarted(s.messageID)
		prepped, err := s.prepareContextForLLM()
		if err != nil {
			s.handleExecutionError(s.ctx, err)
			return err
		}

		if prepped.Terminated {
			break
		}

		if err := s.streamAndProcessLLMResponse(prepped.Context); err != nil {
			s.handleExecutionError(s.ctx, err)
			return err
		}

		if err := s.validateAgentOutput(); err != nil {
			s.handleExecutionError(s.ctx, err)
			return err
		}

		if err := s.saveAgentMessageAndEmitEvents(); err != nil {
			s.handleExecutionError(s.ctx, err)
			return err
		}

		if s.streamResult != nil {
			s.observer.TrackCompleted(s.messageID, s.streamResult.Content, s.streamResult.ToolCalls)
		}

		// 5. Handle Tools (equivalent to RoleTool in blades)
		if s.streamResult != nil && s.streamResult.HasToolCalls() {
			// Transition to ExecutingTools
			s.trackStateChange(StateExecutingTools)

			// Emit ActionsDetected event
			if !s.observer.yield(s.agent.Events.ActionsDetected(s.threadID, s.messageID, s.streamResult.ToolCalls), nil) {
				return errStopIteration
			}

			// Execute tools
			if err := s.executeRequiredActions(); err != nil {
				s.handleExecutionError(s.ctx, err)
				return err
			}

			// If executeRequiredActions set a pause state, return
			if s.pauseReason != PauseReasonNone {
				return nil
			}

			// Continue to next iteration (back to StateInvokingLLM)
			s.trackStateChange(StateInvokingLLM)
			continue
		}

		// No tools or terminated - Success
		s.completionReason = CompletionReasonSuccess
		s.observer.result.Reason = TerminationGoalReached
		s.trackStateChange(StateCompleted)
		s.emitCompletionEvents()
		return nil
	}

	// Exceeded maximum iterations
	err := fmt.Errorf("agent exceeded maximum iterations (%d)", s.safety.maxAgent)
	s.observer.Fail(TerminationMaxIterations, err)
	s.handleExecutionError(s.ctx, err)
	return err
}

func (s *executionState) streamAndProcessLLMResponse(llmContext *LLMContext) error {
	// Wrap LLM call with ChatMiddleware
	if len(s.agent.Config.ChatMiddleware) > 0 {
		// Extract provider/model and parameters from the LLM request for middleware logging
		chatMeta := map[string]any{
			"agent_name":    s.agent.Name(),
			"thread_id":     s.threadID,
			"message_id":    s.messageID,
			"message_count": len(llmContext.Request.Messages),
			"provider":      llmContext.Request.Extensions.GetString("provider"),
			"model":         llmContext.Request.Extensions.GetString("model"),
		}
		if llmContext.Request.Parameters != nil {
			chatMeta["max_tokens"] = llmContext.Request.Parameters.MaxTokens
			if llmContext.Request.Parameters.Temperature != nil {
				chatMeta["temperature"] = *llmContext.Request.Parameters.Temperature
			}
		}

		return ExecuteChatMiddleware(
			s.ctx,
			s.agent.Config.ChatMiddleware,
			nil, // agent.Model (engine uses llm.Client, not agent.Model)
			nil, // []agent.Message (engine uses llm.Message)
			nil, // *agent.ChatOptions (parameters passed via metadata)
			chatMeta,
			func() error {
				return s.streamAndProcessLLMResponseCore(llmContext)
			},
		)
	}
	return s.streamAndProcessLLMResponseCore(llmContext)
}

func (s *executionState) streamAndProcessLLMResponseCore(llmContext *LLMContext) error {
	// Update executor with middleware-processed effective tools
	// This replaces the base executor created in prepareExecutor()
	// with one that has the full context and middleware-applied tools
	s.agent.Executor = s.engine.toolExec.CreateExecutor(
		s.ctx, llmContext.GetEffectiveTools(),
		s.threadID, s.messageID, llmContext.RequestCtx,
	)

	req := llmContext.Request

	streamIter := s.engine.chatClient.Generate(s.ctx, req)

	res, err := s.agent.Stream.Process(s.ctx, streamIter, ProcessParams{
		ThreadID:  s.threadID,
		MessageID: s.messageID,
		Provider:  req.Extensions.GetString("provider"),
		Model:     req.Extensions.GetString("model"),
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
	// Yield AssistantMessageAdded event (durable)
	assistantMsgEvent := &core.AssistantMessageAdded{
		ThreadID:  s.threadID,
		MessageID: s.messageID,
		Content:   s.streamResult.Content,
		ToolCalls: s.streamResult.ToolCalls,
		AgentName: s.agent.Name(),
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
		AgentName: s.agent.Name(),
		Timestamp: assistantMsgEvent.Timestamp,
	})

	return nil
}

func (s *executionState) trackStateChange(newState state) {
	if !s.observer.yield(core.EngineStateChanged{
		ThreadID:  s.threadID,
		State:     newState.String(),
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
	oldState core.ToolState,
	newState core.ToolState,
	result *core.ToolExecutionResult,
	elicitation *core.ElicitationSpec,
) {
	// Emit specific events based on state transitions
	switch newState {
	case core.ToolStateExecuting:
		// Emit ToolStart when entering executing state from idle
		if oldState == core.ToolStateIdle {
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

	case core.ToolStateAwaitingInput:
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

	case core.ToolStateAwaitingAuth:
		// ToolNeedsAuth will be emitted when authorization spec is available
		// For now handled by the tool state machine observer pattern

	case core.ToolStateCompleted:
		// ToolResultAdded is already emitted by ToolProcessor.ProcessActions

	case core.ToolStateError:
		// ToolResultAdded (with isError=true) is already emitted by ToolProcessor.ProcessActions
	}
}

// Verify ToolStateEventEmitter implements ToolStateObserver.
var _ core.ToolStateObserver = (*ToolStateEventEmitter)(nil)
