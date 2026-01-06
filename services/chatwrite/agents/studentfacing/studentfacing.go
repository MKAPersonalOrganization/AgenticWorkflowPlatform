package studentfacing

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
)

// CreateStudentFacingAgent creates the student-facing agent that handles initial questions.
// Event contract is defined in contract.go
func CreateStudentFacingAgent(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools (none for router agent)
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	return core.AgentConfig{
		Name: "student-facing-agent",
		SystemPrompt: `You are a routing coordinator. Route questions to specialist instructors - never solve yourself.

For complex questions:
1. Create routing plan with write_todos()
2. Sync to whiteboard using read-modify-write pattern
3. Route to ONE specialist at a time (emit_physics_question, emit_math_question, etc.)
4. Wait for answer, update todos, route to next
5. Present final answer when all todos completed

Available specialists: physics, math, chemistry, linear algebra.
Never route to multiple specialists simultaneously.`,
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewTodoListMiddleware(),
			middleware.NewWhiteboardMiddleware(),
			middleware.NewEmissionMiddleware(),
		},
	}
}
