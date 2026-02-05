package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"school-with-ai/foundation/agent"
)

func TestAnimateTool_WavePropagation(t *testing.T) {
	t.Parallel()
	tool := NewAnimateTool()

	script := `set xrange [0:10]
set yrange [-2:2]
set grid
set title 'Wave Propagation'
do for [i=0:40] {
  t = i * 0.5
  plot sin(x - t) with lines linewidth 2 title sprintf('t = %.1f', t)
}`

	args := map[string]any{
		"gnuplot_script": script,
		"format":         "gif",
		"delay":          10,
		"title":          "Wave propagation animation",
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
		t.Fatalf("failed to invoke animate tool: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	response := result.Content[0].(map[string]any)

	if !response["success"].(bool) {
		t.Error("expected success to be true")
	}

	animationPath := response["animation_path"].(string)
	if _, err := os.Stat(animationPath); os.IsNotExist(err) {
		t.Errorf("expected animation file to exist at %s", animationPath)
	} else {
		defer func() { _ = os.Remove(animationPath) }()
		t.Logf("Wave animation created successfully: %s", response["animation_url"])
	}
}

func TestAnimateTool_Rotating3D(t *testing.T) {
	t.Parallel()
	tool := NewAnimateTool()

	script := `unset border
unset tics
set view equal xyz
set pm3d
set title '3D Surface Rotation'
do for [ang=0:360:10] {
  set view 60, ang
  splot x**2 + y**2 with pm3d
}`

	args := map[string]any{
		"gnuplot_script": script,
		"format":         "gif",
		"delay":          50,
		"title":          "Rotating 3D surface",
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
		t.Fatalf("failed to create animation: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	response := result.Content[0].(map[string]any)

	animationPath := response["animation_path"].(string)
	defer func() { _ = os.Remove(animationPath) }()
	t.Logf("3D rotation animation created: %s", response["animation_url"])
}

func TestAnimateTool_WebPFormat(t *testing.T) {
	t.Parallel()
	tool := NewAnimateTool()

	script := `set xrange [-10:10]
set yrange [-5:5]
set grid
do for [i=1:30] {
  a = i * 0.1
  plot a*sin(x) with lines title sprintf('y = %.1f sin(x)', a)
}`

	args := map[string]any{
		"gnuplot_script": script,
		"format":         "webp",
		"delay":          30,
		"title":          "Amplitude variation",
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
		t.Fatalf("failed to create webp animation: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	response := result.Content[0].(map[string]any)

	animationPath := response["animation_path"].(string)
	if !strings.HasSuffix(animationPath, ".webp") {
		t.Errorf("expected .webp extension, got: %s", animationPath)
	}
	defer func() { _ = os.Remove(animationPath) }()
	t.Logf("WebP animation created: %s", response["animation_url"])
}

func TestAnimateTool_ValidationMissingScript(t *testing.T) {
	t.Parallel()
	tool := NewAnimateTool()
	args := map[string]any{
		"format": "gif",
		"title":  "Test",
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

func TestAnimateTool_ValidationInvalidFormat(t *testing.T) {
	t.Parallel()
	tool := NewAnimateTool()
	args := map[string]any{
		"gnuplot_script": "plot sin(x)",
		"format":         "mp4", // Invalid format
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
		t.Error("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "format must be 'gif' or 'webp'") {
		t.Errorf("expected error about invalid format, got: %v", err)
	}
}

func TestAnimateTool_ValidationSetTerminal(t *testing.T) {
	t.Parallel()
	tool := NewAnimateTool()
	args := map[string]any{
		"gnuplot_script": "set terminal gif\ndo for [i=1:10] { plot sin(x + i) }",
		"format":         "gif",
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

func TestAnimateTool_DefaultValues(t *testing.T) {
	t.Parallel()
	tool := NewAnimateTool()

	script := `do for [i=1:5] { plot sin(x + i) }`

	args := map[string]any{
		"gnuplot_script": script,
		// Format and Delay not specified - should use defaults
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
		t.Fatalf("failed with defaults: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	response := result.Content[0].(map[string]any)

	if response["format"].(string) != "gif" {
		t.Errorf("expected default format 'gif', got: %s", response["format"])
	}

	// JSON unmarshaling numbers into interface{} results in float64
	delay := response["delay"]
	var delayInt int
	if f, ok := delay.(float64); ok {
		delayInt = int(f)
	} else if i, ok := delay.(int); ok {
		delayInt = i
	} else {
		t.Errorf("unexpected type for delay: %T", delay)
	}

	if delayInt != 50 {
		t.Errorf("expected default delay 50, got: %v", delayInt)
	}

	animationPath := response["animation_path"].(string)
	defer func() { _ = os.Remove(animationPath) }()
}
