package tools

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// MermaidDiagramArgs represents the arguments for the mermaid diagram tool.
type MermaidDiagramArgs struct {
	MermaidCode string `json:"mermaid_code"` // Complete mermaid diagram definition
	Title       string `json:"title"`        // Diagram title (for reference/logging)
	OutputType  string `json:"output_type"`  // Output format: "svg" or "png" (default: "png")
}

// MermaidDiagramTool implements the Tool interface for generating diagrams with mermaid-cli.
type MermaidDiagramTool struct{}

// NewMermaidDiagramTool creates a new mermaid diagram tool.
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
				"type":        "string",
				"description": "Complete mermaid diagram definition. REQUIRED. The code should be valid mermaid syntax.\n\nSUPPORTED DIAGRAM TYPES:\n1. Flowchart - Process flows and decision trees\n2. Sequence Diagram - Interaction between entities over time\n3. Class Diagram - Object-oriented class relationships\n4. State Diagram - State transitions and lifecycle\n5. ER Diagram - Entity relationships in databases\n6. Gantt Chart - Project timelines\n7. Pie Chart - Proportional data\n8. Git Graph - Version control branching\n\nMERMAID SYNTAX EXAMPLES:\n\nFlowchart:\ngraph TD\n    A[Start] --> B{Is it working?}\n    B -->|Yes| C[Great!]\n    B -->|No| D[Debug]\n    D --> B\n\nSequence Diagram:\nsequenceDiagram\n    participant A as Alice\n    participant B as Bob\n    A->>B: Hello Bob!\n    B->>A: Hi Alice!\n    A->>B: How are you?\n    B->>A: I'm good, thanks!\n\nClass Diagram:\nclassDiagram\n    class Animal {\n        +String name\n        +int age\n        +makeSound()\n    }\n    class Dog {\n        +String breed\n        +bark()\n    }\n    Animal <|-- Dog\n\nState Diagram:\nstateDiagram-v2\n    [*] --> Idle\n    --> Processing: Start\n    Processing --> Success: Complete\n    Processing --> Error: Fail\n    Success --> [*]\n    Error --> Idle: Retry\n\nER Diagram:\nerDiagram\n    CUSTOMER ||--o{ ORDER : places\n    ORDER ||--|{ LINE-ITEM : contains\n    CUSTOMER }|..|{ DELIVERY-ADDRESS : uses\n\nPie Chart:\npie title Pets Distribution\n    \"Dogs\" : 386\n    \"Cats\" : 85\n    \"Rabbits\" : 15\n\nGit Graph:\ngitGraph\n    commit\n    branch develop\n    checkout develop\n    commit\n    checkout main\n    merge develop\n    commit\n\nRemember: Ensure your mermaid syntax is valid. The diagram will be rendered exactly as specified.",
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
		return "", errors.New("mermaid_code parameter is required")
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
		return "", errors.New("title exceeds maximum length of 100 characters")
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

// executeMermaidDiagram executes mermaid-cli via Docker to generate the diagram.
func executeMermaidDiagram(mermaidCode, title, outputType string) (string, error) {
	// Ensure charts directory exists
	chartsDir := "assets/charts"
	if err := os.MkdirAll(chartsDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create charts directory: %w", err)
	}

	// Generate unique filename
	hash := fmt.Sprintf("%x", md5.Sum([]byte(mermaidCode+title+strconv.FormatInt(time.Now().UnixNano(), 10))))
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
	// command: docker run --rm -v /path/to/charts:/data minlag/mermaid-cli -i input.mmd -o output.png
	cmd := exec.Command(
		"docker", "run", "--rm",
		"-u", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-v", absChartsDir+":/data",
		"minlag/mermaid-cli",
		"-i", "/data/"+inputFileName,
		"-o", "/data/"+outputFileName,
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
