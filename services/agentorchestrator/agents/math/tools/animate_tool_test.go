package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
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

	args := AnimateArgs{
		GnuplotScript: script,
		Format:        "gif",
		Delay:         10,
		Title:         "Wave propagation animation",
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal args: %v", err)
	}

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to invoke animate tool: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.Result), &response); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

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

	args := AnimateArgs{
		GnuplotScript: script,
		Format:        "gif",
		Delay:         50,
		Title:         "Rotating 3D surface",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to create animation: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.Result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

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

	args := AnimateArgs{
		GnuplotScript: script,
		Format:        "webp",
		Delay:         30,
		Title:         "Amplitude variation",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed to create webp animation: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.Result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

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
	args := AnimateArgs{
		Format: "gif",
		Title:  "Test",
	}

	argsJSON, _ := json.Marshal(args)
	_, err := tool.Invoke(context.Background(), string(argsJSON))

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
	args := AnimateArgs{
		GnuplotScript: "plot sin(x)",
		Format:        "mp4", // Invalid format
	}

	argsJSON, _ := json.Marshal(args)
	_, err := tool.Invoke(context.Background(), string(argsJSON))

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
	args := AnimateArgs{
		GnuplotScript: "set terminal gif\ndo for [i=1:10] { plot sin(x + i) }",
		Format:        "gif",
	}

	argsJSON, _ := json.Marshal(args)
	_, err := tool.Invoke(context.Background(), string(argsJSON))

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

	args := AnimateArgs{
		GnuplotScript: script,
		// Format and Delay not specified - should use defaults
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("failed with defaults: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.Result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["format"].(string) != "gif" {
		t.Errorf("expected default format 'gif', got: %s", response["format"])
	}

	if int(response["delay"].(float64)) != 50 {
		t.Errorf("expected default delay 50, got: %d", int(response["delay"].(float64)))
	}

	animationPath := response["animation_path"].(string)
	defer func() { _ = os.Remove(animationPath) }()
}
