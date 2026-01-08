package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestMermaidDiagramTool_SimpleFlowchart(t *testing.T) {
	tool := NewMermaidDiagramTool()

	mermaidCode := `graph TD
    A[Start] --> B{Decision}
    B -->|Yes| C[Action 1]
    B -->|No| D[Action 2]
    C --> E[End]
    D --> E`

	args := MermaidDiagramArgs{
		MermaidCode: mermaidCode,
		Title:       "Simple flowchart test",
		OutputType:  "png",
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to invoke mermaid diagram tool: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("expected success to be true")
	}

	imagePath := response["image_path"].(string)
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		t.Errorf("expected image file to exist at %s", imagePath)
	} else {
		defer func() { _ = os.Remove(imagePath) }()
		t.Logf("Simple flowchart created: %s", response["image_url"])
	}
}

func TestMermaidDiagramTool_SequenceDiagram(t *testing.T) {
	tool := NewMermaidDiagramTool()

	mermaidCode := `sequenceDiagram
    participant Alice
    participant Bob
    Alice->>Bob: Hello Bob, how are you?
    Bob-->>Alice: Great!
    Alice-)Bob: See you later!`

	args := MermaidDiagramArgs{
		MermaidCode: mermaidCode,
		Title:       "Sequence diagram test",
		OutputType:  "png",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to create sequence diagram: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	imagePath := response["image_path"].(string)
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		t.Errorf("expected image file to exist at %s", imagePath)
	} else {
		defer func() { _ = os.Remove(imagePath) }()
		t.Logf("Sequence diagram created: %s", response["image_url"])
	}
}

func TestMermaidDiagramTool_ClassDiagram(t *testing.T) {
	tool := NewMermaidDiagramTool()

	mermaidCode := `classDiagram
    class Animal {
        +String name
        +int age
        +makeSound()
    }
    class Dog {
        +String breed
        +bark()
    }
    class Cat {
        +Boolean indoor
        +meow()
    }
    Animal <|-- Dog
    Animal <|-- Cat`

	args := MermaidDiagramArgs{
		MermaidCode: mermaidCode,
		Title:       "Class diagram test",
		OutputType:  "png",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to create class diagram: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	imagePath := response["image_path"].(string)
	defer func() { _ = os.Remove(imagePath) }()
	t.Logf("Class diagram created: %s", response["image_url"])
}

func TestMermaidDiagramTool_StateDiagram(t *testing.T) {
	tool := NewMermaidDiagramTool()

	mermaidCode := "stateDiagram-v2\n    [*] --> Idle\n    --> Processing: Start Task\n    Processing --> Success: Complete\n    Processing --> Error: Fail\n    Success --> [*]\n    Error --> Idle: Retry\n    Error --> [*]: Give Up"

	args := MermaidDiagramArgs{
		MermaidCode: mermaidCode,
		Title:       "State diagram test",
		OutputType:  "png",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to create state diagram: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	imagePath := response["image_path"].(string)
	defer func() { _ = os.Remove(imagePath) }()
	t.Logf("State diagram created: %s", response["image_url"])
}

func TestMermaidDiagramTool_SVGOutput(t *testing.T) {
	tool := NewMermaidDiagramTool()

	mermaidCode := `graph LR
    A[Start] --> B[Process]
    B --> C[End]`

	args := MermaidDiagramArgs{
		MermaidCode: mermaidCode,
		Title:       "SVG output test",
		OutputType:  "svg",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to create SVG diagram: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["output_type"].(string) != "svg" {
		t.Errorf("expected output_type to be 'svg', got %s", response["output_type"])
	}

	imagePath := response["image_path"].(string)
	if !strings.HasSuffix(imagePath, ".svg") {
		t.Errorf("expected SVG file extension, got %s", imagePath)
	}

	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		t.Errorf("expected SVG file to exist at %s", imagePath)
	} else {
		defer func() { _ = os.Remove(imagePath) }()
		t.Logf("SVG diagram created: %s", response["image_url"])
	}
}

func TestMermaidDiagramTool_ERDiagram(t *testing.T) {
	tool := NewMermaidDiagramTool()

	mermaidCode := `erDiagram
    CUSTOMER ||--o{ ORDER : places
    ORDER ||--|{ LINE-ITEM : contains
    CUSTOMER }|..|{ DELIVERY-ADDRESS : uses
    CUSTOMER {
        string name
        string email
    }
    ORDER {
        int orderNumber
        date orderDate
    }`

	args := MermaidDiagramArgs{
		MermaidCode: mermaidCode,
		Title:       "ER diagram test",
		OutputType:  "png",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to create ER diagram: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	imagePath := response["image_path"].(string)
	defer func() { _ = os.Remove(imagePath) }()
	t.Logf("ER diagram created: %s", response["image_url"])
}

func TestMermaidDiagramTool_ValidationMissingCode(t *testing.T) {
	tool := NewMermaidDiagramTool()

	args := MermaidDiagramArgs{
		MermaidCode: "",
		Title:       "Test",
	}

	argsJSON, _ := json.Marshal(args)
	_, err := tool.Invoke(context.Background(), string(argsJSON))

	if err == nil {
		t.Error("expected error for missing mermaid_code")
	}
	if !strings.Contains(err.Error(), "mermaid_code parameter is required") {
		t.Errorf("expected 'mermaid_code parameter is required' error, got: %v", err)
	}
}

func TestMermaidDiagramTool_ValidationInvalidOutputType(t *testing.T) {
	tool := NewMermaidDiagramTool()

	args := MermaidDiagramArgs{
		MermaidCode: "graph TD\nA-->B",
		Title:       "Test",
		OutputType:  "pdf",
	}

	argsJSON, _ := json.Marshal(args)
	_, err := tool.Invoke(context.Background(), string(argsJSON))

	if err == nil {
		t.Error("expected error for invalid output_type")
	}
	if !strings.Contains(err.Error(), "output_type must be") {
		t.Errorf("expected output_type validation error, got: %v", err)
	}
}

func TestMermaidDiagramTool_ValidationTitleTooLong(t *testing.T) {
	tool := NewMermaidDiagramTool()

	longTitle := strings.Repeat("a", 101)
	args := MermaidDiagramArgs{
		MermaidCode: "graph TD\nA-->B",
		Title:       longTitle,
	}

	argsJSON, _ := json.Marshal(args)
	_, err := tool.Invoke(context.Background(), string(argsJSON))

	if err == nil {
		t.Error("expected error for title too long")
	}
	if !strings.Contains(err.Error(), "title exceeds maximum length") {
		t.Errorf("expected title length error, got: %v", err)
	}
}

func TestMermaidDiagramTool_ParametersSchema(t *testing.T) {
	tool := NewMermaidDiagramTool()
	params := tool.Parameters()

	// Check that it's an object type
	if params["type"] != "object" {
		t.Errorf("expected type 'object', got %v", params["type"])
	}

	// Check that required fields are present
	props := params["properties"].(map[string]any)
	if _, ok := props["mermaid_code"]; !ok {
		t.Error("expected 'mermaid_code' property to be present")
	}
	if _, ok := props["title"]; !ok {
		t.Error("expected 'title' property to be present")
	}
	if _, ok := props["output_type"]; !ok {
		t.Error("expected 'output_type' property to be present")
	}

	// Check required array
	required := params["required"].([]string)
	if len(required) != 1 || required[0] != "mermaid_code" {
		t.Errorf("expected required to be ['mermaid_code'], got %v", required)
	}
}

func TestMermaidDiagramTool_PieChart(t *testing.T) {
	tool := NewMermaidDiagramTool()

	mermaidCode := `pie title Programming Languages Usage
    "Go" : 45
    "Python" : 30
    "JavaScript" : 15
    "Rust" : 10`

	args := MermaidDiagramArgs{
		MermaidCode: mermaidCode,
		Title:       "Pie chart test",
		OutputType:  "png",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to create pie chart: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	imagePath := response["image_path"].(string)
	defer func() { _ = os.Remove(imagePath) }()
	t.Logf("Pie chart created: %s", response["image_url"])
}

func TestMermaidDiagramTool_ComplexFlowchart(t *testing.T) {
	tool := NewMermaidDiagramTool()

	mermaidCode := "graph TD\n    Start[Start Problem] --> Read[Read Question]\n    Read --> Identify{Identify Type}\n    Identify -->|Algebra| Alg[Use Algebra Tools]\n    Identify -->|Calculus| Calc[Use Calculus Tools]\n    Identify -->|Geometry| Geo[Use Geometry Tools]\n    Alg --> Solve[Solve Problem]\n    Calc --> Solve\n    Geo --> Solve\n    --> Verify{Verify Solution}\n    Verify -->|Correct| End[Present Answer]\n    Verify -->|Incorrect| Read\n    End --> Finish[Complete]"

	args := MermaidDiagramArgs{
		MermaidCode: mermaidCode,
		Title:       "Math problem solving workflow",
		OutputType:  "png",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to create complex flowchart: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	imagePath := response["image_path"].(string)
	defer func() { _ = os.Remove(imagePath) }()

	// Verify file size is reasonable (should be > 1KB for a complex diagram)
	info, err := os.Stat(imagePath)
	if err != nil {
		t.Fatalf("failed to stat image file: %v", err)
	}
	if info.Size() < 1000 {
		t.Errorf("image file seems too small: %d bytes", info.Size())
	}

	t.Logf("Complex flowchart created: %s (size: %d bytes)", response["image_url"], info.Size())
}

func TestMermaidDiagramTool_DefaultOutputType(t *testing.T) {
	tool := NewMermaidDiagramTool()

	mermaidCode := `graph LR
    A[Input] --> B[Process]
    B --> C[Output]`

	args := MermaidDiagramArgs{
		MermaidCode: mermaidCode,
		Title:       "Default output type test",
		// OutputType not specified - should default to "png"
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to create diagram with default output type: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["output_type"].(string) != "png" {
		t.Errorf("expected default output_type to be 'png', got %s", response["output_type"])
	}

	imagePath := response["image_path"].(string)
	if !strings.HasSuffix(imagePath, ".png") {
		t.Errorf("expected PNG file extension with default output type, got %s", imagePath)
	}

	defer func() { _ = os.Remove(imagePath) }()
	t.Logf("Default output type (png) diagram created: %s", response["image_url"])
}

func TestMermaidDiagramTool_JSONResponseFormat(t *testing.T) {
	tool := NewMermaidDiagramTool()

	mermaidCode := `graph TD
    A --> B`

	args := MermaidDiagramArgs{
		MermaidCode: mermaidCode,
		Title:       "Response format test",
		OutputType:  "png",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to invoke tool: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify all expected fields are present
	expectedFields := []string{"success", "image_path", "image_url", "output_type", "message", "title"}
	for _, field := range expectedFields {
		if _, ok := response[field]; !ok {
			t.Errorf("expected field '%s' to be present in response", field)
		}
	}

	// Verify field types and values
	if success, ok := response["success"].(bool); !ok || !success {
		t.Error("expected success to be true")
	}

	if message, ok := response["message"].(string); !ok || message != "Diagram generated successfully" {
		t.Errorf("unexpected message: %v", response["message"])
	}

	imagePath := response["image_path"].(string)
	defer func() { _ = os.Remove(imagePath) }()
}
