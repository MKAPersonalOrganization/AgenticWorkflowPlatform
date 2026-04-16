package analytics

import "time"

// StatRow is the view model for a statistics row.
// Mirrors modules/analytics/statistics.StatRow to decouple the presentation
// layer from the business logic layer.
type StatRow struct {
	AgentName  string
	Category   string
	Name       string
	TotalCount int
	Successes  int
	Failures   int
}

// TimeSeriesPoint is the view model for a time-series data point.
type TimeSeriesPoint struct {
	Bucket     time.Time
	TotalCalls int
	Errors     int
	Successes  int
	ErrorRate  float64
}

// ToolSummary is the view model for a tool summary.
type ToolSummary struct {
	ToolName    string
	TotalCalls  int
	Errors      int
	Successes   int
	SuccessRate float64
}
