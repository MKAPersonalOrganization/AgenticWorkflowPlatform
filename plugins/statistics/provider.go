package statistics

import (
	"context"
	"time"

	"github.com/a-h/templ"

	"school-with-ai/foundation/agent"
	"school-with-ai/services/toolregistry"
)

// Provider implements the toolregistry.PluginProvider interface for the Statistics toolset.
type Provider struct{}

func (p *Provider) ID() string {
	return "statistics"
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "statistics-set",
			Name:        "Statistical Analysis",
			Description: "Statistical analysis and data processing tools",
			Category:    "Math",
			ToolIDs:     []string{"stats-mean", "stats-median", "stats-mode", "stats-variance", "stats-stddev", "stats-correlation", "stats-regression"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{ID: "stats-mean", Name: "Mean", Description: "Calculate the mean of a dataset", Category: "Statistics", ToolSetID: "statistics-set", CreatedAt: now, UpdatedAt: now},
		{ID: "stats-median", Name: "Median", Description: "Calculate the median of a dataset", Category: "Statistics", ToolSetID: "statistics-set", CreatedAt: now, UpdatedAt: now},
		{ID: "stats-mode", Name: "Mode", Description: "Calculate the mode of a dataset", Category: "Statistics", ToolSetID: "statistics-set", CreatedAt: now, UpdatedAt: now},
		{ID: "stats-variance", Name: "Variance", Description: "Calculate the variance of a dataset", Category: "Statistics", ToolSetID: "statistics-set", CreatedAt: now, UpdatedAt: now},
		{ID: "stats-stddev", Name: "Standard Deviation", Description: "Calculate the standard deviation of a dataset", Category: "Statistics", ToolSetID: "statistics-set", CreatedAt: now, UpdatedAt: now},
		{ID: "stats-correlation", Name: "Correlation", Description: "Calculate correlation between two variables", Category: "Statistics", ToolSetID: "statistics-set", CreatedAt: now, UpdatedAt: now},
		{ID: "stats-regression", Name: "Linear Regression", Description: "Perform linear regression analysis", Category: "Statistics", ToolSetID: "statistics-set", CreatedAt: now, UpdatedAt: now},
	}
}

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	switch toolID {
	case "stats-mean":
		return &MeanTool{}, nil
	case "stats-median":
		return &MedianTool{}, nil
	case "stats-mode":
		return &ModeTool{}, nil
	case "stats-variance":
		return &VarianceTool{}, nil
	case "stats-stddev":
		return &StandardDeviationTool{}, nil
	case "stats-correlation":
		return &PearsonCorrelationTool{}, nil
	case "stats-regression":
		return &LinearRegressionTool{}, nil
	default:
		return nil, nil // Not handled by this plugin
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"mean", "median", "mode", "min", "max", "sum", "variance", "standard_deviation", "quartile", "percentile", "interquartile_range", "pearson_correlation", "euclidean_distance", "manhattan_distance", "sample", "quartile_outliers", "normal_pdf", "normal_cdf", "normal_ppf", "normal_mean", "normal_median", "normal_std", "normal_var", "normal_interval", "binomial_ncr", "linear_regression"}
}

func (p *Provider) UIHandledElicitations() []string {
	return nil
}
