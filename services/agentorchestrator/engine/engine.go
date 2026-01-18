package engine

import (
	"context"
	"fmt"
	"iter"
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

func (s *executionState) initAgentComponents(agentName string) error {
	config, err := s.engine.provider.GetConfig(agentName)
	if err != nil {
		return err
	}

	if err := config.Validate(); err != nil {
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
	s.maxTotalLimit = 20 // Safety cap to prevent cross-agent ping-pong
	s.iteration = 0      // Reset iteration counter for new agent

	return nil
}

func (s *executionState) setup(initialState state) error {
	// Configure State Machine
	s.sm = stateless.NewStateMachine(initialState)

	s.sm.Configure(StateInitial).
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
		Permit(TriggerHumanInputRequired, StateElicitingInput)

	s.sm.Configure(StateTransferring).
		OnEntry(func(_ context.Context, args ...any) error {
			targetAgent := args[0].(string)
			return s.handleSwitch(targetAgent)
		}).
		Permit(TriggerTransferComplete, StateDeterminingNextStep)

	s.sm.Configure(StateElicitingInput).
		PermitReentry(TriggerBegin) // Self-transition if run starts here

	return nil
}

func (s *executionState) run() error {
	return s.sm.Fire(TriggerBegin)
}

func (s *executionState) inferState() state {
	if len(s.history) == 0 {
		return StateInitial
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
					// Refresh state and transition to determining next step
					s.thread = s.engine.threadMgmt.ToAgentThread(agg)
					s.history = s.engine.threadMgmt.ToCoreMessages(agg)
					return StateDeterminingNextStep
				}
			}

			if lastMsg.Role == "tool" && lastMsg.ToolCallID == tc.ID {
				// Tool result exists, so we can determine next step
				return StateDeterminingNextStep
			}
			// Still waiting for user input
			return StateElicitingInput
		}
	}

	// Populate streamResult for traditional tool execution
	s.streamResult = &StreamResult{
		Content:   lastAssistantWithTools.Content,
		ToolCalls: lastAssistantWithTools.ToolCalls,
	}
	return StateExecutingActions
}

func (s *executionState) handleStream() error {
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
	if !s.emit(s.events.AssistantStarted(s.threadID, s.messageID)) {
		return context.Canceled
	}
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

	// Check for content repetition loop
	if res.Content != "" {
		if loopInfo := s.loopDetector.RecordContent(res.Content); loopInfo != nil {
			s.engine.log.Warn(s.ctx, "Content repetition loop detected",
				"agent", s.agentName,
				"occurrences", loopInfo.Occurrences,
			)
			s.result.Reason = TerminationLoopDetected
			s.result.LoopInfo = loopInfo
			s.result.Success = false
			return fmt.Errorf("content loop detected: %s (occurrences: %d)",
				loopInfo.Description, loopInfo.Occurrences)
		}
	}

	// Persist and emit completed
	toolCalls := toDomainToolCalls(s.streamResult.ToolCalls)
	agg, err := s.engine.threadMgmt.AddAssistantMessage(s.ctx, s.threadID, s.messageID, s.streamResult.Content, toolCalls, s.agentName)
	if err != nil {
		return err
	}
	s.thread = s.engine.threadMgmt.ToAgentThread(agg)
	s.history = s.engine.threadMgmt.ToCoreMessages(agg)

	if !s.emit(s.events.AssistantCompleted(s.threadID, s.messageID, s.streamResult.Content, toolCalls)) {
		return context.Canceled
	}

	if s.streamResult.HasToolCalls() {
		return s.sm.Fire(TriggerActionRequired)
	}
	return s.sm.Fire(TriggerStepComplete)
}

func (s *executionState) handleTools() error {
	// Collect any transition triggers to fire after all tools are processed
	var handoffTarget string
	var elicitationSpec *core.ElicitationSpec
	var elicitationToolID string

	// Process all tool calls sequentially (OpenAI requires results for all tool_call_ids)
	for _, tc := range s.streamResult.ToolCalls {
		// Track tool call for statistics
		s.totalToolCalls = append(s.totalToolCalls, tc.Function.Name)

		// Check for tool call loop BEFORE execution
		if loopInfo := s.loopDetector.RecordToolCall(tc); loopInfo != nil {
			s.engine.log.Warn(s.ctx, "Tool call loop detected",
				"agent", s.agentName,
				"tool", tc.Function.Name,
				"occurrences", loopInfo.Occurrences,
			)
			s.result.Reason = TerminationLoopDetected
			s.result.LoopInfo = loopInfo
			s.result.Success = false
			return fmt.Errorf("loop detected: %s (tool: %s, occurrences: %d)",
				loopInfo.Description, loopInfo.Pattern, loopInfo.Occurrences)
		}

		if !s.emit(s.events.ToolCallCompleted(s.threadID, tc.ID)) {
			return context.Canceled
		}

		result := s.engine.toolExec.Execute(tc, s.executor)

		// Check for tool error loop AFTER execution
		if loopInfo := s.loopDetector.RecordToolResult(result.ToolName, result.Result, result.IsError); loopInfo != nil {
			s.engine.log.Warn(s.ctx, "Tool error loop detected",
				"agent", s.agentName,
				"tool", result.ToolName,
				"occurrences", loopInfo.Occurrences,
				"pattern", loopInfo.Pattern,
			)
			s.result.Reason = TerminationLoopDetected
			s.result.LoopInfo = loopInfo
			s.result.Success = false
			return fmt.Errorf("tool error loop detected: %s (tool: %s, occurrences: %d)",
				loopInfo.Description, result.ToolName, loopInfo.Occurrences)
		}

		if result.ToolCallID == "" {
			result.ToolCallID = tc.ID // Fallback
		}

		toolResultID := core.GenerateID()
		agg, err := s.engine.threadMgmt.AddToolResultMessage(s.ctx, s.threadID, toolResultID, result.ToolCallID, result.Result, s.agentName)
		if err != nil {
			return err
		}
		s.thread = s.engine.threadMgmt.ToAgentThread(agg)
		s.history = s.engine.threadMgmt.ToCoreMessages(agg)

		if !s.emit(s.events.ToolCallExecuted(
			s.threadID, toolResultID, result.ToolCallID,
			result.ToolName, result.Arguments, result.Result, s.messageID,
		)) {
			return context.Canceled
		}

		// Check for routing command (handoff or elicitation)
		if result.Command != nil {
			if result.Command.Goto != "" {
				handoffTarget = result.Command.Goto
			}

			// Check for elicitation
			if result.Command.Elicitation != nil {
				elicitationSpec = result.Command.Elicitation
				elicitationToolID = tc.ID
			}
		}
	}

	// Now handle state transitions after ALL tool results have been recorded
	if elicitationSpec != nil {
		s.result.Reason = TerminationElicitation
		if !s.emit(s.events.HumanInputRequested(s.threadID, s.messageID, elicitationToolID, elicitationSpec.Question, elicitationSpec.Schema)) {
			return context.Canceled
		}
		return s.sm.Fire(TriggerHumanInputRequired)
	}

	if handoffTarget != "" {
		return s.sm.Fire(TriggerHandoffSwitch, handoffTarget)
	}

	s.messageID = core.GenerateID()
	return s.sm.Fire(TriggerActionDone)
}

func (s *executionState) handleSwitch(targetAgent string) error {
	// Record handoff in result
	s.result.Reason = TerminationHandoff
	s.result.HandoffTarget = targetAgent

	if !s.emit(s.events.AgentHandoff(s.threadID, targetAgent)) {
		return context.Canceled
	}

	if err := s.initAgentComponents(targetAgent); err != nil {
		return err
	}

	// Reset loop detector for new agent
	s.loopDetector.Reset()

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
	StateElicitingInput
	StateDone
	StateError
)

const (
	TriggerBegin              = "Begin"
	TriggerActionRequired     = "ActionRequired"
	TriggerStepComplete       = "StepComplete"
	TriggerActionDone         = "ActionDone"
	TriggerHandoffSwitch      = "HandoffSwitch"
	TriggerHumanInputRequired = "HumanInputRequired"
	TriggerTransferComplete   = "TransferComplete"
)
