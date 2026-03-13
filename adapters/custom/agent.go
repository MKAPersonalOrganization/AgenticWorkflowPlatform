package custom

import (
	"context"
	"fmt"
	"iter"
	"time"

	"school-with-ai/foundation/agent"
)

// CustomAgent is a concrete implementation of agent.Agent that holds the configuration.
// It implements agent.Configurable, agent.HookProvider, and agent.MiddlewareProvider
// so that adapter layers can extract everything they need via interfaces rather
// than concrete type assertions.
type CustomAgent struct {
	config agent.Config
}

// NewAgent creates a new CustomAgent from the given configuration.
func NewAgent(cfg agent.Config) (agent.Agent, error) {
	return &CustomAgent{config: cfg}, nil
}

// ID returns the agent's unique identifier.
func (a *CustomAgent) ID() agent.ID {
	return a.config.Name
}

// Name returns the agent's name.
func (a *CustomAgent) Name() string {
	return a.config.Name
}

// Description returns the agent's description.
func (a *CustomAgent) Description() string {
	return a.config.Description
}

// SubAgents returns child agents available for delegation.
func (a *CustomAgent) SubAgents() []agent.Agent {
	return a.config.SubAgents
}

// Run executes the agent with lifecycle hook support.
// Pattern: BeforeRun hooks → Run logic → AfterRun hooks,
// with early-exit semantics when hooks return non-nil events.
func (a *CustomAgent) Run(ctx context.Context, inv *agent.Invocation) iter.Seq2[agent.Event, error] {
	return func(yield func(agent.Event, error) bool) {
		hookCtx := &agent.HookContext{
			Context:    ctx,
			Invocation: inv,
			AgentName:  a.config.Name,
			SessionID:  inv.SessionID,
			UserID:     inv.UserID,
			State:      agent.NewStateDelta(),
		}

		// 1. Run BeforeRunHooks (early-exit if non-nil event returned).
		for _, hook := range a.config.BeforeRunHooks {
			evt, err := hook(hookCtx)
			if err != nil {
				yield(nil, fmt.Errorf("before-run hook: %w", err))
				return
			}
			if evt != nil {
				if !yield(evt, nil) {
					return
				}
				// Early exit: skip agent run.
				return
			}
		}

		// 2. Emit state delta from before-hooks if any.
		if hookCtx.State.Len() > 0 {
			if !yield(&agent.StateChangeEvent{
				Key:       "_before_agent_delta",
				Value:     hookCtx.State.All(),
				EventTime: time.Now(),
			}, nil) {
				return
			}
		}

		// 3. Agent execution is handled by the RunnerAdapter/Engine.
		// The Run method emits a guidance event if the agent has a system prompt
		// but no engine attached. In production, the RunnerAdapter wraps the
		// engine loop and this Run method is not called directly.
		if a.config.SystemPrompt != "" {
			if !yield(&agent.MessageEvent{
				Message: &agent.Message{
					ID:   agent.GenerateID(),
					Role: agent.RoleSystem,
					Content: []agent.Content{{
						Type: agent.ContentTypeText,
						Text: a.config.SystemPrompt,
					}},
					Author: a.config.Name,
				},
				EventTime: time.Now(),
			}, nil) {
				return
			}
		}

		// 4. Run AfterRunHooks.
		afterCtx := &agent.HookContext{
			Context:    ctx,
			Invocation: inv,
			AgentName:  a.config.Name,
			SessionID:  inv.SessionID,
			UserID:     inv.UserID,
			State:      agent.NewStateDelta(),
		}
		for _, hook := range a.config.AfterRunHooks {
			evt, err := hook(afterCtx)
			if err != nil {
				yield(nil, fmt.Errorf("after-run hook: %w", err))
				return
			}
			if evt != nil {
				yield(evt, nil)
				return
			}
		}
	}
}

// Config returns the underlying configuration.
// Implements agent.Configurable.
func (a *CustomAgent) Config() agent.Config {
	return a.config
}

// AgentMiddlewares returns the agent's run middlewares.
func (a *CustomAgent) AgentMiddlewares() []agent.Middleware {
	return a.config.AgentMiddlewares
}

// ChatMiddlewares returns the agent's chat middlewares.
func (a *CustomAgent) ChatMiddlewares() []agent.Middleware {
	return a.config.ChatMiddlewares
}

// FunctionMiddlewares returns the agent's function middlewares.
func (a *CustomAgent) FunctionMiddlewares() []agent.Middleware {
	return a.config.FunctionMiddlewares
}

// Tools returns the agent's tools.
func (a *CustomAgent) Tools() []agent.Tool {
	return a.config.Tools
}

// BeforeToolHooks returns before-tool hooks from config.
func (a *CustomAgent) BeforeToolHooks() []agent.BeforeToolHook {
	return a.config.BeforeToolHooks
}

// AfterToolHooks returns after-tool hooks from config.
func (a *CustomAgent) AfterToolHooks() []agent.AfterToolHook {
	return a.config.AfterToolHooks
}

// BeforeModelHooks returns before-model hooks from config.
func (a *CustomAgent) BeforeModelHooks() []agent.BeforeModelHook {
	return a.config.BeforeModelHooks
}

// AfterModelHooks returns after-model hooks from config.
func (a *CustomAgent) AfterModelHooks() []agent.AfterModelHook {
	return a.config.AfterModelHooks
}

// BeforeRunHooks returns before-run hooks from config.
func (a *CustomAgent) BeforeRunHooks() []agent.BeforeRunHook {
	return a.config.BeforeRunHooks
}

// AfterRunHooks returns after-run hooks from config.
func (a *CustomAgent) AfterRunHooks() []agent.AfterRunHook {
	return a.config.AfterRunHooks
}

// Compile-time interface verification.
var (
	_ agent.Agent              = (*CustomAgent)(nil)
	_ agent.Configurable       = (*CustomAgent)(nil)
	_ agent.HookProvider       = (*CustomAgent)(nil)
	_ agent.MiddlewareProvider = (*CustomAgent)(nil)
)
