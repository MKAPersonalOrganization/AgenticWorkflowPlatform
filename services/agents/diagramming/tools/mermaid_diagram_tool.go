package tools

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// MermaidDiagramArgs represents the arguments for the mermaid diagram tool
type MermaidDiagramArgs struct {
	MermaidCode string `json:"mermaid_code"` // Complete mermaid diagram definition
	Title       string `json:"title"`        // Diagram title (for reference/logging)
	OutputType  string `json:"output_type"`  // Output format: "svg" or "png" (default: "png")
}

// MermaidDiagramTool implements the Tool interface for generating diagrams with mermaid-cli
type MermaidDiagramTool struct{}

// NewMermaidDiagramTool creates a new mermaid diagram tool
func NewMermaidDiagramTool() *MermaidDiagramTool {
	return &MermaidDiagramTool{}
}

func (m *MermaidDiagramTool) Name() string {
	return "mermaid_diagram"
}

func (m *MermaidDiagramTool) Description() string {
	return "Generate diagrams using Mermaid syntax via Docker. Create flowcharts, sequence diagrams, class diagrams, state diagrams, ER diagrams, and more. You provide mermaid diagram code, and the tool executes it via Docker to create an image file. Perfect for visualizing processes, workflows, algorithms, and relationships."
}

func (m *MermaidDiagramTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mermaid_code": map[string]any{
				"type": "string",
				"description": `Complete mermaid diagram definition. REQUIRED. The code should be valid mermaid syntax.

SUPPORTED DIAGRAM TYPES:
1. Flowchart - Process flows and decision trees
2. Sequence Diagram - Interaction between entities over time
3. Class Diagram - Object-oriented class relationships
4. State Diagram - State transitions and lifecycle
5. ER Diagram - Entity relationships in databases
6. Gantt Chart - Project timelines
7. Pie Chart - Proportional data
8. Git Graph - Version control branching

MERMAID SYNTAX EXAMPLES:

Flowchart:
graph TD
    A[Start] --> B{Is it working?}
    B -->|Yes| C[Great!]
    B -->|No| D[Debug]
    D --> B

Sequence Diagram:
sequenceDiagram
    participant A as Alice
    participant B as Bob
    A->>B: Hello Bob!
    B->>A: Hi Alice!
    A->>B: How are you?
    B->>A: I'm good, thanks!

Class Diagram:
classDiagram
    class Animal {
        +String name
        +int age
        +makeSound()
    }
    class Dog {
        +String breed
        +bark()
    }
    Animal <|-- Dog

State Diagram:
stateDiagram-v2
    [*] --> Idle
    Idle --> Processing: Start
    Processing --> Success: Complete
    Processing --> Error: Fail
    Success --> [*]
    Error --> Idle: Retry

ER Diagram:
erDiagram
    CUSTOMER ||--o{ ORDER : places
    ORDER ||--|{ LINE-ITEM : contains
    CUSTOMER }|..|{ DELIVERY-ADDRESS : uses

Pie Chart:
pie title Pets Distribution
    "Dogs" : 386
    "Cats" : 85
    "Rabbits" : 15

Git Graph:
gitGraph
    commit
    branch develop
    checkout develop
    commit
    checkout main
    merge develop
    commit

Remember: Ensure your mermaid syntax is valid. The diagram will be rendered exactly as specified.`,
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Brief description of the diagram for logging/reference purposes. Example: 'Algorithm flowchart', 'System architecture'",
				"maxLength":   100,
			},
			"output_type": map[string]any{
				"type":        "string",
				"description": "Output format: 'svg' for scalable vector graphics or 'png' for raster image. Default is 'png'.",
				"enum":        []string{"svg", "png"},
			},
		},
		"required": []string{"mermaid_code"},
	}
}

func (m *MermaidDiagramTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args MermaidDiagramArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse mermaid diagram args: %w", err)
	}

	// Validate required mermaid_code parameter
	if args.MermaidCode == "" {
		return "", fmt.Errorf("mermaid_code parameter is required")
	}

	// Set default output type
	if args.OutputType == "" {
		args.OutputType = "png"
	}

	// Validate output type
	if args.OutputType != "svg" && args.OutputType != "png" {
		return "", fmt.Errorf("output_type must be 'svg' or 'png', got: %s", args.OutputType)
	}

	// Validate title length
	if len(args.Title) > 100 {
		return "", fmt.Errorf("title exceeds maximum length of 100 characters")
	}

	// Generate the diagram
	imagePath, err := executeMermaidDiagram(args.MermaidCode, args.Title, args.OutputType)
	if err != nil {
		return "", fmt.Errorf("failed to generate diagram: %w", err)
	}

	// Return result with image path
	result := map[string]any{
		"success":     true,
		"image_path":  imagePath,
		"image_url":   "/assets/charts/" + filepath.Base(imagePath),
		"output_type": args.OutputType,
		"message":     "Diagram generated successfully",
	}
	if args.Title != "" {
		result["title"] = args.Title
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal diagram result: %w", err)
	}

	return string(resultJSON), nil
}

// executeMermaidDiagram executes mermaid-cli via Docker to generate the diagram
func executeMermaidDiagram(mermaidCode, title, outputType string) (string, error) {
	// Ensure charts directory exists
	chartsDir := "assets/charts"
	if err := os.MkdirAll(chartsDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create charts directory: %w", err)
	}

	// Generate unique filename
	hash := fmt.Sprintf("%x", md5.Sum([]byte(mermaidCode+title+fmt.Sprintf("%d", time.Now().UnixNano()))))
	hash = hash[:16] // Use first 16 chars

	// Define file paths
	inputFileName := fmt.Sprintf("mermaid_%s_%d.mmd", hash, time.Now().Unix())
	outputFileName := fmt.Sprintf("diagram_%s_%d.%s", hash, time.Now().Unix(), outputType)

	inputPath := filepath.Join(chartsDir, inputFileName)
	outputPath := filepath.Join(chartsDir, outputFileName)

	// Write mermaid code to input file
	if err := os.WriteFile(inputPath, []byte(mermaidCode), 0o644); err != nil {
		return "", fmt.Errorf("failed to write mermaid input file: %w", err)
	}
	defer func() { _ = os.Remove(inputPath) }() // Clean up input file

	// Get absolute path for Docker volume mounting
	absChartsDir, err := filepath.Abs(chartsDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Execute mermaid-cli via Docker
	// Docker command: docker run --rm -v /path/to/charts:/data minlag/mermaid-cli -i input.mmd -o output.png
	cmd := exec.Command(
		"docker", "run", "--rm",
		"-u", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-v", fmt.Sprintf("%s:/data", absChartsDir),
		"minlag/mermaid-cli",
		"-i", fmt.Sprintf("/data/%s", inputFileName),
		"-o", fmt.Sprintf("/data/%s", outputFileName),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("mermaid-cli execution failed: %w\nMermaid Code:\n%s\nOutput: %s", err, mermaidCode, string(output))
	}

	// Verify the image was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("mermaid-cli did not create the expected output file at %s\nOutput: %s", outputPath, string(output))
	}

	return outputPath, nil
}
