package demoinput

import (
	"aicompass/foundation/agent"
	agentio "aicompass/foundation/agent/io"
	"context"
	"fmt"
)

// GreetingTool demonstrates the io.Read() custom input pattern.
// It asks the user for their name, then returns a personalized greeting.
type GreetingTool struct{}

func (t *GreetingTool) Name() string        { return "greeting" }
func (t *GreetingTool) Description() string { return "Ask the user for their name and greet them" }
func (t *GreetingTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type:       agent.TypeObject,
		Properties: map[string]*agent.Schema{},
	}
}

func (t *GreetingTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	// Ask the user for their name via io.Read().
	// On first call: PauseHandler returns ErrInputRequired → agent pauses.
	// On resume: CachedReadHandler returns the user's text → continues.
	name, err := agentio.Read(ctx, "What is your name?", map[string]any{"type": "string"})
	if err != nil {
		return agent.Result{}, err
	}

	greeting := fmt.Sprintf("Hello, %s! Nice to meet you.", name)
	return agent.JSONResult(map[string]any{
		"greeting": greeting,
		"name":     name,
	}), nil
}
