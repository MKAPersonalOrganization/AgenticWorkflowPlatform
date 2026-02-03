package tools

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"school-with-ai/foundation/agent"
)

func TestMeanTool(t *testing.T) {
	t.Parallel()
	tool := &MeanTool{}
	args := map[string]any{
		"data": []float64{1, 2, 3, 4, 5},
	}
	argsBytes, _ := json.Marshal(args)
	tCtx := &agent.ToolContext{
		Context:   context.Background(),
		Arguments: string(argsBytes),
	}

	result, err := tool.Execute(context.Background(), tCtx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var resMap map[string]any
	if err := json.Unmarshal([]byte(result.Output), &resMap); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}

	if resMap["result"] != 3.0 {
		t.Errorf("Expected 3.0, got %v", resMap["result"])
	}
}

func TestMedianTool(t *testing.T) {
	t.Parallel()
	tool := &MedianTool{}
	args := map[string]any{
		"data": []float64{1, 2, 3, 4, 5},
	}
	argsBytes, _ := json.Marshal(args)
	tCtx := &agent.ToolContext{
		Context:   context.Background(),
		Arguments: string(argsBytes),
	}

	result, err := tool.Execute(context.Background(), tCtx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var resMap map[string]any
	if err := json.Unmarshal([]byte(result.Output), &resMap); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}

	if resMap["result"] != 3.0 {
		t.Errorf("Expected 3.0, got %v", resMap["result"])
	}
}

func TestPearsonCorrelationTool(t *testing.T) {
	t.Parallel()
	tool := &PearsonCorrelationTool{}
	args := map[string]any{
		"data1": []float64{1, 2, 3, 4, 5},
		"data2": []float64{2, 4, 6, 8, 10},
	}
	argsBytes, _ := json.Marshal(args)
	tCtx := &agent.ToolContext{
		Context:   context.Background(),
		Arguments: string(argsBytes),
	}

	result, err := tool.Execute(context.Background(), tCtx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var resMap map[string]any
	if err := json.Unmarshal([]byte(result.Output), &resMap); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}

	corr := resMap["result"].(float64)
	if math.Abs(corr-1.0) > 1e-9 {
		t.Errorf("Expected 1.0, got %v", corr)
	}
}

func TestNormalPdfTool(t *testing.T) {
	t.Parallel()
	tool := &NormalPdfTool{}
	args := map[string]any{
		"x":     0.0,
		"loc":   0.0,
		"scale": 1.0,
	}
	argsBytes, _ := json.Marshal(args)
	tCtx := &agent.ToolContext{
		Context:   context.Background(),
		Arguments: string(argsBytes),
	}

	result, err := tool.Execute(context.Background(), tCtx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var resMap map[string]any
	if err := json.Unmarshal([]byte(result.Output), &resMap); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}

	pdf := resMap["result"].(float64)
	expected := 0.3989422804014327
	if math.Abs(pdf-expected) > 1e-5 {
		t.Errorf("Expected ~%v, got %v", expected, pdf)
	}
}

func TestLinearRegressionTool(t *testing.T) {
	t.Parallel()
	tool := &LinearRegressionTool{}
	args := map[string]any{
		"x": []float64{1, 2, 3},
		"y": []float64{2, 4, 6},
	}
	argsBytes, _ := json.Marshal(args)
	tCtx := &agent.ToolContext{
		Context:   context.Background(),
		Arguments: string(argsBytes),
	}

	result, err := tool.Execute(context.Background(), tCtx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var resMap map[string]any
	if err := json.Unmarshal([]byte(result.Output), &resMap); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}

	regRaw := resMap["regression"].([]any)
	if len(regRaw) != 3 {
		t.Errorf("Expected 3 points, got %d", len(regRaw))
	}
}

func TestSampleTool(t *testing.T) {
	t.Parallel()
	tool := &SampleTool{}
	args := map[string]any{
		"data":    []float64{1, 2, 3, 4, 5, 6},
		"takeNum": 3,
	}
	argsBytes, _ := json.Marshal(args)
	tCtx := &agent.ToolContext{
		Context:   context.Background(),
		Arguments: string(argsBytes),
	}

	result, err := tool.Execute(context.Background(), tCtx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var resMap map[string]any
	if err := json.Unmarshal([]byte(result.Output), &resMap); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}

	sampleRaw := resMap["result"].([]any)
	if len(sampleRaw) != 3 {
		t.Errorf("Expected 3 samples, got %d", len(sampleRaw))
	}
}
