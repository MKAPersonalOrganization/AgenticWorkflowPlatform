package engine

import (
	"testing"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
)

// TestHistoryAnalyzer_EmptyHistory tests analysis of empty conversation history.
func TestHistoryAnalyzer_EmptyHistory(t *testing.T) {
	analyzer := &HistoryAnalyzer{history: []core.Message{}}
	analysis := analyzer.Analyze()

	if analysis.LastAssistantWithTools != nil {
		t.Error("Expected nil LastAssistantWithTools for empty history")
	}
	if analysis.PendingToolCall != nil {
		t.Error("Expected nil PendingToolCall for empty history")
	}
	if analysis.LastMessage != nil {
		t.Error("Expected nil LastMessage for empty history")
	}
	if len(analysis.PendingToolCalls) != 0 {
		t.Error("Expected empty PendingToolCalls for empty history")
	}
}

// TestHistoryAnalyzer_NoToolCalls tests analysis when there are no tool calls.
func TestHistoryAnalyzer_NoToolCalls(t *testing.T) {
	history := []core.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}
	analyzer := &HistoryAnalyzer{history: history}
	analysis := analyzer.Analyze()

	if analysis.LastAssistantWithTools != nil {
		t.Error("Expected nil LastAssistantWithTools when no tools are called")
	}
	if analysis.PendingToolCall != nil {
		t.Error("Expected nil PendingToolCall when no tools are called")
	}
	if analysis.LastMessage == nil || analysis.LastMessage.Content != "Hi there!" {
		t.Error("Expected LastMessage to be the assistant response")
	}
}

// TestHistoryAnalyzer_PendingToolCalls tests detection of pending tool calls.
func TestHistoryAnalyzer_PendingToolCalls(t *testing.T) {
	history := []core.Message{
		{Role: "user", Content: "Calculate 2+2"},
		{
			Role:    "assistant",
			Content: "",
			ID:      "msg-123",
			ToolCalls: []core.ToolCall{
				{ID: "call-1", Function: core.ToolFunction{Name: "calculator", Arguments: `{"expression":"2+2"}`}},
				{ID: "call-2", Function: core.ToolFunction{Name: "calculator", Arguments: `{"expression":"3+3"}`}},
			},
		},
	}
	analyzer := &HistoryAnalyzer{history: history}
	analysis := analyzer.Analyze()

	if analysis.LastAssistantWithTools == nil {
		t.Fatal("Expected LastAssistantWithTools to be set")
	}
	if len(analysis.PendingToolCalls) != 2 {
		t.Errorf("Expected 2 pending tool calls, got %d", len(analysis.PendingToolCalls))
	}
	if analysis.PendingToolCall == nil || analysis.PendingToolCall.ID != "call-1" {
		t.Error("Expected first pending tool call to be call-1")
	}
}

// TestHistoryAnalyzer_PartialToolExecution tests when some tools are completed.
func TestHistoryAnalyzer_PartialToolExecution(t *testing.T) {
	history := []core.Message{
		{Role: "user", Content: "Calculate 2+2 and 3+3"},
		{
			Role:    "assistant",
			Content: "",
			ID:      "msg-123",
			ToolCalls: []core.ToolCall{
				{ID: "call-1", Function: core.ToolFunction{Name: "calculator", Arguments: `{"expression":"2+2"}`}},
				{ID: "call-2", Function: core.ToolFunction{Name: "calculator", Arguments: `{"expression":"3+3"}`}},
			},
		},
		{Role: "tool", ToolCallID: "call-1", Content: "4"},
	}
	analyzer := &HistoryAnalyzer{history: history}
	analysis := analyzer.Analyze()

	if len(analysis.PendingToolCalls) != 1 {
		t.Errorf("Expected 1 pending tool call, got %d", len(analysis.PendingToolCalls))
	}
	if analysis.PendingToolCall == nil || analysis.PendingToolCall.ID != "call-2" {
		t.Error("Expected pending tool call to be call-2")
	}
	if !analysis.CompletedToolCalls["call-1"] {
		t.Error("Expected call-1 to be marked as completed")
	}
	if analysis.CompletedToolCalls["call-2"] {
		t.Error("Expected call-2 to NOT be marked as completed")
	}
}

// TestHistoryAnalyzer_AllToolsCompleted tests when all tools are completed.
func TestHistoryAnalyzer_AllToolsCompleted(t *testing.T) {
	history := []core.Message{
		{Role: "user", Content: "Calculate 2+2"},
		{
			Role:    "assistant",
			Content: "",
			ID:      "msg-123",
			ToolCalls: []core.ToolCall{
				{ID: "call-1", Function: core.ToolFunction{Name: "calculator", Arguments: `{"expression":"2+2"}`}},
			},
		},
		{Role: "tool", ToolCallID: "call-1", Content: "4"},
	}
	analyzer := &HistoryAnalyzer{history: history}
	analysis := analyzer.Analyze()

	if len(analysis.PendingToolCalls) != 0 {
		t.Errorf("Expected 0 pending tool calls, got %d", len(analysis.PendingToolCalls))
	}
	if analysis.PendingToolCall != nil {
		t.Error("Expected nil PendingToolCall when all tools are completed")
	}
}

// TestStateResolver_NoAssistantWithTools tests state resolution with no tools.
func TestStateResolver_NoAssistantWithTools(t *testing.T) {
	resolver := &StateResolver{}
	analysis := &HistoryAnalysis{}

	state := resolver.ResolveState(analysis)

	if state != StateAwaitingTurn {
		t.Errorf("Expected StateAwaitingTurn, got %v", state)
	}
}

// TestStateResolver_NoPendingTools tests state resolution when all tools completed.
func TestStateResolver_NoPendingTools(t *testing.T) {
	resolver := &StateResolver{}
	analysis := &HistoryAnalysis{
		LastAssistantWithTools: &core.Message{ID: "msg-123"},
		PendingToolCall:        nil,
	}

	state := resolver.ResolveState(analysis)

	if state != StateAwaitingTurn {
		t.Errorf("Expected StateAwaitingTurn, got %v", state)
	}
}

// TestStateResolver_NormalToolExecution tests normal tool execution path.
func TestStateResolver_NormalToolExecution(t *testing.T) {
	resolver := &StateResolver{}
	analysis := &HistoryAnalysis{
		LastAssistantWithTools: &core.Message{
			ID: "msg-123",
			ToolCalls: []core.ToolCall{
				{ID: "call-1", Function: core.ToolFunction{Name: "calculator"}},
			},
		},
		PendingToolCall: &core.ToolCall{ID: "call-1", Function: core.ToolFunction{Name: "calculator"}},
	}

	state := resolver.ResolveState(analysis)

	if state != StatePerformingActions {
		t.Errorf("Expected StatePerformingActions, got %v", state)
	}
}

// TestStateResolver_AwaitingInput tests elicitation awaiting user input.
func TestStateResolver_AwaitingInput(t *testing.T) {
	resolver := &StateResolver{}
	analysis := &HistoryAnalysis{
		LastAssistantWithTools: &core.Message{
			ID: "msg-123",
			ToolCalls: []core.ToolCall{
				{ID: "call-1", Function: core.ToolFunction{Name: "elicit"}},
			},
			Metadata: map[string]any{
				"tool_states": map[string]domain.ToolCallState{
					"call-1": {Status: "AwaitingInput"},
				},
			},
		},
		PendingToolCall: &core.ToolCall{ID: "call-1", Function: core.ToolFunction{Name: "elicit"}},
		LastMessage:     &core.Message{Role: "assistant"},
	}

	state := resolver.ResolveState(analysis)

	// Still waiting for user input
	if state != StateDone {
		t.Errorf("Expected StateDone (paused), got %v", state)
	}
}

// TestStateResolver_AwaitingInputWithUserResponse tests elicitation with user response.
func TestStateResolver_AwaitingInputWithUserResponse(t *testing.T) {
	resolver := &StateResolver{}
	analysis := &HistoryAnalysis{
		LastAssistantWithTools: &core.Message{
			ID: "msg-123",
			ToolCalls: []core.ToolCall{
				{ID: "call-1", Function: core.ToolFunction{Name: "elicit"}},
			},
			Metadata: map[string]any{
				"tool_states": map[string]domain.ToolCallState{
					"call-1": {Status: "AwaitingInput"},
				},
			},
		},
		PendingToolCall: &core.ToolCall{ID: "call-1", Function: core.ToolFunction{Name: "elicit"}},
		LastMessage:     &core.Message{Role: "user", Content: "Yes, proceed"},
	}

	state := resolver.ResolveState(analysis)

	// User provided input, can proceed to generate response
	if state != StateGeneratingResponse {
		t.Errorf("Expected StateGeneratingResponse, got %v", state)
	}
}

// TestStateResolver_AwaitingAuth tests authorization flow.
func TestStateResolver_AwaitingAuth(t *testing.T) {
	resolver := &StateResolver{}
	analysis := &HistoryAnalysis{
		LastAssistantWithTools: &core.Message{
			ID: "msg-123",
			ToolCalls: []core.ToolCall{
				{ID: "call-1", Function: core.ToolFunction{Name: "dangerous_tool"}},
			},
			Metadata: map[string]any{
				"tool_states": map[string]domain.ToolCallState{
					"call-1": {Status: "AwaitingAuth"},
				},
			},
		},
		PendingToolCall: &core.ToolCall{ID: "call-1", Function: core.ToolFunction{Name: "dangerous_tool"}},
		LastMessage:     &core.Message{Role: "assistant"},
	}

	state := resolver.ResolveState(analysis)

	// Still waiting for authorization
	if state != StateDone {
		t.Errorf("Expected StateDone (paused), got %v", state)
	}
}

// TestResumptionPlanner_NoResumptionNeeded tests when no plan is needed.
func TestResumptionPlanner_NoResumptionNeeded(t *testing.T) {
	planner := &ResumptionPlanner{}
	analysis := &HistoryAnalysis{
		LastAssistantWithTools: &core.Message{ID: "msg-123"},
		PendingToolCall:        &core.ToolCall{ID: "call-1"},
	}

	// Plan only created for StateGeneratingResponse
	plan := planner.CreatePlan(analysis, StatePerformingActions)

	if plan != nil {
		t.Error("Expected nil plan for StatePerformingActions")
	}
}

// TestResumptionPlanner_CreatePlanForUserInput tests plan creation.
func TestResumptionPlanner_CreatePlanForUserInput(t *testing.T) {
	planner := &ResumptionPlanner{}
	analysis := &HistoryAnalysis{
		LastAssistantWithTools: &core.Message{ID: "msg-123"},
		PendingToolCall:        &core.ToolCall{ID: "call-1"},
		LastMessage:            &core.Message{Role: "user", Content: "Yes, proceed"},
	}

	plan := planner.CreatePlan(analysis, StateGeneratingResponse)

	if plan == nil {
		t.Fatal("Expected plan to be created")
	}
	if plan.ToolCallID != "call-1" {
		t.Errorf("Expected ToolCallID call-1, got %s", plan.ToolCallID)
	}
	if plan.MessageID != "msg-123" {
		t.Errorf("Expected MessageID msg-123, got %s", plan.MessageID)
	}
	if plan.Content != "Yes, proceed" {
		t.Errorf("Expected Content 'Yes, proceed', got %s", plan.Content)
	}
}

// TestResumptionPlanner_NoUserInput tests no plan when no user input.
func TestResumptionPlanner_NoUserInput(t *testing.T) {
	planner := &ResumptionPlanner{}
	analysis := &HistoryAnalysis{
		LastAssistantWithTools: &core.Message{ID: "msg-123"},
		PendingToolCall:        &core.ToolCall{ID: "call-1"},
		LastMessage:            &core.Message{Role: "assistant", Content: "Waiting..."},
	}

	plan := planner.CreatePlan(analysis, StateGeneratingResponse)

	if plan != nil {
		t.Error("Expected nil plan when last message is not user input")
	}
}

// TestResumptionPlanner_CreateStreamResult tests stream result creation.
func TestResumptionPlanner_CreateStreamResult(t *testing.T) {
	planner := &ResumptionPlanner{}
	analysis := &HistoryAnalysis{
		LastAssistantWithTools: &core.Message{
			ID:      "msg-123",
			Content: "Let me calculate that",
		},
		PendingToolCalls: []core.ToolCall{
			{ID: "call-1", Function: core.ToolFunction{Name: "calculator"}},
			{ID: "call-2", Function: core.ToolFunction{Name: "calculator"}},
		},
	}

	result := planner.CreateStreamResult(analysis, StatePerformingActions)

	if result == nil {
		t.Fatal("Expected stream result to be created")
	}
	if result.Content != "Let me calculate that" {
		t.Errorf("Expected content 'Let me calculate that', got %s", result.Content)
	}
	if len(result.ToolCalls) != 2 {
		t.Errorf("Expected 2 tool calls, got %d", len(result.ToolCalls))
	}
}

// TestResumptionPlanner_NoStreamResultForWrongState tests no result for wrong state.
func TestResumptionPlanner_NoStreamResultForWrongState(t *testing.T) {
	planner := &ResumptionPlanner{}
	analysis := &HistoryAnalysis{
		LastAssistantWithTools: &core.Message{ID: "msg-123"},
		PendingToolCalls:       []core.ToolCall{{ID: "call-1"}},
	}

	result := planner.CreateStreamResult(analysis, StateGeneratingResponse)

	if result != nil {
		t.Error("Expected nil stream result for StateGeneratingResponse")
	}
}

// TestIntegration_NormalToolExecution tests the full flow for normal tool execution.
func TestIntegration_NormalToolExecution(t *testing.T) {
	history := []core.Message{
		{Role: "user", Content: "Calculate 2+2"},
		{
			Role:    "assistant",
			Content: "",
			ID:      "msg-123",
			ToolCalls: []core.ToolCall{
				{ID: "call-1", Function: core.ToolFunction{Name: "calculator", Arguments: `{"expression":"2+2"}`}},
			},
		},
	}

	// Use the new components
	analyzer := &HistoryAnalyzer{history: history}
	resolver := &StateResolver{}
	planner := &ResumptionPlanner{}

	analysis := analyzer.Analyze()
	state := resolver.ResolveState(analysis)
	streamResult := planner.CreateStreamResult(analysis, state)
	plan := planner.CreatePlan(analysis, state)

	if state != StatePerformingActions {
		t.Errorf("Expected StatePerformingActions, got %v", state)
	}
	if streamResult == nil {
		t.Fatal("Expected stream result")
	}
	if len(streamResult.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(streamResult.ToolCalls))
	}
	if plan != nil {
		t.Error("Expected nil plan for normal tool execution")
	}
}

// TestIntegration_ElicitationFlow tests the full flow for elicitation resumption.
func TestIntegration_ElicitationFlow(t *testing.T) {
	history := []core.Message{
		{Role: "user", Content: "Do something dangerous"},
		{
			Role:    "assistant",
			Content: "",
			ID:      "msg-123",
			ToolCalls: []core.ToolCall{
				{ID: "call-1", Function: core.ToolFunction{Name: "elicit"}},
			},
			Metadata: map[string]any{
				"tool_states": map[string]domain.ToolCallState{
					"call-1": {Status: "AwaitingInput"},
				},
			},
		},
		{Role: "user", Content: "Yes, proceed"},
	}

	// Use the new components
	analyzer := &HistoryAnalyzer{history: history}
	resolver := &StateResolver{}
	planner := &ResumptionPlanner{}

	analysis := analyzer.Analyze()
	state := resolver.ResolveState(analysis)
	streamResult := planner.CreateStreamResult(analysis, state)
	plan := planner.CreatePlan(analysis, state)

	if state != StateGeneratingResponse {
		t.Errorf("Expected StateGeneratingResponse, got %v", state)
	}
	if streamResult != nil {
		t.Error("Expected nil stream result for elicitation resumption")
	}
	if plan == nil {
		t.Fatal("Expected resumption plan")
	}
	if plan.ToolCallID != "call-1" {
		t.Errorf("Expected ToolCallID call-1, got %s", plan.ToolCallID)
	}
	if plan.Content != "Yes, proceed" {
		t.Errorf("Expected Content 'Yes, proceed', got %s", plan.Content)
	}
}
