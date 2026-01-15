package tools

import (
	"errors"
	"fmt"
	"school-with-ai/foundation/agentsdk/core"

	"github.com/montanaflynn/stats"
)

var (
	ErrCorrelationRequiresData          = errors.New("correlation requires data1 and data2")
	ErrPearsonRequiresData              = errors.New("pearson requires data1 and data2")
	ErrCovarianceRequiresData           = errors.New("covariance requires data1 and data2")
	ErrCovariancePopulationRequiresData = errors.New("covariance_population requires data1 and data2")
	ErrAutocorrelationRequiresData      = errors.New("autocorrelation requires data1")
	ErrNormFitRequiresData              = errors.New("norm_fit requires data")
	ErrXYArrayLengthMismatch            = errors.New("x and y arrays must have the same length")
	ErrUnknownOperation                 = errors.New("unknown operation")
	ErrUnknownRegressionType            = errors.New("unknown regression type")
)

// =============================================================================
// DESCRIPTIVE STATISTICS TOOL
// =============================================================================

type DescriptiveStatsTool struct{}

func NewDescriptiveStatsTool() *DescriptiveStatsTool {
	return &DescriptiveStatsTool{}
}

func (t *DescriptiveStatsTool) Name() string {
	return "descriptive_statistics"
}

func (t *DescriptiveStatsTool) Description() string {
	return "Calculate descriptive statistics: mean, median, mode, min, max, sum, variance, standard deviation, quartiles, percentiles"
}

func (t *DescriptiveStatsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type": "string",
				"enum": []string{
					"mean", "median", "mode", "min", "max", "sum",
					"variance", "population_variance", "sample_variance",
					"standard_deviation", "standard_deviation_population", "standard_deviation_sample",
					"quartile", "percentile", "percentile_nearest_rank",
					"geometric_mean", "harmonic_mean", "midhinge", "trimean",
					"interquartile_range", "median_absolute_deviation",
					"median_absolute_deviation_population", "describe",
				},
				"description": "Statistical operation to perform",
			},
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers to analyze",
			},
			"percent": map[string]any{
				"type":        "number",
				"description": "Percentile value (0-100) - required for percentile operations",
			},
			"allowNaN": map[string]any{
				"type":        "boolean",
				"description": "Allow NaN values in describe operation",
			},
			"percentiles": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Custom percentiles for describe operation",
			},
		},
		"required": []string{"operation", "data"},
	}
}

func (t *DescriptiveStatsTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		Operation   string    `json:"operation"`
		Data        []float64 `json:"data"`
		Percent     float64   `json:"percent,omitempty"`
		AllowNaN    bool      `json:"allowNaN,omitempty"`
		Percentiles []float64 `json:"percentiles,omitempty"`
	}

	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	data := stats.Float64Data(params.Data)
	var result any
	var err error

	switch params.Operation {
	case "mean":
		result, err = stats.Mean(data)
	case "median":
		result, err = stats.Median(data)
	case "mode":
		result, err = stats.Mode(data)
	case "min":
		result, err = stats.Min(data)
	case "max":
		result, err = stats.Max(data)
	case "sum":
		result, err = stats.Sum(data)
	case "variance":
		result, err = stats.Variance(data)
	case "population_variance":
		result, err = stats.PopulationVariance(data)
	case "sample_variance":
		result, err = stats.SampleVariance(data)
	case "standard_deviation":
		result, err = stats.StandardDeviation(data)
	case "standard_deviation_population":
		result, err = stats.StandardDeviationPopulation(data)
	case "standard_deviation_sample":
		result, err = stats.StandardDeviationSample(data)
	case "quartile":
		result, err = stats.Quartile(data)
	case "percentile":
		result, err = stats.Percentile(data, params.Percent)
	case "percentile_nearest_rank":
		result, err = stats.PercentileNearestRank(data, params.Percent)
	case "geometric_mean":
		result, err = stats.GeometricMean(data)
	case "harmonic_mean":
		result, err = stats.HarmonicMean(data)
	case "midhinge":
		result, err = stats.Midhinge(data)
	case "trimean":
		result, err = stats.Trimean(data)
	case "interquartile_range":
		result, err = stats.InterQuartileRange(data)
	case "median_absolute_deviation":
		result, err = stats.MedianAbsoluteDeviation(data)
	case "median_absolute_deviation_population":
		result, err = stats.MedianAbsoluteDeviationPopulation(data)
	case "describe":
		var percentiles *[]float64
		if len(params.Percentiles) > 0 {
			percentiles = &params.Percentiles
		}
		desc, err1 := stats.Describe(data, params.AllowNaN, percentiles)
		if err1 != nil {
			err = err1
		} else {
			quartiles, _ := stats.Quartile(data)
			result = map[string]any{
				"count":  desc.Count,
				"mean":   desc.Mean,
				"std":    desc.Std,
				"min":    desc.Min,
				"q1":     quartiles.Q1,
				"median": quartiles.Q2,
				"q3":     quartiles.Q3,
				"max":    desc.Max,
			}
		}
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownOperation, params.Operation)
	}

	if err != nil {
		return nil, fmt.Errorf("operation failed: %w", err)
	}

	return map[string]any{
		"operation": params.Operation,
		"result":    result,
	}, nil
}

// =============================================================================
// CORRELATION AND COVARIANCE TOOL
// =============================================================================

type CorrelationTool struct{}

func NewCorrelationTool() *CorrelationTool {
	return &CorrelationTool{}
}

func (t *CorrelationTool) Name() string {
	return "correlation_analysis"
}

func (t *CorrelationTool) Description() string {
	return "Calculate correlation and covariance between datasets: correlation, pearson, covariance, covariance_population, autocorrelation"
}

func (t *CorrelationTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"enum":        []string{"correlation", "pearson", "covariance", "covariance_population", "autocorrelation"},
				"description": "Correlation operation to perform",
			},
			"data1": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "First dataset (not required for autocorrelation)",
			},
			"data2": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Second dataset (not required for autocorrelation)",
			},
			"lags": map[string]any{
				"type":        "integer",
				"description": "Number of lags for autocorrelation",
			},
		},
		"required": []string{"operation"},
	}
}

func (t *CorrelationTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		Operation string    `json:"operation"`
		Data1     []float64 `json:"data1,omitempty"`
		Data2     []float64 `json:"data2,omitempty"`
		Lags      int       `json:"lags,omitempty"`
	}

	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var result any
	var err error

	switch params.Operation {
	case "correlation":
		if len(params.Data1) == 0 || len(params.Data2) == 0 {
			return nil, ErrCorrelationRequiresData
		}
		result, err = stats.Correlation(stats.Float64Data(params.Data1), stats.Float64Data(params.Data2))
	case "pearson":
		if len(params.Data1) == 0 || len(params.Data2) == 0 {
			return nil, ErrPearsonRequiresData
		}
		result, err = stats.Pearson(stats.Float64Data(params.Data1), stats.Float64Data(params.Data2))
	case "covariance":
		if len(params.Data1) == 0 || len(params.Data2) == 0 {
			return nil, ErrCovarianceRequiresData
		}
		result, err = stats.Covariance(stats.Float64Data(params.Data1), stats.Float64Data(params.Data2))
	case "covariance_population":
		if len(params.Data1) == 0 || len(params.Data2) == 0 {
			return nil, ErrCovariancePopulationRequiresData
		}
		result, err = stats.CovariancePopulation(stats.Float64Data(params.Data1), stats.Float64Data(params.Data2))
	case "autocorrelation":
		if len(params.Data1) == 0 {
			return nil, ErrAutocorrelationRequiresData
		}
		result, err = stats.AutoCorrelation(stats.Float64Data(params.Data1), params.Lags)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownOperation, params.Operation)
	}

	if err != nil {
		return nil, fmt.Errorf("operation failed: %w", err)
	}

	return map[string]any{
		"operation": params.Operation,
		"result":    result,
	}, nil
}

// =============================================================================
// DISTANCE METRICS TOOL
// =============================================================================

type DistanceMetricsTool struct{}

func NewDistanceMetricsTool() *DistanceMetricsTool {
	return &DistanceMetricsTool{}
}

func (t *DistanceMetricsTool) Name() string {
	return "distance_metrics"
}

func (t *DistanceMetricsTool) Description() string {
	return "Calculate distance between two points: euclidean_distance, manhattan_distance, chebyshev_distance, minkowski_distance"
}

func (t *DistanceMetricsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"enum":        []string{"euclidean_distance", "manhattan_distance", "chebyshev_distance", "minkowski_distance"},
				"description": "Distance metric to calculate",
			},
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
			"lambda": map[string]any{
				"type":        "number",
				"description": "Lambda parameter for Minkowski distance (default: 2)",
			},
		},
		"required": []string{"operation", "point1", "point2"},
	}
}

func (t *DistanceMetricsTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		Operation string    `json:"operation"`
		Point1    []float64 `json:"point1"`
		Point2    []float64 `json:"point2"`
		Lambda    float64   `json:"lambda,omitempty"`
	}

	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var result float64
	var err error

	p1 := stats.Float64Data(params.Point1)
	p2 := stats.Float64Data(params.Point2)

	switch params.Operation {
	case "euclidean_distance":
		result, err = stats.EuclideanDistance(p1, p2)
	case "manhattan_distance":
		result, err = stats.ManhattanDistance(p1, p2)
	case "chebyshev_distance":
		result, err = stats.ChebyshevDistance(p1, p2)
	case "minkowski_distance":
		lambda := params.Lambda
		if lambda == 0 {
			lambda = 2
		}
		result, err = stats.MinkowskiDistance(p1, p2, lambda)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownOperation, params.Operation)
	}

	if err != nil {
		return nil, fmt.Errorf("operation failed: %w", err)
	}

	return map[string]any{
		"operation": params.Operation,
		"distance":  result,
	}, nil
}

// =============================================================================
// SAMPLING AND OUTLIERS TOOL
// =============================================================================

type SamplingTool struct{}

func NewSamplingTool() *SamplingTool {
	return &SamplingTool{}
}

func (t *SamplingTool) Name() string {
	return "sampling_and_outliers"
}

func (t *SamplingTool) Description() string {
	return "Sample data and detect outliers: sample, stable_sample, quartile_outliers"
}

func (t *SamplingTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"enum":        []string{"sample", "stable_sample", "quartile_outliers"},
				"description": "Sampling operation to perform",
			},
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers",
			},
			"takeNum": map[string]any{
				"type":        "integer",
				"description": "Number of samples to take",
			},
			"replacement": map[string]any{
				"type":        "boolean",
				"description": "Sample with replacement (for sample operation)",
			},
		},
		"required": []string{"operation", "data"},
	}
}

func (t *SamplingTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		Operation   string    `json:"operation"`
		Data        []float64 `json:"data"`
		TakeNum     int       `json:"takeNum,omitempty"`
		Replacement bool      `json:"replacement,omitempty"`
	}

	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	data := stats.Float64Data(params.Data)
	var result any
	var err error

	switch params.Operation {
	case "sample":
		if params.TakeNum == 0 {
			params.TakeNum = len(params.Data) / 2
		}
		result, err = stats.Sample(data, params.TakeNum, params.Replacement)
	case "stable_sample":
		if params.TakeNum == 0 {
			params.TakeNum = len(params.Data) / 2
		}
		result, err = stats.StableSample(data, params.TakeNum)
	case "quartile_outliers":
		outliers, err1 := stats.QuartileOutliers(data)
		if err1 != nil {
			err = err1
		} else {
			result = map[string]any{
				"mild":    outliers.Mild,
				"extreme": outliers.Extreme,
			}
		}
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownOperation, params.Operation)
	}

	if err != nil {
		return nil, fmt.Errorf("operation failed: %w", err)
	}

	return map[string]any{
		"operation": params.Operation,
		"result":    result,
	}, nil
}

// =============================================================================
// DATA TRANSFORMATION TOOL
// =============================================================================

type DataTransformTool struct{}

func NewDataTransformTool() *DataTransformTool {
	return &DataTransformTool{}
}

func (t *DataTransformTool) Name() string {
	return "data_transformation"
}

func (t *DataTransformTool) Description() string {
	return "Transform data: sigmoid, softmax, cumulative_sum, entropy, round"
}

func (t *DataTransformTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"enum":        []string{"sigmoid", "softmax", "cumulative_sum", "entropy", "round"},
				"description": "Transformation operation to perform",
			},
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Array of numbers to transform",
			},
			"value": map[string]any{
				"type":        "number",
				"description": "Single value (for round operation)",
			},
			"places": map[string]any{
				"type":        "integer",
				"description": "Decimal places (for round operation)",
			},
		},
		"required": []string{"operation"},
	}
}

func (t *DataTransformTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		Operation string    `json:"operation"`
		Data      []float64 `json:"data,omitempty"`
		Value     float64   `json:"value,omitempty"`
		Places    int       `json:"places,omitempty"`
	}

	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var result any
	var err error

	switch params.Operation {
	case "sigmoid":
		result, err = stats.Sigmoid(stats.Float64Data(params.Data))
	case "softmax":
		result, err = stats.SoftMax(stats.Float64Data(params.Data))
	case "cumulative_sum":
		result, err = stats.CumulativeSum(stats.Float64Data(params.Data))
	case "entropy":
		result, err = stats.Entropy(stats.Float64Data(params.Data))
	case "round":
		result, err = stats.Round(params.Value, params.Places)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownOperation, params.Operation)
	}

	if err != nil {
		return nil, fmt.Errorf("operation failed: %w", err)
	}

	return map[string]any{
		"operation": params.Operation,
		"result":    result,
	}, nil
}

// =============================================================================
// NORMAL DISTRIBUTION TOOL
// =============================================================================

type NormalDistributionTool struct{}

func NewNormalDistributionTool() *NormalDistributionTool {
	return &NormalDistributionTool{}
}

func (t *NormalDistributionTool) Name() string {
	return "normal_distribution"
}

func (t *NormalDistributionTool) Description() string {
	return "Normal distribution functions: pdf, cdf, ppf, sf, isf, log variants, fit, moments, stats, interval, random sampling, ncr (N choose R combinatorics)"
}

func (t *NormalDistributionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type": "string",
				"enum": []string{
					"norm_pdf", "norm_cdf", "norm_ppf", "norm_sf", "norm_isf",
					"norm_log_pdf", "norm_log_cdf", "norm_log_sf",
					"norm_mean", "norm_median", "norm_std", "norm_var",
					"norm_entropy", "norm_moment", "norm_stats", "norm_interval",
					"norm_fit", "norm_box_muller_rvs", "norm_ppf_rvs", "ncr",
				},
				"description": "Normal distribution operation",
			},
			"x": map[string]any{
				"type":        "number",
				"description": "X value for pdf/cdf/sf/isf/log functions",
			},
			"p": map[string]any{
				"type":        "number",
				"description": "Probability for ppf/isf functions",
			},
			"loc": map[string]any{
				"type":        "number",
				"description": "Location parameter (mean)",
			},
			"scale": map[string]any{
				"type":        "number",
				"description": "Scale parameter (standard deviation)",
			},
			"n": map[string]any{
				"type":        "integer",
				"description": "Moment order (for norm_moment) or N value (for ncr N choose R)",
			},
			"moments": map[string]any{
				"type":        "string",
				"description": "Moments string (e.g., 'mv' for mean and variance)",
			},
			"alpha": map[string]any{
				"type":        "number",
				"description": "Alpha parameter for norm_interval",
			},
			"size": map[string]any{
				"type":        "integer",
				"description": "Number of random samples to generate",
			},
			"data": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Data for norm_fit operation",
			},
			"r": map[string]any{
				"type":        "integer",
				"description": "R value for ncr (N choose R) operation",
			},
		},
		"required": []string{"operation"},
	}
}

func (t *NormalDistributionTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		Operation string    `json:"operation"`
		X         float64   `json:"x,omitempty"`
		P         float64   `json:"p,omitempty"`
		Loc       float64   `json:"loc,omitempty"`
		Scale     float64   `json:"scale,omitempty"`
		N         int       `json:"n,omitempty"`
		R         int       `json:"r,omitempty"`
		Moments   string    `json:"moments,omitempty"`
		Alpha     float64   `json:"alpha,omitempty"`
		Size      int       `json:"size,omitempty"`
		Data      []float64 `json:"data,omitempty"`
	}

	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var result any

	switch params.Operation {
	case "norm_pdf":
		result = stats.NormPdf(params.X, params.Loc, params.Scale)
	case "norm_cdf":
		result = stats.NormCdf(params.X, params.Loc, params.Scale)
	case "norm_ppf":
		result = stats.NormPpf(params.P, params.Loc, params.Scale)
	case "norm_sf":
		result = stats.NormSf(params.X, params.Loc, params.Scale)
	case "norm_isf":
		result = stats.NormIsf(params.P, params.Loc, params.Scale)
	case "norm_log_pdf":
		result = stats.NormLogPdf(params.X, params.Loc, params.Scale)
	case "norm_log_cdf":
		result = stats.NormLogCdf(params.X, params.Loc, params.Scale)
	case "norm_log_sf":
		result = stats.NormLogSf(params.X, params.Loc, params.Scale)
	case "norm_mean":
		result = stats.NormMean(params.Loc, params.Scale)
	case "norm_median":
		result = stats.NormMedian(params.Loc, params.Scale)
	case "norm_std":
		result = stats.NormStd(params.Loc, params.Scale)
	case "norm_var":
		result = stats.NormVar(params.Loc, params.Scale)
	case "norm_entropy":
		result = stats.NormEntropy(params.Loc, params.Scale)
	case "norm_moment":
		result = stats.NormMoment(params.N, params.Loc, params.Scale)
	case "norm_stats":
		if params.Moments == "" {
			params.Moments = "mv"
		}
		result = stats.NormStats(params.Loc, params.Scale, params.Moments)
	case "norm_interval":
		interval := stats.NormInterval(params.Alpha, params.Loc, params.Scale)
		result = map[string]any{
			"lower": interval[0],
			"upper": interval[1],
		}
	case "norm_fit":
		if len(params.Data) == 0 {
			return nil, ErrNormFitRequiresData
		}
		fitParams := stats.NormFit(params.Data)
		result = map[string]any{
			"loc":   fitParams[0],
			"scale": fitParams[1],
		}
	case "norm_box_muller_rvs":
		if params.Size == 0 {
			params.Size = 100
		}
		result = stats.NormBoxMullerRvs(params.Loc, params.Scale, params.Size)
	case "norm_ppf_rvs":
		if params.Size == 0 {
			params.Size = 100
		}
		result = stats.NormPpfRvs(params.Loc, params.Scale, params.Size)
	case "ncr":
		result = stats.Ncr(params.N, params.R)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownOperation, params.Operation)
	}

	return map[string]any{
		"operation": params.Operation,
		"result":    result,
	}, nil
}

// =============================================================================
// GEOMETRIC DISTRIBUTION TOOL
// =============================================================================

type GeometricDistributionTool struct{}

func NewGeometricDistributionTool() *GeometricDistributionTool {
	return &GeometricDistributionTool{}
}

func (t *GeometricDistributionTool) Name() string {
	return "geometric_distribution"
}

func (t *GeometricDistributionTool) Description() string {
	return "Geometric distribution functions: probability, expectation, variance"
}

func (t *GeometricDistributionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"enum":        []string{"prob_geom", "exp_geom", "var_geom"},
				"description": "Geometric distribution operation",
			},
			"p": map[string]any{
				"type":        "number",
				"description": "Probability parameter (0 < p <= 1)",
			},
			"a": map[string]any{
				"type":        "integer",
				"description": "Lower bound (for prob_geom)",
			},
			"b": map[string]any{
				"type":        "integer",
				"description": "Upper bound (for prob_geom)",
			},
		},
		"required": []string{"operation", "p"},
	}
}

func (t *GeometricDistributionTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		Operation string  `json:"operation"`
		P         float64 `json:"p"`
		A         int     `json:"a,omitempty"`
		B         int     `json:"b,omitempty"`
	}

	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var result float64
	var err error

	switch params.Operation {
	case "prob_geom":
		result, err = stats.ProbGeom(params.A, params.B, params.P)
	case "exp_geom":
		result, err = stats.ExpGeom(params.P)
	case "var_geom":
		result, err = stats.VarGeom(params.P)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownOperation, params.Operation)
	}

	if err != nil {
		return nil, fmt.Errorf("operation failed: %w", err)
	}

	return map[string]any{
		"operation": params.Operation,
		"result":    result,
	}, nil
}

// =============================================================================
// REGRESSION TOOL
// =============================================================================

type RegressionTool struct{}

func NewRegressionTool() *RegressionTool {
	return &RegressionTool{}
}

func (r *RegressionTool) Name() string {
	return "regression_analysis"
}

func (r *RegressionTool) Description() string {
	return "Perform regression analysis: linear, exponential, logarithmic"
}

func (r *RegressionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type": map[string]any{
				"type":        "string",
				"enum":        []string{"linear", "exponential", "logarithmic"},
				"description": "Type of regression",
			},
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
		"required": []string{"type", "x", "y"},
	}
}

func (r *RegressionTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		Type string    `json:"type"`
		X    []float64 `json:"x"`
		Y    []float64 `json:"y"`
	}

	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if len(params.X) != len(params.Y) {
		return nil, ErrXYArrayLengthMismatch
	}

	series := make(stats.Series, len(params.X))
	for i := range params.X {
		series[i] = stats.Coordinate{X: params.X[i], Y: params.Y[i]}
	}

	var result stats.Series
	var err error

	switch params.Type {
	case "linear":
		result, err = stats.LinearRegression(series)
	case "exponential":
		result, err = stats.ExponentialRegression(series)
	case "logarithmic":
		result, err = stats.LogarithmicRegression(series)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownRegressionType, params.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("regression failed: %w", err)
	}

	resultCoords := make([]map[string]float64, len(result))
	for i, coord := range result {
		resultCoords[i] = map[string]float64{
			"x": coord.X,
			"y": coord.Y,
		}
	}

	return map[string]any{
		"type":       params.Type,
		"regression": resultCoords,
	}, nil
}
