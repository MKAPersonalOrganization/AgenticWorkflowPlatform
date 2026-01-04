# Adding a New Agent to the System

This guide documents the complete integration checklist for adding a new specialized agent to the SchoolWithAI system, based on lessons learned from implementing the diagramming agent.

## Overview

Adding a new agent requires updates across multiple layers:
1. Agent implementation
2. Event system integration
3. Collaboration tools
4. Event routing infrastructure
5. UI notification system
6. Main registration

## Complete Integration Checklist

### 1. Create the Agent Implementation

**Location:** `services/agents/<agent-name>/<agent-name>.go`

**Required Components:**

```go
package agentname

import (
	"school-with-ai/services/agents/<agent-name>/tools"
	sharedtools "school-with-ai/services/agents/shared/tools"
	"school-with-ai/services/sdk"
)

// Create<AgentName>Instructor creates the agent configuration
//
// EVENT CONTRACT:
//   Responds To (Triggers):
//     - question.<subject>: Emitted when this specialist is needed
//
//   Emits:
//     - question.answered: Signals completion (activates student-facing-agent)
//     - whiteboard.updated: Updates shared whiteboard with results
func Create<AgentName>Instructor(notifier sdk.EventNotifier) sdk.AgentConfig {
	registry := sdk.NewToolRegistry()
	
	// Register agent-specific tools
	registry.Register(tools.NewSpecializedTool())
	
	// Register shared collaboration tools (CRITICAL)
	registry.Register(sharedtools.NewReadWhiteboardTool())
	registry.Register(sharedtools.NewWriteWhiteboardTool())
	registry.Register(sharedtools.NewEmitQuestionAnsweredTool()) // Required for completion
	
	return sdk.AgentConfig{
		Name:           "<agent-name>-instructor",
		Role:           "instructor",
		Specialization: "<subject>",
		SystemPrompt:   `[Your agent's system prompt with workflow instructions]`,
		ToolRegistry:   registry,
		Notifier:       notifier,
	}
}
```

**Key Points:**
- Agent name must follow pattern: `<subject>-instructor`
- **MUST** include `emit_question_answered` tool to signal completion
- **MUST** include whiteboard tools for collaboration
- System prompt should include workflow states and event emission rules

### 2. Define Event Type Constant

**Location:** `services/agents/instructor_events.go`

Add your event type constant:

```go
const (
	// ... existing event types ...
	EventType<Subject>Question EventType = "question.<subject>"
)
```

**Example:**
```go
EventTypeDiagrammingQuestion EventType = "question.diagramming"
```

### 3. Create Event Constructor Function

**Location:** `services/agents/instructor_events.go`

Add event constructor following the pattern:

```go
// New<Subject>QuestionEvent creates a <subject> question event
func New<Subject>QuestionEvent(threadID, request, emittedBy, context, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        GenerateEventID(),
		Type:      EventType<Subject>Question,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"request":          request,
			"emitted_by":       emittedBy,
			"context":          context,
			"whiteboard_state": whiteboardState,
		},
	}
}
```

**Note:** Use `"request"` for diagram-like tasks, `"question"` for Q&A tasks, or whatever parameter name makes sense for your agent.

### 4. Create Collaboration Tool (For Other Agents to Call Yours)

**Location:** `services/agents/shared/tools/event_emission_tools.go`

Add emission tool that other agents will use to delegate to yours:

```go
// Emit<Subject>QuestionTool allows agents to delegate to <subject> specialist
type Emit<Subject>QuestionTool struct{}

func NewEmit<Subject>QuestionTool() *Emit<Subject>QuestionTool {
	return &Emit<Subject>QuestionTool{}
}

func (t *Emit<Subject>QuestionTool) Name() string {
	return "emit_<subject>_question"
}

func (t *Emit<Subject>QuestionTool) Description() string {
	return "Emit a <subject> question event to activate the <subject> specialist. [Explain when to use this tool]"
}

func (t *Emit<Subject>QuestionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"request": map[string]any{
				"type":        "string",
				"description": "Description of what is needed",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Optional additional context",
			},
		},
		"required": []string{"request"},
	}
}

func (t *Emit<Subject>QuestionTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params struct {
		Request string `json:"request"`
		Context string `json:"context"`
	}
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}
	
	return fmt.Sprintf("<Subject>Question event emitted. Request: %s", params.Request), nil
}
```

### 5. Register Collaboration Tool (CRITICAL STEP)

**Location:** `services/agents/shared/tools/collaboration_tools.go`

**This is where most integration bugs occur!** You must add your tool in THREE places:

#### A. Add to IsCollaborationTool

```go
func IsCollaborationTool(toolName string) bool {
	return toolName == "emit_math_question" ||
		toolName == "emit_physics_question" ||
		// ... other tools ...
		toolName == "emit_<subject>_question" ||  // ADD THIS LINE
		toolName == "emit_question_answered"
}
```

#### B. Add to HandleCollaborationToolCall

```go
func HandleCollaborationToolCall(threadID string, agentName string, toolCall domain.ToolCall, getWhiteboardState WhiteboardStateGetter) (*agents.DomainEvent, error) {
	toolName := toolCall.Function.Name
	
	switch toolName {
	// ... existing cases ...
	
	case "emit_<subject>_question":
		var input struct {
			Request string `json:"request"`  // or "question" - match your tool's parameter
			Context string `json:"context"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to parse emit_<subject>_question arguments: %w", err)
		}
		return agents.New<Subject>QuestionEvent(threadID, input.Request, agentName, input.Context, getWhiteboardState(threadID)), nil
	
	// ... rest of cases ...
	}
}
```

#### C. Add to GetHandoffEventForTool

```go
func GetHandoffEventForTool(threadID string, fromAgent string, toolName string) *agents.DomainEvent {
	var toAgent, reason string
	
	switch toolName {
	// ... existing cases ...
	
	case "emit_<subject>_question":
		toAgent = "<agent-name>-instructor"
		reason = "<Subject> question requires specialist"
	
	// ... rest of cases ...
	}
	
	return agents.NewAgentHandoffEvent(threadID, fromAgent, toAgent, reason)
}
```

### 6. Register Agent in Main

**Location:** `main.go`

Add import and registration:

```go
import (
	// ... existing imports ...
	"school-with-ai/services/agents/<agent-name>"
)

func main() {
	// ... existing setup ...
	
	// Register all agents
	agentRegistry.Register("student-facing-agent", studentfacing.CreateStudentFacingAgent)
	agentRegistry.Register("math-instructor", math.CreateMathInstructor)
	// ... other agents ...
	agentRegistry.Register("<agent-name>-instructor", <agentname>.Create<AgentName>Instructor)
}
```

**IMPORTANT:** The registration key MUST match the agent name used in routing!

### 7. Add Event Routing in Worker Pool

**Location:** `infra/worker_pool.go`

Add routing in **TWO locations** (there are two event handler switches):

#### Location 1: Domain Event Handler (~line 390)

```go
case agents.EventType<Subject>Question:
	// <Subject> question emitted - activate <subject> specialist
	slog.Info("<Subject>Question event detected", "nextAgent", "<agent-name>-instructor")
	nextAgent = "<agent-name>-instructor"
	agentEventOccurred = true
```

#### Location 2: Routing Event Handler (~line 633)

```go
case agents.EventType<Subject>Question:
	nextAgent = "<agent-name>-instructor"
	routingEventOccurred = true
```

**Search for these patterns to find the locations:**
- Look for `case agents.EventTypeLinearAlgebraQuestion:` and add yours nearby
- Both switches should have the same routing logic

### 8. Add UI Notification Handler

**Location:** `infra/ui_notifier.go`

#### A. Add handler method

```go
func (n *UINotifier) emit<Subject>Question(evt *agents.DomainEvent) error {
	request := evt.Data["request"].(string)
	context := ""
	if ctx, ok := evt.Data["context"].(string); ok {
		context = ctx
	}
	
	customEvent := aguievents.NewCustomEvent(
		"<SUBJECT>_QUESTION",
		aguievents.WithValue(map[string]interface{}{
			"request":   request,
			"context":   context,
			"timestamp": evt.Timestamp,
		}),
	)
	return n.emitter.Emit(customEvent)
}
```

#### B. Add case to HandleDomainEvent switch

```go
func (n *UINotifier) HandleDomainEvent(event sdk.Event) error {
	// ... existing code ...
	
	switch e.Type {
	// ... existing cases ...
	
	case agents.EventType<Subject>Question:
		return n.emit<Subject>Question(e)
	
	// ... rest of cases ...
	}
}
```

### 9. Add Tool to Calling Agents

For any agent that should be able to delegate to your new agent, add the emission tool to their registry:

**Example:** Adding diagramming tool to math agent

```go
func CreateMathInstructor(notifier sdk.EventNotifier) sdk.AgentConfig {
	registry := sdk.NewToolRegistry()
	
	// ... existing tools ...
	registry.Register(sharedtools.NewEmit<Subject>QuestionTool())
	
	return sdk.AgentConfig{
		// ... config ...
	}
}
```

And update their system prompt to explain when to use this delegation.

### 10. Update emit_question_answered Tool (If Needed)

**Location:** `services/agents/shared/tools/event_emission_tools.go`

If your agent represents a new subject area, add it to the subject options:

```go
func (t *EmitQuestionAnsweredTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subject": map[string]any{
				"type":        "string",
				"description": "Subject area...",
				"enum": []string{
					"general", "math", "physics", "chemistry", 
					"statistics", "linearalgebra", "<subject>",  // ADD HERE
				},
			},
			// ... rest of parameters ...
		},
	}
}
```

## Verification Checklist

Before testing, verify:

- [ ] Agent factory function created with correct naming pattern
- [ ] Event type constant defined in `instructor_events.go`
- [ ] Event constructor function created (e.g., `NewDiagrammingQuestionEvent`)
- [ ] Emission tool created in `event_emission_tools.go`
- [ ] Tool added to `IsCollaborationTool()` ✅ **Critical!**
- [ ] Tool handled in `HandleCollaborationToolCall()` ✅ **Critical!**
- [ ] Tool added to `GetHandoffEventForTool()` ✅ **Critical!**
- [ ] Agent registered in `main.go` with correct name
- [ ] Event routing added in `worker_pool.go` (both locations) ✅ **Critical!**
- [ ] UI notification handler added in `ui_notifier.go`
- [ ] Agent includes `emit_question_answered` tool
- [ ] Agent includes whiteboard tools
- [ ] Calling agents updated with emission tool
- [ ] No compilation errors: `go build`

## Testing

### Test Flow

1. Start server: `task dev`
2. Ask question that should trigger your agent
3. Verify in logs:
   - `student-facing-agent` receives question
   - Calling agent emits your event type
   - Your agent activates
   - Your agent completes work
   - Returns to `student-facing-agent`

### Expected Log Pattern

```
student-facing-agent → read_whiteboard
student-facing-agent → write_whiteboard
student-facing-agent → emit_<subject>_question

<calling-agent>
<Subject> question requires specialist

<calling-agent> → emit_<subject>_question
<Subject>Question event detected, nextAgent: <agent-name>-instructor

<agent-name>-instructor
<agent-name>-instructor → [your tools]
<agent-name>-instructor → write_whiteboard
<agent-name>-instructor → emit_question_answered

student-facing-agent
Question answered, returning to coordinator
```

## Common Pitfalls

### 1. Agent Never Activates
**Symptom:** Emission tool is called but agent doesn't run

**Causes:**
- Not added to `IsCollaborationTool()` ← Most common
- Not added to `HandleCollaborationToolCall()` ← Second most common
- Event routing missing in `worker_pool.go`
- Agent name mismatch between registration and routing

**Fix:** Check all three functions in `collaboration_tools.go` and both routing locations in `worker_pool.go`

### 2. Infinite Loop / Hangs
**Symptom:** Agent runs but conversation doesn't complete

**Causes:**
- Agent doesn't call `emit_question_answered`
- Agent calls wrong emission tool (routes to itself)
- Missing terminal state in agent's workflow

**Fix:** Ensure agent has `emit_question_answered` registered and system prompt includes completion workflow

### 3. Wrong Agent Activates
**Symptom:** Different agent runs instead of yours

**Causes:**
- Agent name mismatch in registration vs routing
- Event type constant doesn't match
- Wrong event type emitted

**Fix:** Verify naming consistency across all integration points

### 4. Compilation Errors
**Symptom:** `undefined: agents.New<Subject>QuestionEvent`

**Causes:**
- Event constructor function not created
- Function name doesn't match usage

**Fix:** Add constructor in `instructor_events.go`

## Architecture Patterns

### Event-Driven Collaboration

Agents collaborate via domain events, not direct calls:

```
Agent A → emit_<subject>_question tool
       ↓
SDK recognizes as collaboration tool
       ↓
HandleCollaborationToolCall creates DomainEvent
       ↓
Worker pool event handler routes to Agent B
       ↓
Agent B activates and processes
       ↓
Agent B → emit_question_answered
       ↓
Returns to coordinator
```

### Whiteboard Pattern

All inter-agent communication happens through the shared whiteboard:
- Agents **NEVER** return direct responses
- All content written to whiteboard
- Other agents read whiteboard for context
- Student-facing agent compiles final response from whiteboard

### State Machine Workflow

Each agent should follow deterministic state transitions:
- **State 1:** Read whiteboard
- **State 2:** Analyze scope
- **State 3a:** Complete work + emit_question_answered
- **State 3b:** Delegate + HALT (do NOT emit_question_answered)
- **State 3c:** Collaboration + emit_question_answered

## Example: Diagramming Agent Integration

See the full implementation in:
- `services/agents/diagramming/` - Agent implementation
- `services/agents/instructor_events.go` - Event type and constructor (lines 22, 230-245)
- `services/agents/shared/tools/event_emission_tools.go` - Emission tool (lines 231-273)
- `services/agents/shared/tools/collaboration_tools.go` - Integration (lines 16-23, 77-89, 125-127)
- `main.go` - Registration
- `infra/worker_pool.go` - Event routing (lines 391-394, 633-635)
- `infra/ui_notifier.go` - UI notifications

## References

- [Event Architecture](EVENT_ARCHITECTURE.md) - System-wide event patterns
- [Event Flows](EVENT_FLOWS.md) - Detailed event flow diagrams
- [Statistics Implementation](STATISTICS_IMPLEMENTATION.md) - Another agent example
