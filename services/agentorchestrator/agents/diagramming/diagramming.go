package diagramming

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/services/agentorchestrator/agents/diagramming/tools"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
)

// domainTools returns the domain-specific tools for the diagramming instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		tools.NewMermaidDiagramTool(),
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

	// Shared collaboration tools
	registry.Register(sharedtools.NewWriteWhiteboardTool())

	return core.AgentConfig{
		Name: "diagramming-instructor",
		SystemPrompt: "# Diagram Expert\n\n## Core Rules\n\n1. **Check Whiteboard**: Current whiteboard content is always shown in your system prompt\n2. **Whiteboard Only**: All content via write_whiteboard - never text responses\n3. **Use Exact URLs**: Copy image_url from mermaid_diagram response exactly\n4. **No Code Leakage**: Never show mermaid code to users - only rendered diagrams\n\n## Workflow\n\n1. Review whiteboard content in your system prompt for context\n2. mermaid_diagram({mermaid_code: \"...\", title: \"...\", output_type: \"png\"})\n3. write_whiteboard with ![Description](EXACT_image_url) and explanation\n\n## Diagram Type Selection\n\n| Request Contains | Diagram Type |\n|-----------------|--------------|\n| algorithm, steps, process | Flowchart (graph TD) | interaction, sequence | Sequence Diagram | states, lifecycle | State Diagram | classes, objects | Class Diagram | database, entities | ER Diagram | percentage, proportion | Pie Chart |\n\n## Mermaid Syntax Reference\n\n**Flowchart:**\ngraph TD\n    A[Start] --> B{Decision?}\n    B -->|Yes| C[Action A]\n    B -->|No| D[Action B]\n\nNode shapes: [rect], (rounded), {diamond}, ((circle))\nArrows: -->, -->|label|, -.->, ==>\nDirections: TD (top-down), LR (left-right)\n\n**Sequence:**\nsequenceDiagram\n    participant A\n    participant B\n    A->>B: Request\n    B-->>A: Response\n\n**State:**\nstateDiagram-v2\n    [*] --> State1\n    --> State2: trigger\n    State2 --> [*]\n\n**Class:**\nclassDiagram\n    class Name {\n        +field\n        +method()\n    }\n    Parent <|-- Child\n\n**ER:**\nerDiagram\n    ENTITY1 ||--o{ ENTITY2 : relationship\n\n**Pie:**\npie title Title\n    \"A\" : 45\n    \"B\" : 30",
		Tools:        registry.All(),
		Notifier:     notifier,
	}
}
