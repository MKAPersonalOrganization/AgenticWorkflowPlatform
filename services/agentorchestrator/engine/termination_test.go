package engine

import (
	"errors"
	"testing"
	"time"
)

func TestExecutionResult_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		result   ExecutionResult
		contains []string
	}{
		{
			name: "successful execution",
			result: ExecutionResult{
				Success:    true,
				Reason:     TerminationGoalReached,
				FinalAgent: "math-instructor",
				Iterations: 5,
				Duration:   time.Second * 10,
			},
			contains: []string{"succeeded", "goal_reached", "math-instructor", "5"},
		},
		{
			name: "max iterations",
			result: ExecutionResult{
				Success:    false,
				Reason:     TerminationMaxIterations,
				FinalAgent: "physics-instructor",
				Iterations: 20,
				Duration:   time.Minute,
			},
			contains: []string{"failed", "max_iterations", "physics-instructor", "20"},
		},
		{
			name: "loop detected",
			result: ExecutionResult{
				Success:    false,
				Reason:     TerminationLoopDetected,
				FinalAgent: "chemistry-instructor",
				Iterations: 8,
				Duration:   time.Second * 30,
				LoopInfo: &LoopInfo{
					Type:        LoopTypeToolCall,
					Occurrences: 5,
					Pattern:     "calculate",
				},
			},
			contains: []string{"failed", "loop_detected", "tool_call_loop", "5"},
		},
		{
			name: "error",
			result: ExecutionResult{
				Success:    false,
				Reason:     TerminationError,
				FinalAgent: "student-facing-agent",
				Iterations: 1,
				Duration:   time.Millisecond * 500,
				Error:      errors.New("connection timeout"),
			},
			contains: []string{"failed", "error", "connection timeout"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			str := tt.result.String()
			for _, substr := range tt.contains {
				if !containsString(str, substr) {
					t.Errorf("Expected string to contain %q, got: %s", substr, str)
				}
			}
		})
	}
}

func TestExecutionResult_IsRecoverable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		reason      TerminationReason
		recoverable bool
	}{
		{TerminationGoalReached, false},
		{TerminationMaxIterations, false},
		{TerminationTimeout, false},
		{TerminationUserAborted, false},
		{TerminationLoopDetected, false},
		{TerminationError, false},
		{TerminationHandoff, true},     // Recoverable
		{TerminationElicitation, true}, // Recoverable
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.reason), func(t *testing.T) {
			t.Parallel()
			result := ExecutionResult{Reason: tt.reason}
			if result.IsRecoverable() != tt.recoverable {
				t.Errorf("Expected IsRecoverable()=%v for reason %s", tt.recoverable, tt.reason)
			}
		})
	}
}

func TestLoopInfo(t *testing.T) {
	t.Parallel()
	info := LoopInfo{
		Type:        LoopTypeToolCall,
		Description: "Agent is repeatedly calling the same tool",
		Occurrences: 5,
		Pattern:     "calculate",
	}

	if info.Type != LoopTypeToolCall {
		t.Errorf("Expected type %s, got %s", LoopTypeToolCall, info.Type)
	}
	if info.Occurrences != 5 {
		t.Errorf("Expected 5 occurrences, got %d", info.Occurrences)
	}
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
