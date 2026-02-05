package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"school-with-ai/foundation/agent"
)

func TestPlotTool_SimpleFunctionPlot(t *testing.T) {
	t.Parallel()
	tool := NewPlotTool()

	script := `set title 'Sine Wave'
set xlabel 'x'
set ylabel 'sin(x)'
set grid
set xrange [-6.28:6.28]
plot sin(x) with lines linewidth 2 title 'y = sin(x)'`

	args := map[string]any{
		"gnuplot_script": script,
		"title":          "Sine wave test",
	}

	argBytes, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	toolArgs := agent.Arguments{
		Raw: json.RawMessage(argBytes),
	}

	result, err := tool.Execute(context.Background(), toolArgs)
	if err != nil {
		t.Fatalf("failed to invoke plot tool: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	response := result.Content[0].(map[string]any)

	if !response["success"].(bool) {
		t.Error("expected success to be true")
	}

	imagePath := response["image_path"].(string)
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		t.Errorf("expected image file to exist at %s", imagePath)
	} else {
		defer func() { _ = os.Remove(imagePath) }()
		t.Logf("Sine plot created successfully: %s", response["image_url"])
	}
}

func TestPlotTool_MultipleFunctions(t *testing.T) {
	t.Parallel()
	tool := NewPlotTool()

	script := `set title 'Trigonometric Functions'
set grid
set xrange [-6.28:6.28]
plot sin(x) title 'sin(x)', cos(x) title 'cos(x)', tan(x) title 'tan(x)'`

	args := map[string]any{
		"gnuplot_script": script,
		"title":          "Multiple trig functions",
	}

	argBytes, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	toolArgs := agent.Arguments{
		Raw: json.RawMessage(argBytes),
	}

	result, err := tool.Execute(context.Background(), toolArgs)
	if err != nil {
		t.Fatalf("failed to create plot: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	response := result.Content[0].(map[string]any)

	imagePath := response["image_path"].(string)
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		t.Errorf("expected image file to exist at %s", imagePath)
	} else {
		defer func() { _ = os.Remove(imagePath) }()
		t.Logf("Multiple functions plot created: %s", response["image_url"])
	}
}

func TestPlotTool_QuadraticWithRoots(t *testing.T) {
	t.Parallel()
	tool := NewPlotTool()

	script := `set title 'Quadratic Function with Roots'
set grid
set xrange [-2:6]
set yrange [-5:10]
plot x**2 - 4*x + 3 with lines title 'y = x² - 4x + 3', 0 with lines dashtype 2 notitle`

	args := map[string]any{
		"gnuplot_script": script,
		"title":          "Quadratic function",
	}

	argBytes, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	toolArgs := agent.Arguments{
		Raw: json.RawMessage(argBytes),
	}

	result, err := tool.Execute(context.Background(), toolArgs)
	if err != nil {
		t.Fatalf("failed to create plot: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	response := result.Content[0].(map[string]any)

	imagePath := response["image_path"].(string)
	defer func() { _ = os.Remove(imagePath) }()
	t.Logf("Quadratic plot created: %s", response["image_url"])
}

func TestPlotTool_InlineDataPlot(t *testing.T) {
	t.Parallel()
	tool := NewPlotTool()

	script := `set title 'Scatter Plot with Data'
set grid
plot '-' with points pointtype 7 pointsize 1.5 title 'Data Points'
1 2.3
2 4.1
3 5.8
4 7.2
5 9.1
e`

	args := map[string]any{
		"gnuplot_script": script,
		"title":          "Scatter data",
	}

	argBytes, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	toolArgs := agent.Arguments{
		Raw: json.RawMessage(argBytes),
	}

	result, err := tool.Execute(context.Background(), toolArgs)
	if err != nil {
		t.Fatalf("failed to create plot: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	response := result.Content[0].(map[string]any)

	imagePath := response["image_path"].(string)
	defer func() { _ = os.Remove(imagePath) }()
	t.Logf("Scatter plot created: %s", response["image_url"])
}

func TestPlotTool_3DSurface(t *testing.T) {
	t.Parallel()
	tool := NewPlotTool()

	script := `set title '3D Surface Plot'
set grid
splot x**2 + y**2 with lines`

	args := map[string]any{
		"gnuplot_script": script,
		"title":          "3D surface",
	}

	argBytes, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	toolArgs := agent.Arguments{
		Raw: json.RawMessage(argBytes),
	}

	result, err := tool.Execute(context.Background(), toolArgs)
	if err != nil {
		t.Fatalf("failed to create plot: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	response := result.Content[0].(map[string]any)

	imagePath := response["image_path"].(string)
	defer func() {
		err := os.Remove(imagePath)
		if err != nil {
			t.Logf("Failed to remove test image: %v", err)
		}
	}()
	t.Logf("3D surface plot created: %s", response["image_url"])
}

func TestPlotTool_ValidationMissingScript(t *testing.T) {
	t.Parallel()
	tool := NewPlotTool()
	args := map[string]any{
		// Missing gnuplot_script
		"title": "Test",
	}

	argBytes, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	toolArgs := agent.Arguments{
		Raw: json.RawMessage(argBytes),
	}

	_, err = tool.Execute(context.Background(), toolArgs)

	if err == nil {
		t.Error("expected error for missing gnuplot_script parameter")
	}
	if !strings.Contains(err.Error(), "gnuplot_script parameter is required") {
		t.Errorf("expected error about missing script, got: %v", err)
	}
}

func TestPlotTool_ValidationSetTerminal(t *testing.T) {
	t.Parallel()
	tool := NewPlotTool()
	args := map[string]any{
		"gnuplot_script": "set terminal png\nplot sin(x)",
		"title":          "Test",
	}

	argBytes, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	toolArgs := agent.Arguments{
		Raw: json.RawMessage(argBytes),
	}

	_, err = tool.Execute(context.Background(), toolArgs)

	if err == nil {
		t.Error("expected error for script containing 'set terminal'")
	}
	if !strings.Contains(err.Error(), "should not contain 'set terminal'") {
		t.Errorf("expected error about set terminal, got: %v", err)
	}
}

func TestPlotTool_ValidationSetOutput(t *testing.T) {
	t.Parallel()
	tool := NewPlotTool()
	args := map[string]any{
		"gnuplot_script": "set output 'test.png'\nplot sin(x)",
		"title":          "Test",
	}

	argBytes, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	toolArgs := agent.Arguments{
		Raw: json.RawMessage(argBytes),
	}

	_, err = tool.Execute(context.Background(), toolArgs)

	if err == nil {
		t.Error("expected error for script containing 'set output'")
	}
	if !strings.Contains(err.Error(), "should not contain 'set output'") {
		t.Errorf("expected error about set output, got: %v", err)
	}
}

func TestPlotTool_ValidationTitleTooLong(t *testing.T) {
	t.Parallel()
	tool := NewPlotTool()
	longTitle := strings.Repeat("a", 101) // 101 characters (exceeds 100 limit)
	args := map[string]any{
		"gnuplot_script": "plot sin(x)",
		"title":          longTitle,
	}

	argBytes, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	toolArgs := agent.Arguments{
		Raw: json.RawMessage(argBytes),
	}

	_, err = tool.Execute(context.Background(), toolArgs)

	if err == nil {
		t.Error("expected error for title too long")
	}
	if !strings.Contains(err.Error(), "title exceeds maximum length") {
		t.Errorf("expected error about title length, got: %v", err)
	}
}


func TestPlotTool_ParametersSchema(t *testing.T) {
t.Parallel()
tool := NewPlotTool()
schema := tool.Schema()

// Verify schema structure
if schema.Type != "object" {
t.Error("expected type to be 'object'")
}

if schema.Properties == nil {
t.Fatal("expected properties to be defined")
}

// Check gnuplot_script property exists
if _, ok := schema.Properties["gnuplot_script"]; !ok {
t.Error("expected gnuplot_script property")
}

// Check required fields
if len(schema.Required) != 2 {
t.Errorf("expected required to contain 2 fields, got: %v", schema.Required)
}
if schema.Required[0] != "gnuplot_script" || schema.Required[1] != "title" {
t.Errorf("expected required to contain 'gnuplot_script' and 'title', got: %v", schema.Required)
}
}
