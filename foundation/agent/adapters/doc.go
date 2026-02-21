// Package adapters provides framework-specific adapters for the agent abstraction layer.
//
// Each adapter translates between the foundation/agent interfaces and a specific
// agent framework. This allows the application layer to work with any framework
// through a consistent interface.
//
// # Available Adapters
//
//   - custom: Wraps foundation/agentsdk
//   - adkgo: Wraps Google ADK-Go
//   - blades: Wraps Go-Kratos Blades
//
// # Adapter Responsibilities (per ADR 001)
//
//  1. Implement agent.Agent and agent.Tool interfaces
//  2. Translate framework-specific events to agent.Event types
//  3. Translate agent.Event types to domain.Event for the service layer
//  4. Handle framework-specific configuration and initialization
//
// # What Adapters Should NOT Do (per ADR 002)
//
//   - Emit run lifecycle events (RunStarted, RunFinished, RunError)
//   - Persist events directly to the event store
//   - Make decisions about event classification (ephemeral vs persistent)
package adapters
