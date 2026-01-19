package diagramming

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/services/agentorchestrator/agents/diagramming/tools"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
)

// domainTools returns the domain-specific tools for the diagramming instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		tools.NewMermaidDiagramTool(),
		core.NewWriteTodosTool(),
	}
}

// CreateDiagrammingInstructor creates the diagramming specialist agent configuration.
// Event contract is defined in contract.go.
func CreateDiagrammingInstructor(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	// Shared tools
	for _, tool := range sharedtools.NewGrepTool() {
		registry.Register(tool)
	}
	for _, tool := range sharedtools.NewLibraryTools() {
		registry.Register(tool)
	}

	prompt, err := core.GenerateSystemPrompt(core.PromptConfig{
		Role:        "Diagram Expert",
		Preamble:    "You are a specialist in creating clear, informative, and visually appealing diagrams using Mermaid.js to help students visualize complex concepts.",
		Topic:       "Diagramming and Visualization",
		Interactive: true,
		ExtraRules: `## Core Diagramming Rules
1. **Subject Authority**: You are the SOLE authority for Diagramming and visualization.
2. **Use Exact URLs**: Copy image_url from mermaid_diagram response exactly.
3. **No Code Leakage**: Never show mermaid code to users - only rendered diagrams.

## Diagram Workflow
1. mermaid_diagram({mermaid_code: "...", title: "...", output_type: "png"})
2. Include the diagram in your response: ![Description](EXACT_image_url)

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

**Sequence Diagram:**
sequenceDiagram
    participant A
    participant B
    A->>B: Request
    B-->>A: Response

**State Diagram:**
stateDiagram-v2
    [*] --> State1
    --> State2: trigger
    State2 --> [*]

**Class Diagram:**
classDiagram
    class Name {
        +field
        +method()
    }
    Parent <|-- Child

**ER Diagram:**
erDiagram
    ENTITY1 ||--o{ ENTITY2 : relationship

**Pie Chart:**
pie title Title
    "A" : 45
    "B" : 30`,
		IncludeTodos: true,
	})
	if err != nil {
		panic(err)
	}

	return core.AgentConfig{
		Name:          "diagramming-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Notifier:      notifier,
		Middleware: []core.Middleware{
			middleware.NewElicitationMiddleware(),
		},
	}
}
