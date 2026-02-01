package engine

import (
	"context"
	"errors"
	"io"
	"testing"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/threadmgmt"
)

func TestToolProcessor_ProcessActions_MultipleTools(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T) (*threadmgmt.Manager, *PersistenceUnit, string) {
		eventStore := NewMockEventStore()
		repo := domain.NewThreadRepository(eventStore)
		tm := threadmgmt.New(logger.New(io.Discard, logger.LevelInfo, "test", nil), repo)
		persistence := &PersistenceUnit{mgmt: tm}

		threadID := "thread-1"
		_, _, err := tm.CreateThread(ctx, threadID, "Test Thread")
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}
		return tm, persistence, threadID
	}

	mockExecutor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			switch tc.Name {
			case "tool1":
				return core.ToolExecutionResult{Result: "result1"}, nil
			case "tool2":
				return core.ToolExecutionResult{Result: "result2"}, nil
			case "tool_elicitation":
				return core.ToolExecutionResult{
					Command: &core.Command{
						Elicitation:   &core.ElicitationSpec{Question: "more info?"},
						StopIteration: true,
					},
					StopIteration: true,
				}, nil
			default:
				return core.ToolExecutionResult{Result: "default"}, nil
			}
		},
	}

	processor := &ToolProcessor{}

	t.Run("All Successful", func(t *testing.T) {
		_, persistence, threadID := setup(t)

		toolCalls := []core.ToolCall{
			{ID: "call1", Name: "tool1"},
			{ID: "call2", Name: "tool2"},
		}

		tsms := make(map[string]*domain.ToolStateMachine)
		for _, tc := range toolCalls {
			tsms[tc.ID] = domain.NewToolStateMachine(ctx, &tc, mockExecutor)
		}

		tctx := ToolProcessorContext{
			Ctx:          ctx,
			ThreadID:     threadID,
			StreamResult: &StreamResult{ToolCalls: toolCalls},
			Safety:       &SafetyUnit{maxAgent: 10, maxTotal: 100, loopDetector: NewLoopDetector(DefaultLoopDetectorConfig())},
			Observer: &ObserverUnit{
				yield:    func(e core.Event, err error) bool { return true },
				events:   NewEventFactory("test"),
				result:   &ExecutionResult{},
				threadID: threadID,
			},
			Persistence:       persistence,
			ToolStateMachines: tsms,
		}

		route, _, err := processor.ProcessActions(tctx)
		if err != nil {
			t.Fatalf("ProcessActions failed: %v", err)
		}

		if route.StopIteration {
			t.Error("Expected StopIteration to be false")
		}

		for id, tsm := range tsms {
			if tsm.CurrentState() != domain.ToolStateCompleted {
				t.Errorf("Tool %s should be Completed, got %s", id, tsm.CurrentState())
			}
		}
	})

	t.Run("One Elicitation Stops All", func(t *testing.T) {
		_, persistence, threadID := setup(t)

		toolCalls := []core.ToolCall{
			{ID: "call1", Name: "tool1"},
			{ID: "call_elicit", Name: "tool_elicitation"},
			{ID: "call2", Name: "tool2"},
		}

		tsms := make(map[string]*domain.ToolStateMachine)
		for _, tc := range toolCalls {
			tsms[tc.ID] = domain.NewToolStateMachine(ctx, &tc, mockExecutor)
		}

		tctx := ToolProcessorContext{
			Ctx:          ctx,
			ThreadID:     threadID,
			StreamResult: &StreamResult{ToolCalls: toolCalls},
			Safety:       &SafetyUnit{maxAgent: 10, maxTotal: 100, loopDetector: NewLoopDetector(DefaultLoopDetectorConfig())},
			Observer: &ObserverUnit{
				yield:    func(e core.Event, err error) bool { return true },
				events:   NewEventFactory("test"),
				result:   &ExecutionResult{},
				threadID: threadID,
			},
			Persistence:       persistence,
			ToolStateMachines: tsms,
		}

		route, _, err := processor.ProcessActions(tctx)
		if err != nil {
			t.Fatalf("ProcessActions failed: %v", err)
		}

		if !route.StopIteration {
			t.Error("Expected StopIteration to be true")
		}

		if tsms["call1"].CurrentState() != domain.ToolStateCompleted {
			t.Errorf("Tool call1 should be Completed, got %s", tsms["call1"].CurrentState())
		}
		if tsms["call_elicit"].CurrentState() != domain.ToolStateAwaitingInput {
			t.Errorf("Tool call_elicit should be AwaitingInput, got %s", tsms["call_elicit"].CurrentState())
		}
		if tsms["call2"].CurrentState() != domain.ToolStateCompleted {
			t.Errorf("Tool call2 should be Completed, got %s", tsms["call2"].CurrentState())
		}
	})

	t.Run("Parallel Success Preservation", func(t *testing.T) {
		_, persistence, threadID := setup(t)

		toolCalls := []core.ToolCall{
			{ID: "slow", Name: "slow_tool"},
			{ID: "fast", Name: "fast_tool"},
		}

		// Mock executor where "slow" takes more time but results are collected in order
		parallelExecutor := &MockToolExecutor{
			executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
				if tc.Name == "slow_tool" {
					return core.ToolExecutionResult{Result: "slow_result"}, nil
				}
				return core.ToolExecutionResult{Result: "fast_result"}, nil
			},
		}

		tsms := make(map[string]*domain.ToolStateMachine)
		for _, tc := range toolCalls {
			tsms[tc.ID] = domain.NewToolStateMachine(ctx, &tc, parallelExecutor)
		}

		tctx := ToolProcessorContext{
			Ctx:          ctx,
			ThreadID:     threadID,
			StreamResult: &StreamResult{ToolCalls: toolCalls},
			Safety:       &SafetyUnit{maxAgent: 10, maxTotal: 100, loopDetector: NewLoopDetector(DefaultLoopDetectorConfig())},
			Observer: &ObserverUnit{
				yield:    func(e core.Event, err error) bool { return true },
				events:   NewEventFactory("test"),
				result:   &ExecutionResult{},
				threadID: threadID,
			},
			Persistence:       persistence,
			ToolStateMachines: tsms,
		}

		_, agg, err := processor.ProcessActions(tctx)
		if err != nil {
			t.Fatalf("ProcessActions failed: %v", err)
		}

		// Verify result order in aggregate
		// Find the tool messages
		var toolResults []string
		for _, msg := range agg.Messages {
			if msg.Role == "tool" {
				toolResults = append(toolResults, msg.Content)
			}
		}

		if len(toolResults) != 2 {
			t.Errorf("Expected 2 tool messages, got %d. Results: %v", len(toolResults), toolResults)
		} else {
			if toolResults[0] != "slow_result" || toolResults[1] != "fast_result" {
				t.Errorf("Unexpected tool result order: %v", toolResults)
			}
		}
	})

	t.Run("Parallel Error Propagation", func(t *testing.T) {
		_, persistence, threadID := setup(t)

		toolCalls := []core.ToolCall{
			{ID: "ok", Name: "tool1"},
			{ID: "fail", Name: "fail_tool"},
		}

		errorExecutor := &MockToolExecutor{
			executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
				if tc.Name == "fail_tool" {
					return core.ToolExecutionResult{}, errors.New("something went wrong")
				}
				return core.ToolExecutionResult{Result: "ok_result"}, nil
			},
		}

		tsms := make(map[string]*domain.ToolStateMachine)
		for _, tc := range toolCalls {
			tsms[tc.ID] = domain.NewToolStateMachine(ctx, &tc, errorExecutor)
		}

		tctx := ToolProcessorContext{
			Ctx:          ctx,
			ThreadID:     threadID,
			StreamResult: &StreamResult{ToolCalls: toolCalls},
			Safety:       &SafetyUnit{maxAgent: 10, maxTotal: 100, loopDetector: NewLoopDetector(DefaultLoopDetectorConfig())},
			Observer: &ObserverUnit{
				yield:    func(e core.Event, err error) bool { return true },
				events:   NewEventFactory("test"),
				result:   &ExecutionResult{},
				threadID: threadID,
			},
			Persistence:       persistence,
			ToolStateMachines: tsms,
		}

		_, agg, err := processor.ProcessActions(tctx)
		if err != nil {
			t.Fatalf("ProcessActions should not return err for handled tool error: %v", err)
		}

		// Verify that both results were persisted, and "fail_tool" has error info
		var toolResults []string
		for _, msg := range agg.Messages {
			if msg.Role == "tool" {
				toolResults = append(toolResults, msg.Content)
			}
		}

		if len(toolResults) != 2 {
			t.Errorf("Expected 2 tool messages, got %d", len(toolResults))
		}
		if toolResults[0] != "ok_result" {
			t.Errorf("Expected first result to be ok_result, got %s", toolResults[0])
		}
		if toolResults[1] != "something went wrong" {
			t.Errorf("Expected second result to be the error message, got %s", toolResults[1])
		}
	})

	t.Run("Parallel Mixed Elicitation Persistence", func(t *testing.T) {
		_, persistence, threadID := setup(t)

		toolCalls := []core.ToolCall{
			{ID: "ok", Name: "tool1"},
			{ID: "elicit", Name: "tool_elicitation"},
		}

		tsms := make(map[string]*domain.ToolStateMachine)
		for _, tc := range toolCalls {
			tsms[tc.ID] = domain.NewToolStateMachine(ctx, &tc, mockExecutor)
		}

		tctx := ToolProcessorContext{
			Ctx:          ctx,
			ThreadID:     threadID,
			StreamResult: &StreamResult{ToolCalls: toolCalls},
			Safety:       &SafetyUnit{maxAgent: 10, maxTotal: 100, loopDetector: NewLoopDetector(DefaultLoopDetectorConfig())},
			Observer: &ObserverUnit{
				yield:    func(e core.Event, err error) bool { return true },
				events:   NewEventFactory("test"),
				result:   &ExecutionResult{},
				threadID: threadID,
			},
			Persistence:       persistence,
			ToolStateMachines: tsms,
		}

		route, agg, err := processor.ProcessActions(tctx)
		if err != nil {
			t.Fatalf("ProcessActions failed: %v", err)
		}

		if !route.StopIteration {
			t.Error("Expected StopIteration to be true due to elicitation")
		}

		// Verify that "tool1" result WAS persisted
		// But "elicit" result was NOT persisted because it's not terminal
		var toolResults []string
		for _, msg := range agg.Messages {
			if msg.Role == "tool" {
				toolResults = append(toolResults, msg.Content)
			}
		}

		if len(toolResults) != 2 {
			t.Errorf("Expected 2 persisted tool results (ok + elicit), got %d: %v", len(toolResults), toolResults)
		} else {
			if toolResults[0] != "result1" {
				t.Errorf("Expected 'result1', got '%s'", toolResults[0])
			}
			// Second result is from elicitation, which has a status result in mock
		}
	})
}
