package evaluation

import "context"

// AgentInvoker invokes an agent with a single input for evaluation purposes.
// Implemented in adapters/adkgo/eval_runner.go.
type AgentInvoker interface {
	InvokeForEval(ctx context.Context, req EvalInvokeRequest) (*EvalInvokeResult, error)
}

// EvalInvokeRequest is the input for agent invocation during evaluation.
type EvalInvokeRequest struct {
	AgentID string
	UserID  string
	Input   string
}

// EvalInvokeResult contains the output from a single agent invocation.
type EvalInvokeResult struct {
	FinalResponse string
	ToolCalls     []EvalToolCall
	JudgeAPIKey   string // Gemini API key reusable for LLM judge
	JudgeModel    string // e.g. "gemini-2.5-flash"
}

// EvalToolCall represents a single tool call made by the agent.
type EvalToolCall struct {
	Name string
	Args map[string]any
}
