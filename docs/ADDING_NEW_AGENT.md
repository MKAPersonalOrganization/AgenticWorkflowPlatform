# Adding a New Agent to the System

This guide documents the complete integration checklist for adding a new specialized agent to the SchoolWithAI system using the **declarative agent contract pattern**.

## Overview

The system now uses a declarative approach via **AgentContracts**. Adding a new agent requires only:

1. Define the agent's event contract (`contract.go`)
2. Implement the agent factory function
3. Add event type constant
4. Register the contract in `main.go`

That's it! The event routing, tool generation, and collaboration setup is automatic.

---

---

## Step-by-Step Guide

### Step 1: Create the Agent Contract

**Location:** `services/agents/<agent-name>/contract.go`

This file defines your agent's declarative event contract. The contract specifies:
- What events the agent listens to (triggers)
- What events the agent can emit
- What domain-specific tools the agent has
- The factory function to create the agent

**Template:**

```go
package agentname

import (
	"school-with-ai/services/chatwrite/agents"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
	"school-with-ai/services/chatwrite/agents/<agent-name>/tools"
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
	"school-with-ai/sdk/agent-sdk/core"
)

// domainTools returns the domain-specific tools for this agent.
// Defined as a function to avoid initialization cycles.
func domainTools() []sdk.Tool {
	return []sdk.Tool{
		// Add your agent's specialized tools here
		// Example: tools.NewSpecializedTool(),
		
		// DO NOT add emission or collaboration tools here - those are auto-generated
	}
}

// emits returns the events this agent can emit.
// Defined as a function to avoid initialization cycles.
func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,      // Always include - signals completion
		agents.EventTypeWhiteboardUpdated,     // Always include - for collaboration
		
		// Add handoff events for other specialists you might need:
		// agents.EventTypeMathQuestion,
		// agents.EventTypePhysicsQuestion,
		// agents.EventTypeChemistryQuestion,
		// agents.EventTypeStatisticsQuestion,
		// agents.EventTypeLinearAlgebraQuestion,
		// agents.EventTypeDiagrammingQuestion,
	}
}

// Contract defines the event-driven contract for this agent.
// The system uses this to:
// - Route events to this agent automatically
// - Auto-generate emission tools for collaboration
// - Register the agent factory
var Contract = &eventbus.AgentContract{
	Name: "<agent-name>-instructor",

	// Events this agent reacts to (triggers)
	ListensTo: []agents.EventType{
		agents.EventType<Subject>Question, // "question.<subject>"
	},

	// Events this agent can emit (emission tools are auto-generated)
	Emits: emits(),

	// Domain-specific tools (emission tools auto-generated from Emits)
	Tools: domainTools(),

	// Factory function to create the agent
	Factory: Create<AgentName>Instructor,
}
```

**Real Example (Math Agent):**

```go
var Contract = &eventbus.AgentContract{
	Name: "math-instructor",
	
	ListensTo: []agents.EventType{
		agents.EventTypeMathQuestion,
	},
	
	Emits: emits(), // Returns question.answered, whiteboard.updated, physics, etc.
	
	Tools: domainTools(), // Returns calculator, graphing, plotting tools
	
	Factory: CreateMathInstructor,
}
```

---

### Step 2: Implement the Agent Factory Function

**Location:** `services/agents/<agent-name>/<agent-name>.go`

Create the factory function that builds your agent configuration.

**Template:**

```go
package agentname

import (
	"school-with-ai/sdk/agent-sdk/core"
)

// Create<AgentName>Instructor creates the agent configuration.
//
// EVENT CONTRACT:
//   Responds To (Triggers):
//     - question.<subject>: Emitted when this specialist is needed
//
//   Emits:
//     - question.answered: Signals completion (activates student-facing-agent)
//     - whiteboard.updated: Updates shared whiteboard with results
//     - question.*: Handoffs to other specialists as needed
func Create<AgentName>Instructor(notifier sdk.EventNotifier) sdk.AgentConfig {
	return sdk.AgentConfig{
		Name:           "<agent-name>-instructor",
		Role:           "instructor",
		Specialization: "<subject>",
		SystemPrompt:   `You are a specialized <subject> instructor...
		
Your workflow:
1. Read the whiteboard to see what the student asked
2. Use your specialized tools to solve the problem
3. Write your solution to the whiteboard
4. When complete, use emit_question_answered tool
5. If you need help from another specialist, use emit_<subject>_question tool

Available collaboration tools (auto-generated):
- read_whiteboard: Read current whiteboard content
- write_whiteboard: Update whiteboard with your work
- emit_question_answered: Signal you're done
- emit_<subject>_question: Ask for help from other specialists
`,
		ToolRegistry: nil, // Will be populated by eventbus system
		Notifier:     notifier,
	}
}
```

**Key Points:**
- The agent factory receives a `notifier` for emitting events
- `ToolRegistry` is initially `nil` - the eventbus system will populate it with:
  - Your domain tools from `Contract.Tools`
  - Auto-generated emission tools from `Contract.Emits`
  - Shared collaboration tools (whiteboard, etc.)
- System prompt should guide the agent on when to use emission tools

---

### Step 3: Add Event Type Constant

**Location:** `services/agents/instructor_events.go`

Add your event type constant to the list:

```go
const (
	// Student-facing agent events
	EventTypeStudentMessage     EventType = "student.message"
	EventTypeQuestionAnswered   EventType = "question.answered"
	EventTypeWhiteboardUpdated  EventType = "whiteboard.updated"
	EventTypeAgentHandoff       EventType = "agent.handoff"

	// Specialist instructor events
	EventTypeMathQuestion          EventType = "question.math"
	EventTypePhysicsQuestion       EventType = "question.physics"
	EventTypeChemistryQuestion     EventType = "question.chemistry"
	EventTypeStatisticsQuestion    EventType = "question.statistics"
	EventTypeLinearAlgebraQuestion EventType = "question.linearalgebra"
	EventTypeDiagrammingQuestion   EventType = "question.diagramming"
	
	// Add your new event type here:
	EventType<Subject>Question EventType = "question.<subject>"
)
```

**Example:**
```go
EventTypeGeometryQuestion EventType = "question.geometry"
```

---

### Step 4: Register Contract in Main

**Location:** `main.go`

Import your agent package and register its contract:

```go
import (
	// ... existing imports ...
	"school-with-ai/services/chatwrite/agents/<agent-name>"
)

func main() {
	// ... existing setup ...
	
	// Register all agent contracts
	eventbus.GlobalContracts.Register(studentfacing.Contract)
	eventbus.GlobalContracts.Register(math.Contract)
	eventbus.GlobalContracts.Register(physics.Contract)
	eventbus.GlobalContracts.Register(chemistry.Contract)
	eventbus.GlobalContracts.Register(statistics.Contract)
	eventbus.GlobalContracts.Register(linearalgebra.Contract)
	eventbus.GlobalContracts.Register(diagramming.Contract)
	
	// Add your new contract here:
	eventbus.GlobalContracts.Register(<agentname>.Contract)
	
	// ... rest of main ...
}
```

---

## What Happens Automatically

When you register a contract, the system automatically:

1. **Generates Emission Tools**: For each event in `Contract.Emits`, creates a tool like `emit_<event>_question`
2. **Routes Events**: Maps events in `Contract.ListensTo` to your agent
3. **Builds Tool Registry**: Combines domain tools + emission tools + shared tools
4. **Enables Collaboration**: Agents can emit events to trigger other specialists

### Auto-Generated Tools

If your contract emits these events:
```go
Emits: []agents.EventType{
	agents.EventTypeQuestionAnswered,
	agents.EventTypePhysicsQuestion,
}
```

The system generates:
- `emit_question_answered` tool
- `emit_physics_question` tool

Your agent can use these to signal completion or request help from other specialists.

---

## Complete Example: Geometry Agent

Here's a complete example of adding a geometry agent.

### File: `services/agents/geometry/contract.go`

```go
package geometry

import (
	"school-with-ai/services/chatwrite/agents"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
	"school-with-ai/services/chatwrite/agents/geometry/tools"
	"school-with-ai/sdk/agent-sdk/core"
)

func domainTools() []sdk.Tool {
	return []sdk.Tool{
		tools.NewShapeCalculatorTool(),
		tools.NewGeometricProofTool(),
	}
}

func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,
		agents.EventTypeWhiteboardUpdated,
		agents.EventTypeMathQuestion,          // Might need calculus
		agents.EventTypeLinearAlgebraQuestion, // Might need vectors
	}
}

var Contract = &eventbus.AgentContract{
	Name:      "geometry-instructor",
	ListensTo: []agents.EventType{agents.EventTypeGeometryQuestion},
	Emits:     emits(),
	Tools:     domainTools(),
	Factory:   CreateGeometryInstructor,
}
```

### File: `services/agents/geometry/geometry.go`

```go
package geometry

import "school-with-ai/sdk/agent-sdk/core"

func CreateGeometryInstructor(notifier sdk.EventNotifier) sdk.AgentConfig {
	return sdk.AgentConfig{
		Name:           "geometry-instructor",
		Role:           "instructor",
		Specialization: "geometry",
		SystemPrompt: `You are a geometry specialist.

Workflow:
1. Use read_whiteboard to see the student's geometry question
2. Use shape_calculator or geometric_proof tools to solve
3. Use write_whiteboard to show your work
4. Use emit_question_answered when done
5. Use emit_math_question if you need calculus help
6. Use emit_linearalgebra_question if you need vector help
`,
		ToolRegistry: nil,
		Notifier:     notifier,
	}
}
```

### File: `services/agents/instructor_events.go`

```go
const (
	// ... existing events ...
	EventTypeGeometryQuestion EventType = "question.geometry"
)
```

### File: `main.go`

```go
import (
	// ... existing ...
	"school-with-ai/services/chatwrite/agents/geometry"
)

func main() {
	// ... setup ...
	eventbus.GlobalContracts.Register(geometry.Contract)
	// ... rest ...
}
```

Done! The geometry agent is now fully integrated.

---

## What You DON'T Need to Do Anymore

The old pattern required manual updates in many places. **You no longer need to:**

❌ ~~Manually create emission tool structs~~ (auto-generated)  
❌ ~~Update `worker_pool.go` routing logic~~ (uses contract registry)  
❌ ~~Update `collaboration_tools.go`~~ (uses contract registry)  
❌ ~~Add tools to `GetHandoffEventForTool()`~~ (deprecated)  
❌ ~~Add cases to `HandleCollaborationToolCall()`~~ (deprecated)  
❌ ~~Update `ui_notifier.go` switch statements~~ (optional)

---

## Troubleshooting

### Agent Not Receiving Events

Check:
1. Event type constant is defined in `instructor_events.go`
2. Event type is listed in `Contract.ListensTo`
3. Contract is registered in `main.go`
4. Event is being emitted with correct event type string

### Emission Tool Not Available

Check:
1. Event type is listed in `Contract.Emits`
2. Contract is registered before worker pool starts
3. Tool name matches pattern: `emit_<event>_question` (e.g., `emit_physics_question`)

### Import Cycles

If you get import cycle errors:
- Use functions like `domainTools()` and `emits()` instead of package-level variables
- Don't import other agent packages in contract files

---

## Testing Your Agent

1. **Start the server**
   ```bash
   task dev
   ```

2. **Send a test message** that should trigger your agent
   ```
   "Can you help me with <your subject topic>?"
   ```

3. **Check logs** for:
   - Event emission: `Emitting event: question.<subject>`
   - Routing: `Routing event question.<subject> to <agent-name>-instructor`
   - Tool execution: `Executing tool: emit_question_answered`

4. **Verify whiteboard** updates show your agent's work

---

## Summary Checklist

- [ ] Create `services/agents/<name>/contract.go` with `Contract` variable
- [ ] Implement `Create<Name>Instructor()` factory function
- [ ] Add `EventType<Subject>Question` constant to `instructor_events.go`
- [ ] Register contract in `main.go` via `eventbus.GlobalContracts.Register()`
- [ ] Test agent receives events and can emit events
- [ ] Verify collaboration with other agents works

**That's it!** The system handles everything else automatically.

---

## Key Files Reference

| File | Purpose |
|------|---------|
| `services/agents/<name>/contract.go` | Define agent's event contract |
| `services/agents/<name>/<name>.go` | Implement agent factory function |
| `services/agents/instructor_events.go` | Event type constants |
| `services/agents/shared/tools/eventbus/registry.go` | Global contract registry |
| `services/agents/shared/tools/eventbus/emission.go` | Auto-generates emission tools |
| `main.go` | Register all contracts |
| `infra/worker_pool.go` | Routes events via contract registry |

For more details on the event architecture, see:
- [EVENT_ARCHITECTURE.md](EVENT_ARCHITECTURE.md)
- [EVENT_FLOWS.md](EVENT_FLOWS.md)

