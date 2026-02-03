package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"school-with-ai/foundation/agent"

	"github.com/montanaflynn/stats"
)

func jsonResult(v any) (*agent.ToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &agent.ToolResult{Output: string(b)}, nil
}

var (
	ErrCorrelationRequiresData = errors.New("correlation requires data1 and data2")
	ErrPearsonRequiresData     = errors.New("pearson requires data1 and data2")
	ErrXYArrayLengthMismatch   = errors.New("x and y arrays must have the same length")
	ErrUnknownOperation        = errors.New("unknown operation")
	ErrUnknownRegressionType   = errors.New("unknown regression type")
)

// =============================================================================
// =============================================================================
// DESCRIPTIVE STATISTICS TOOLS
// =============================================================================

// MeanTool calculates the arithmetic mean.
type MeanTool struct{}

func (t *MeanTool) Name() string        { return "mean" }
func (t *MeanTool) Description() string { return "Calculate the arithmetic mean of a dataset" }
func (t *MeanTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
		},
		"required": []string{"data"},
	}
}

func (t *MeanTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data []float64 `json:"data"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result, err := stats.Mean(stats.Float64Data(params.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to calculate mean: %w", err)
	}
	return jsonResult(map[string]any{"result": result})
}

// MedianTool calculates the median.
type MedianTool struct{}

func (t *MedianTool) Name() string        { return "median" }
func (t *MedianTool) Description() string { return "Calculate the median of a dataset" }
func (t *MedianTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
		},
		"required": []string{"data"},
	}
}

func (t *MedianTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data []float64 `json:"data"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result, err := stats.Median(stats.Float64Data(params.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to calculate median: %w", err)
	}
	return jsonResult(map[string]any{"result": result})
}

// ModeTool calculates the mode.
type ModeTool struct{}

func (t *ModeTool) Name() string        { return "mode" }
func (t *ModeTool) Description() string { return "Calculate the mode(s) of a dataset" }
func (t *ModeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
		},
		"required": []string{"data"},
	}
}

func (t *ModeTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data []float64 `json:"data"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result, err := stats.Mode(stats.Float64Data(params.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to calculate mode: %w", err)
	}
	return jsonResult(map[string]any{"result": result})
}

// MinTool calculates the minimum.
type MinTool struct{}

func (t *MinTool) Name() string        { return "min" }
func (t *MinTool) Description() string { return "Find the minimum value in a dataset" }
func (t *MinTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
		},
		"required": []string{"data"},
	}
}

func (t *MinTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data []float64 `json:"data"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result, err := stats.Min(stats.Float64Data(params.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to calculate min: %w", err)
	}
	return jsonResult(map[string]any{"result": result})
}

// MaxTool calculates the maximum.
type MaxTool struct{}

func (t *MaxTool) Name() string        { return "max" }
func (t *MaxTool) Description() string { return "Find the maximum value in a dataset" }
func (t *MaxTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
		},
		"required": []string{"data"},
	}
}

func (t *MaxTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data []float64 `json:"data"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result, err := stats.Max(stats.Float64Data(params.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to calculate max: %w", err)
	}
	return jsonResult(map[string]any{"result": result})
}

// SumTool calculates the sum.
type SumTool struct{}

func (t *SumTool) Name() string        { return "sum" }
func (t *SumTool) Description() string { return "Calculate the sum of a dataset" }
func (t *SumTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
		},
		"required": []string{"data"},
	}
}

func (t *SumTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data []float64 `json:"data"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result, err := stats.Sum(stats.Float64Data(params.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to calculate sum: %w", err)
	}
	return jsonResult(map[string]any{"result": result})
}

// VarianceTool calculates the variance.
type VarianceTool struct{}

func (t *VarianceTool) Name() string        { return "variance" }
func (t *VarianceTool) Description() string { return "Calculate the variance of a dataset" }
func (t *VarianceTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
			"sample": map[string]any{
				"type":        "boolean",
				"description": "If true, calculate sample variance. If false, population variance. Default is false (population).",
			},
		},
		"required": []string{"data"},
	}
}

func (t *VarianceTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data   []float64 `json:"data"`
		Sample bool      `json:"sample"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	var result float64
	var err error
	if params.Sample {
		result, err = stats.SampleVariance(stats.Float64Data(params.Data))
	} else {
		result, err = stats.Variance(stats.Float64Data(params.Data))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to calculate variance: %w", err)
	}
	return jsonResult(map[string]any{"result": result})
}

// StandardDeviationTool calculates the standard deviation.
type StandardDeviationTool struct{}

func (t *StandardDeviationTool) Name() string { return "standard_deviation" }
func (t *StandardDeviationTool) Description() string {
	return "Calculate the standard deviation of a dataset"
}

func (t *StandardDeviationTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
			"sample": map[string]any{
				"type":        "boolean",
				"description": "If true, calculate sample standard deviation. If false, population. Default is false (population).",
			},
		},
		"required": []string{"data"},
	}
}

func (t *StandardDeviationTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data   []float64 `json:"data"`
		Sample bool      `json:"sample"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	var result float64
	var err error
	if params.Sample {
		result, err = stats.StandardDeviationSample(stats.Float64Data(params.Data))
	} else {
		result, err = stats.StandardDeviation(stats.Float64Data(params.Data))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to calculate standard deviation: %w", err)
	}
	return jsonResult(map[string]any{"result": result})
}

// QuartileTool calculates the quartiles (Q1, Q2/Median, Q3).
type QuartileTool struct{}

func (t *QuartileTool) Name() string { return "quartiles" }
func (t *QuartileTool) Description() string {
	return "Calculate the quartiles (Q1, Median, Q3) of a dataset"
}

func (t *QuartileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
		},
		"required": []string{"data"},
	}
}

func (t *QuartileTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data []float64 `json:"data"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result, err := stats.Quartile(stats.Float64Data(params.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to calculate quartiles: %w", err)
	}
	return jsonResult(map[string]any{
		"q1":     result.Q1,
		"median": result.Q2,
		"q3":     result.Q3,
	})
}

// PercentileTool calculates a specific percentile.
type PercentileTool struct{}

func (t *PercentileTool) Name() string        { return "percentile" }
func (t *PercentileTool) Description() string { return "Calculate the value at a specific percentile" }
func (t *PercentileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
			"percent": map[string]any{
				"type":        "number",
				"description": "Percentile to calculate (0-100)",
			},
		},
		"required": []string{"data", "percent"},
	}
}

func (t *PercentileTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data    []float64 `json:"data"`
		Percent float64   `json:"percent"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result, err := stats.Percentile(stats.Float64Data(params.Data), params.Percent)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate percentile: %w", err)
	}
	return jsonResult(map[string]any{"result": result})
}

// IQRTool calculates the Interquartile Range.
type IQRTool struct{}

func (t *IQRTool) Name() string        { return "interquartile_range" }
func (t *IQRTool) Description() string { return "Calculate the Interquartile Range (IQR) of a dataset" }
func (t *IQRTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
		},
		"required": []string{"data"},
	}
}

func (t *IQRTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data []float64 `json:"data"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result, err := stats.InterQuartileRange(stats.Float64Data(params.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to calculate IQR: %w", err)
	}
	return jsonResult(map[string]any{"result": result})
}

// =============================================================================
// CORRELATION TOOLS
// =============================================================================

// PearsonCorrelationTool calculates the Pearson correlation coefficient.
type PearsonCorrelationTool struct{}

func (t *PearsonCorrelationTool) Name() string { return "pearson_correlation" }
func (t *PearsonCorrelationTool) Description() string {
	return "Calculate the Pearson correlation coefficient between two datasets"
}

func (t *PearsonCorrelationTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data1": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "First dataset",
			},
			"data2": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Second dataset",
			},
		},
		"required": []string{"data1", "data2"},
	}
}

func (t *PearsonCorrelationTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data1 []float64 `json:"data1"`
		Data2 []float64 `json:"data2"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if len(params.Data1) == 0 || len(params.Data2) == 0 {
		return nil, ErrPearsonRequiresData
	}
	result, err := stats.Pearson(stats.Float64Data(params.Data1), stats.Float64Data(params.Data2))
	if err != nil {
		return nil, fmt.Errorf("failed to calculate Pearson correlation: %w", err)
	}
	return jsonResult(map[string]any{"result": result})
}

// =============================================================================
// DISTANCE TOOLS
// =============================================================================

// EuclideanDistanceTool calculates the Euclidean distance.
type EuclideanDistanceTool struct{}

func (t *EuclideanDistanceTool) Name() string { return "euclidean_distance" }
func (t *EuclideanDistanceTool) Description() string {
	return "Calculate the Euclidean distance between two points"
}

func (t *EuclideanDistanceTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"point1": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "First point coordinates",
			},
			"point2": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Second point coordinates",
			},
		},
		"required": []string{"point1", "point2"},
	}
}

func (t *EuclideanDistanceTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Point1 []float64 `json:"point1"`
		Point2 []float64 `json:"point2"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	p1 := stats.Float64Data(params.Point1)
	p2 := stats.Float64Data(params.Point2)
	result, err := stats.EuclideanDistance(p1, p2)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate Euclidean distance: %w", err)
	}
	return jsonResult(map[string]any{"distance": result})
}

// ManhattanDistanceTool calculates the Manhattan distance.
type ManhattanDistanceTool struct{}

func (t *ManhattanDistanceTool) Name() string { return "manhattan_distance" }
func (t *ManhattanDistanceTool) Description() string {
	return "Calculate the Manhattan distance between two points"
}

func (t *ManhattanDistanceTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"point1": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "First point coordinates",
			},
			"point2": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Second point coordinates",
			},
		},
		"required": []string{"point1", "point2"},
	}
}

func (t *ManhattanDistanceTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Point1 []float64 `json:"point1"`
		Point2 []float64 `json:"point2"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	p1 := stats.Float64Data(params.Point1)
	p2 := stats.Float64Data(params.Point2)
	result, err := stats.ManhattanDistance(p1, p2)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate Manhattan distance: %w", err)
	}
	return jsonResult(map[string]any{"distance": result})
}

// =============================================================================
// SAMPLING AND OUTLIERS TOOL
// =============================================================================

// =============================================================================
// SAMPLING TOOLS
// =============================================================================

// SampleTool performs random sampling.
type SampleTool struct{}

func (t *SampleTool) Name() string        { return "sample" }
func (t *SampleTool) Description() string { return "Perform random sampling from a dataset" }
func (t *SampleTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
			"takeNum": map[string]any{
				"type":        "integer",
				"description": "Number of samples to take (default: half of data length)",
			},
			"replacement": map[string]any{
				"type":        "boolean",
				"description": "Sample with replacement (default: false)",
			},
		},
		"required": []string{"data"},
	}
}

func (t *SampleTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data        []float64 `json:"data"`
		TakeNum     int       `json:"takeNum,omitempty"`
		Replacement bool      `json:"replacement,omitempty"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if params.TakeNum == 0 {
		params.TakeNum = len(params.Data) / 2
	}
	result, err := stats.Sample(stats.Float64Data(params.Data), params.TakeNum, params.Replacement)
	if err != nil {
		return nil, fmt.Errorf("failed to sample data: %w", err)
	}
	return jsonResult(map[string]any{"result": result})
}

// QuartileOutliersTool detects outliers using quartiles.
type QuartileOutliersTool struct{}

func (t *QuartileOutliersTool) Name() string { return "quartile_outliers" }
func (t *QuartileOutliersTool) Description() string {
	return "Detect mild and extreme outliers using the Interquartile Range (IQR) method"
}

func (t *QuartileOutliersTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
		},
		"required": []string{"data"},
	}
}

func (t *QuartileOutliersTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Data []float64 `json:"data"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	outliers, err := stats.QuartileOutliers(stats.Float64Data(params.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to detect outliers: %w", err)
	}
	return jsonResult(map[string]any{
		"mild":    outliers.Mild,
		"extreme": outliers.Extreme,
	})
}

// =============================================================================
// NORMAL DISTRIBUTION TOOL
// =============================================================================

// =============================================================================
// NORMAL DISTRIBUTION TOOLS
// =============================================================================

// NormalPdfTool calculates the Normal Probability Density Function.
type NormalPdfTool struct{}

func (t *NormalPdfTool) Name() string        { return "norm_pdf" }
func (t *NormalPdfTool) Description() string { return "Calculate the Normal PDF at x" }
func (t *NormalPdfTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"x": map[string]any{
				"type":        "number",
				"description": "X value",
			},
			"loc": map[string]any{
				"type":        "number",
				"description": "Mean (default: 0)",
			},
			"scale": map[string]any{
				"type":        "number",
				"description": "Standard deviation (default: 1)",
			},
		},
		"required": []string{"x"},
	}
}

func (t *NormalPdfTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		X     float64 `json:"x"`
		Loc   float64 `json:"loc,omitempty"`
		Scale float64 `json:"scale,omitempty"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result := stats.NormPdf(params.X, params.Loc, params.Scale)
	return jsonResult(map[string]any{"result": result})
}

// NormalCdfTool calculates the Normal Cumulative Distribution Function.
type NormalCdfTool struct{}

func (t *NormalCdfTool) Name() string        { return "norm_cdf" }
func (t *NormalCdfTool) Description() string { return "Calculate the Normal CDF at x" }
func (t *NormalCdfTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"x": map[string]any{
				"type":        "number",
				"description": "X value",
			},
			"loc": map[string]any{
				"type":        "number",
				"description": "Mean",
			},
			"scale": map[string]any{
				"type":        "number",
				"description": "Standard deviation",
			},
		},
		"required": []string{"x"},
	}
}

func (t *NormalCdfTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		X     float64 `json:"x"`
		Loc   float64 `json:"loc"`
		Scale float64 `json:"scale"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result := stats.NormCdf(params.X, params.Loc, params.Scale)
	return jsonResult(map[string]any{"result": result})
}

// NormalPpfTool calculates the Normal Percent Point Function (Inverse CDF).
type NormalPpfTool struct{}

func (t *NormalPpfTool) Name() string { return "norm_ppf" }
func (t *NormalPpfTool) Description() string {
	return "Calculate the Normal PPF (Inverse CDF) for probability p"
}

func (t *NormalPpfTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"p": map[string]any{
				"type":        "number",
				"description": "Probability (0-1)",
			},
			"loc": map[string]any{
				"type":        "number",
				"description": "Mean",
			},
			"scale": map[string]any{
				"type":        "number",
				"description": "Standard deviation",
			},
		},
		"required": []string{"p"},
	}
}

func (t *NormalPpfTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		P     float64 `json:"p"`
		Loc   float64 `json:"loc"`
		Scale float64 `json:"scale"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result := stats.NormPpf(params.P, params.Loc, params.Scale)
	return jsonResult(map[string]any{"result": result})
}

// NormalMeanTool calculates the Mean of the distribution.
type NormalMeanTool struct{}

func (t *NormalMeanTool) Name() string        { return "norm_mean" }
func (t *NormalMeanTool) Description() string { return "Calculate the Mean of a Normal distribution" }
func (t *NormalMeanTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"loc": map[string]any{
				"type":        "number",
				"description": "Mean",
			},
			"scale": map[string]any{
				"type":        "number",
				"description": "Standard deviation",
			},
		},
		"required": []string{},
	}
}

func (t *NormalMeanTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Loc   float64 `json:"loc"`
		Scale float64 `json:"scale"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result := stats.NormMean(params.Loc, params.Scale)
	return jsonResult(map[string]any{"result": result})
}

// NormalMedianTool calculates the Median of the distribution.
type NormalMedianTool struct{}

func (t *NormalMedianTool) Name() string { return "norm_median" }
func (t *NormalMedianTool) Description() string {
	return "Calculate the Median of a Normal distribution"
}

func (t *NormalMedianTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"loc": map[string]any{
				"type":        "number",
				"description": "Mean",
			},
			"scale": map[string]any{
				"type":        "number",
				"description": "Standard deviation",
			},
		},
		"required": []string{},
	}
}

func (t *NormalMedianTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Loc   float64 `json:"loc"`
		Scale float64 `json:"scale"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result := stats.NormMedian(params.Loc, params.Scale)
	return jsonResult(map[string]any{"result": result})
}

// NormalStdTool calculates the Standard Deviation of the distribution.
type NormalStdTool struct{}

func (t *NormalStdTool) Name() string { return "norm_std" }
func (t *NormalStdTool) Description() string {
	return "Calculate the Standard Deviation of a Normal distribution"
}

func (t *NormalStdTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"loc": map[string]any{
				"type":        "number",
				"description": "Mean",
			},
			"scale": map[string]any{
				"type":        "number",
				"description": "Standard deviation",
			},
		},
		"required": []string{},
	}
}

func (t *NormalStdTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Loc   float64 `json:"loc"`
		Scale float64 `json:"scale"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result := stats.NormStd(params.Loc, params.Scale)
	return jsonResult(map[string]any{"result": result})
}

// NormalVarTool calculates the Variance of the distribution.
type NormalVarTool struct{}

func (t *NormalVarTool) Name() string { return "norm_var" }
func (t *NormalVarTool) Description() string {
	return "Calculate the Variance of a Normal distribution"
}

func (t *NormalVarTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"loc": map[string]any{
				"type":        "number",
				"description": "Mean",
			},
			"scale": map[string]any{
				"type":        "number",
				"description": "Standard deviation",
			},
		},
		"required": []string{},
	}
}

func (t *NormalVarTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Loc   float64 `json:"loc"`
		Scale float64 `json:"scale"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result := stats.NormVar(params.Loc, params.Scale)
	return jsonResult(map[string]any{"result": result})
}

// NormalIntervalTool calculates the confidence interval.
type NormalIntervalTool struct{}

func (t *NormalIntervalTool) Name() string { return "norm_interval" }
func (t *NormalIntervalTool) Description() string {
	return "Calculate the confidence interval for a Normal distribution"
}

func (t *NormalIntervalTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"alpha": map[string]any{
				"type":        "number",
				"description": "Confidence level (e.g., 0.95)",
			},
			"loc": map[string]any{
				"type":        "number",
				"description": "Mean",
			},
			"scale": map[string]any{
				"type":        "number",
				"description": "Standard deviation",
			},
		},
		"required": []string{"alpha"},
	}
}

func (t *NormalIntervalTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		Alpha float64 `json:"alpha"`
		Loc   float64 `json:"loc"`
		Scale float64 `json:"scale"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	interval := stats.NormInterval(params.Alpha, params.Loc, params.Scale)
	return jsonResult(map[string]any{
		"lower": interval[0],
		"upper": interval[1],
	})
}

// BinomialNcrTool calculates combinations (N choose R).
type BinomialNcrTool struct{}

func (t *BinomialNcrTool) Name() string { return "ncr" }
func (t *BinomialNcrTool) Description() string {
	return "Calculate the number of combinations (N choose R)"
}

func (t *BinomialNcrTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"n": map[string]any{
				"type":        "integer",
				"description": "Total items (N)",
			},
			"r": map[string]any{
				"type":        "integer",
				"description": "Selected items (R)",
			},
		},
		"required": []string{"n", "r"},
	}
}

func (t *BinomialNcrTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		N int `json:"n"`
		R int `json:"r"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	result := stats.Ncr(params.N, params.R)
	return jsonResult(map[string]any{"result": result})
}

// =============================================================================
// REGRESSION TOOLS
// =============================================================================

// LinearRegressionTool performs linear regression.
type LinearRegressionTool struct{}

func (t *LinearRegressionTool) Name() string        { return "linear_regression" }
func (t *LinearRegressionTool) Description() string { return "Perform linear regression analysis" }
func (t *LinearRegressionTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"x": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "X coordinates",
			},
			"y": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Y coordinates",
			},
		},
		"required": []string{"x", "y"},
	}
}

func (t *LinearRegressionTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params struct {
		X []float64 `json:"x"`
		Y []float64 `json:"y"`
	}
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if len(params.X) != len(params.Y) {
		return nil, ErrXYArrayLengthMismatch
	}
	series := make(stats.Series, len(params.X))
	for i := range params.X {
		series[i] = stats.Coordinate{X: params.X[i], Y: params.Y[i]}
	}
	result, err := stats.LinearRegression(series)
	if err != nil {
		return nil, fmt.Errorf("linear regression failed: %w", err)
	}
	resultCoords := make([]map[string]float64, len(result))
	for i, coord := range result {
		resultCoords[i] = map[string]float64{"x": coord.X, "y": coord.Y}
	}
	return jsonResult(map[string]any{"regression": resultCoords})
}
