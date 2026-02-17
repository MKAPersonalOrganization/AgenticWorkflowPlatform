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
	mockStore := &MockArtifactStore{}
	tool := NewPlotTool(mockStore)

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
		Raw:       json.RawMessage(argBytes),
		UserID:    "test-user",
		SessionID: "test-session",
	}

	result, err := tool.Execute(context.Background(), toolArgs)
	if err != nil {
		t.Fatalf("failed to invoke plot tool: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].(string)), &response); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("expected success to be true")
	}

	if !mockStore.SaveCalled {
		t.Error("expected ArtifactStore.Save to be called")
	}

	if !strings.Contains(response["image_url"].(string), "/artifacts/") {
		t.Errorf("expected artifact URL, got: %s", response["image_url"])
	}

	// Local file should be cleaned up
	imagePath := response["image_path"].(string)
	if _, err := os.Stat(imagePath); !os.IsNotExist(err) {
		t.Errorf("expected local image file to be cleaned up from %s", imagePath)
	}
}

func TestPlotTool_MultipleFunctions(t *testing.T) {
	t.Parallel()
	mockStore := &MockArtifactStore{}
	tool := NewPlotTool(mockStore)

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
		Raw:       json.RawMessage(argBytes),
		UserID:    "test-user",
		SessionID: "test-session",
	}

	result, err := tool.Execute(context.Background(), toolArgs)
	if err != nil {
		t.Fatalf("failed to create plot: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].(string)), &response); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if !mockStore.SaveCalled {
		t.Error("expected ArtifactStore.Save to be called")
	}
}

func TestPlotTool_QuadraticWithRoots(t *testing.T) {
	t.Parallel()
	mockStore := &MockArtifactStore{}
	tool := NewPlotTool(mockStore)

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
		Raw:       json.RawMessage(argBytes),
		UserID:    "test-user",
		SessionID: "test-session",
	}

	result, err := tool.Execute(context.Background(), toolArgs)
	if err != nil {
		t.Fatalf("failed to create plot: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].(string)), &response); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if !mockStore.SaveCalled {
		t.Error("expected ArtifactStore.Save to be called")
	}
}

func TestPlotTool_InlineDataPlot(t *testing.T) {
	t.Parallel()
	mockStore := &MockArtifactStore{}
	tool := NewPlotTool(mockStore)

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
		Raw:       json.RawMessage(argBytes),
		UserID:    "test-user",
		SessionID: "test-session",
	}

	result, err := tool.Execute(context.Background(), toolArgs)
	if err != nil {
		t.Fatalf("failed to create plot: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].(string)), &response); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if !mockStore.SaveCalled {
		t.Error("expected ArtifactStore.Save to be called")
	}
}

func TestPlotTool_3DSurface(t *testing.T) {
	t.Parallel()
	mockStore := &MockArtifactStore{}
	tool := NewPlotTool(mockStore)

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
		Raw:       json.RawMessage(argBytes),
		UserID:    "test-user",
		SessionID: "test-session",
	}

	result, err := tool.Execute(context.Background(), toolArgs)
	if err != nil {
		t.Fatalf("failed to create plot: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].(string)), &response); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if !mockStore.SaveCalled {
		t.Error("expected ArtifactStore.Save to be called")
	}
}

func TestPlotTool_ValidationMissingScript(t *testing.T) {
	t.Parallel()
	mockStore := &MockArtifactStore{}
	tool := NewPlotTool(mockStore)
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

	result, _ := tool.Execute(context.Background(), toolArgs)

	if result.Error == nil {
		t.Error("expected error for missing gnuplot_script parameter")
	} else if !strings.Contains(result.Error.Error(), "gnuplot_script parameter is required") {
		t.Errorf("expected error about missing script, got: %v", result.Error)
	}
}

func TestPlotTool_ValidationSetTerminal(t *testing.T) {
	t.Parallel()
	mockStore := &MockArtifactStore{}
	tool := NewPlotTool(mockStore)
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

	result, _ := tool.Execute(context.Background(), toolArgs)

	if result.Error == nil {
		t.Error("expected error for script containing 'set terminal'")
	} else if !strings.Contains(result.Error.Error(), "should not contain 'set terminal'") {
		t.Errorf("expected error about set terminal, got: %v", result.Error)
	}
}

func TestPlotTool_ValidationSetOutput(t *testing.T) {
	t.Parallel()
	mockStore := &MockArtifactStore{}
	tool := NewPlotTool(mockStore)
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

	result, _ := tool.Execute(context.Background(), toolArgs)

	if result.Error == nil {
		t.Error("expected error for script containing 'set output'")
	} else if !strings.Contains(result.Error.Error(), "should not contain 'set output'") {
		t.Errorf("expected error about set output, got: %v", result.Error)
	}
}

func TestPlotTool_ValidationTitleTooLong(t *testing.T) {
	t.Parallel()
	mockStore := &MockArtifactStore{}
	tool := NewPlotTool(mockStore)
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

	result, _ := tool.Execute(context.Background(), toolArgs)

	if result.Error == nil {
		t.Error("expected error for title too long")
	} else if !strings.Contains(result.Error.Error(), "title exceeds maximum length") {
		t.Errorf("expected error about title length, got: %v", result.Error)
	}
}

func TestPlotTool_ParametersSchema(t *testing.T) {
	t.Parallel()
	mockStore := &MockArtifactStore{}
	tool := NewPlotTool(mockStore)
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
