package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDescriptiveStatsTool_Mean(t *testing.T) {
	t.Parallel()
	tool := NewDescriptiveStatsTool()
	args := map[string]any{
		"operation": "mean",
		"data":      []float64{1, 2, 3, 4, 5},
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response["result"] != 3.0 {
		t.Errorf("Expected mean 3.0, got %v", response["result"])
	}
}

func TestDescriptiveStatsTool_Describe(t *testing.T) {
	t.Parallel()
	tool := NewDescriptiveStatsTool()
	args := map[string]any{
		"operation": "describe",
		"data":      []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	resultMap := response["result"].(map[string]any)

	if resultMap["count"] != float64(10) {
		t.Errorf("Expected count 10, got %v", resultMap["count"])
	}
	if resultMap["mean"] != 5.5 {
		t.Errorf("Expected mean 5.5, got %v", resultMap["mean"])
	}
}

func TestCorrelationTool_Pearson(t *testing.T) {
	t.Parallel()
	tool := NewCorrelationTool()
	args := map[string]any{
		"operation": "pearson",
		"data1":     []float64{1, 2, 3, 4, 5},
		"data2":     []float64{2, 4, 6, 8, 10},
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	correlation := response["result"].(float64)

	if correlation < 0.99 || correlation > 1.01 {
		t.Errorf("Expected perfect correlation ~1.0, got %v", correlation)
	}
}

func TestDistanceMetricsTool_Euclidean(t *testing.T) {
	t.Parallel()
	tool := NewDistanceMetricsTool()
	args := map[string]any{
		"operation": "euclidean_distance",
		"point1":    []float64{0, 0, 0},
		"point2":    []float64{3, 4, 0},
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	distance := response["distance"].(float64)

	if distance != 5.0 {
		t.Errorf("Expected distance 5.0, got %v", distance)
	}
}

func TestSamplingTool_QuartileOutliers(t *testing.T) {
	t.Parallel()
	tool := NewSamplingTool()
	args := map[string]any{
		"operation": "quartile_outliers",
		"data":      []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 100},
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	resultMap := response["result"].(map[string]any)

	if resultMap["extreme"] == nil {
		t.Error("Expected outliers to be detected")
	}
}

func TestDataTransformTool_Sigmoid(t *testing.T) {
	t.Parallel()
	tool := NewDataTransformTool()
	args := map[string]any{
		"operation": "sigmoid",
		"data":      []float64{0, 1, -1},
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["result"] == nil {
		t.Error("Expected sigmoid transformation result")
	}
}

func TestDataTransformTool_Round(t *testing.T) {
	t.Parallel()
	tool := NewDataTransformTool()
	args := map[string]any{
		"operation": "round",
		"value":     3.14159,
		"places":    2,
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["result"] != 3.14 {
		t.Errorf("Expected 3.14, got %v", response["result"])
	}
}

func TestNormalDistributionTool_Pdf(t *testing.T) {
	t.Parallel()
	tool := NewNormalDistributionTool()
	args := map[string]any{
		"operation": "norm_pdf",
		"x":         0.0,
		"loc":       0.0,
		"scale":     1.0,
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	pdf := response["result"].(float64)
	expected := 0.3989422804014327 // 1/sqrt(2*pi)

	if pdf < expected-0.0001 || pdf > expected+0.0001 {
		t.Errorf("Expected PDF ~%v, got %v", expected, pdf)
	}
}

func TestNormalDistributionTool_Fit(t *testing.T) {
	t.Parallel()
	tool := NewNormalDistributionTool()
	args := map[string]any{
		"operation": "norm_fit",
		"data":      []float64{1, 2, 3, 4, 5},
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	resultMap := response["result"].(map[string]any)

	if resultMap["loc"] != 3.0 {
		t.Errorf("Expected loc 3.0, got %v", resultMap["loc"])
	}
}

func TestGeometricDistributionTool_ExpGeom(t *testing.T) {
	t.Parallel()
	tool := NewGeometricDistributionTool()
	args := map[string]any{
		"operation": "exp_geom",
		"p":         0.5,
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["result"] != 2.0 {
		t.Errorf("Expected expectation 2.0, got %v", response["result"])
	}
}

func TestRegressionTool_Linear(t *testing.T) {
	t.Parallel()
	tool := NewRegressionTool()
	args := map[string]any{
		"type": "linear",
		"x":    []float64{1, 2, 3, 4, 5},
		"y":    []float64{2, 4, 6, 8, 10},
	}
	argsJSON, _ := json.Marshal(args)

	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["type"] != "linear" {
		t.Errorf("Expected linear regression type, got %v", response["type"])
	}
}
