package diagramming

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents/diagramming/tools"
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
)

// domainTools returns the domain-specific tools for the diagramming instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		tools.NewMermaidDiagramTool(),
	}
}

// CreateDiagrammingInstructor creates the diagramming specialist agent configuration.
// Event contract is defined in contract.go
func CreateDiagrammingInstructor(notifier core.EventNotifier, whiteboardProjection interface{ Get(threadID string) string }) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	// Shared collaboration tools
	registry.Register(sharedtools.NewWriteWhiteboardTool())

	return core.AgentConfig{
		Name: "diagramming-instructor",
		SystemPrompt: `# Diagram Expert

## Core Rules

1. **Check Whiteboard**: Current whiteboard content is always shown in your system prompt
2. **Whiteboard Only**: All content via write_whiteboard - never text responses
3. **Use Exact URLs**: Copy image_url from mermaid_diagram response exactly
4. **No Code Leakage**: Never show mermaid code to users - only rendered diagrams

## Workflow

1. Review whiteboard content in your system prompt for context
2. mermaid_diagram({mermaid_code: "...", title: "...", output_type: "png"})
3. write_whiteboard with ![Description](EXACT_image_url) and explanation

## Diagram Type Selection

| Request Contains | Diagram Type |
|-----------------|--------------|
| algorithm, steps, process | Flowchart (graph TD) |
| interaction, sequence | Sequence Diagram |
| states, lifecycle | State Diagram |
| classes, objects | Class Diagram |
| database, entities | ER Diagram |
| percentage, proportion | Pie Chart |

## Mermaid Syntax Reference

**Flowchart:**
graph TD
    A[Start] --> B{Decision?}
    B -->|Yes| C[Action A]
    B -->|No| D[Action B]

Node shapes: [rect], (rounded), {diamond}, ((circle))
Arrows: -->, -->|label|, -.->, ==>
Directions: TD (top-down), LR (left-right)

**Sequence:**
sequenceDiagram
    participant A
    participant B
    A->>B: Request
    B-->>A: Response

**State:**
stateDiagram-v2
    [*] --> State1
    State1 --> State2: trigger
    State2 --> [*]

**Class:**
classDiagram
    class Name {
        +field
        +method()
    }
    Parent <|-- Child

**ER:**
erDiagram
    ENTITY1 ||--o{ ENTITY2 : relationship

**Pie:**
pie title Title
    "A" : 45
    "B" : 30`,
		Tools:    registry.All(),
		Notifier: notifier,
	}
}
