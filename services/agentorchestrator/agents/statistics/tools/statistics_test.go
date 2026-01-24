package tools

import (
	"context"
	"math"
	"testing"

	"school-with-ai/foundation/agentsdk/core"
)

func TestMeanTool(t *testing.T) {
	t.Parallel()
	tool := &MeanTool{}
	args := map[string]any{
		"data": []float64{1, 2, 3, 4, 5},
	}
	result, err := tool.Invoke(core.NewToolContext(context.Background(), "", "", "", nil), args)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	resMap := result.(map[string]any)
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
	result, err := tool.Invoke(core.NewToolContext(context.Background(), "", "", "", nil), args)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	resMap := result.(map[string]any)
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
	result, err := tool.Invoke(core.NewToolContext(context.Background(), "", "", "", nil), args)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	resMap := result.(map[string]any)
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
	result, err := tool.Invoke(core.NewToolContext(context.Background(), "", "", "", nil), args)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	resMap := result.(map[string]any)
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
	result, err := tool.Invoke(core.NewToolContext(context.Background(), "", "", "", nil), args)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	resMap := result.(map[string]any)
	reg := resMap["regression"].([]map[string]float64)
	if len(reg) != 3 {
		t.Errorf("Expected 3 points, got %d", len(reg))
	}
}

func TestSampleTool(t *testing.T) {
	t.Parallel()
	tool := &SampleTool{}
	args := map[string]any{
		"data":    []float64{1, 2, 3, 4, 5, 6},
		"takeNum": 3,
	}
	result, err := tool.Invoke(core.NewToolContext(context.Background(), "", "", "", nil), args)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	resMap := result.(map[string]any)
	sample := resMap["result"].([]float64)
	if len(sample) != 3 {
		t.Errorf("Expected 3 samples, got %d", len(sample))
	}
}
