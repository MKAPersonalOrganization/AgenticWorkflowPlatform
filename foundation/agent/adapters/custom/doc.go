// Package custom provides adapters that wrap the foundation/agentsdk implementation
// to conform to the foundation/agent interfaces.
//
// This adapter layer allows the existing agentsdk-based code to work with the new
// framework-agnostic interfaces, enabling a gradual migration and future support
// for multiple agent frameworks (ADK-Go, Blades, etc.).
//
// # Architecture
//
// The custom adapter wraps the following agentsdk types:
//
//   - AgentAdapter: wraps core.AgentConfig
//   - ToolAdapter: wraps core.Tool
//   - ModelAdapter: wraps llm.ChatClient
//   - SessionAdapter: wraps domain.ThreadAggregate
//   - SessionStoreAdapter: wraps domain.ThreadRepository
//   - RunnerAdapter: wraps agentorchestrator.Service
//
// # Usage
//
// Create a runner from an existing orchestrator service:
//
//	runner := custom.NewRunnerAdapter(log, orchestratorService)
//
//	req := &agent.RunRequest{
//	    AgentName: "math-instructor",
//	    SessionID: threadID,
//	    UserID:    userID,
//	    Message: &agent.Message{
//	        Role: agent.RoleUser,
//	        Content: []agent.Content{{
//	            Type: agent.ContentTypeText,
//	            Text: "What is 2+2?",
//	        }},
//	    },
//	}
//
//	for event, err := range runner.Run(ctx, req) {
//	    // Handle events
//	}
//
// # Event Conversion
//
// The adapter automatically converts between core.Event and agent.Event types,
// mapping event types and payloads appropriately.
package custom
