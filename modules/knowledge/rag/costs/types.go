package costs

import "encoding/json"

// WorkspaceCostSummaryResponse represents the cost summary for a workspace.
type WorkspaceCostSummaryResponse struct {
	WorkspaceID            string               `json:"workspace_id"`
	TotalCost              float64              `json:"total_cost"`
	DocumentCount          int                  `json:"document_count"`
	TotalTokens            int                  `json:"total_tokens"`
	AverageCostPerDocument float64              `json:"average_cost_per_document"`
	PeriodStart            *string              `json:"period_start"`
	PeriodEnd              *string              `json:"period_end"`
	ByOperation            []OperationBreakdown `json:"by_operation"`
	Budget                 *BudgetInfo          `json:"budget"`
}

// OperationBreakdown represents a cost breakdown by operation.
type OperationBreakdown struct {
	Operation    string  `json:"operation"`
	Cost         float64 `json:"cost"`
	Percentage   float64 `json:"percentage"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	CallCount    int     `json:"call_count"`
}

// BudgetInfo represents budget information.
type BudgetInfo struct {
	MonthlyBudgetUSD float64 `json:"monthly_budget_usd"`
	SpentUSD         float64 `json:"spent_usd"`
	RemainingUSD     float64 `json:"remaining_usd"`
	AlertThreshold   float64 `json:"alert_threshold"`
	IsOverBudget     bool    `json:"is_over_budget"`
}

// CostHistoryPoint represents a single cost history data point.
type CostHistoryPoint struct {
	Timestamp     string  `json:"timestamp"`
	TotalCost     float64 `json:"total_cost"`
	TotalTokens   int     `json:"total_tokens"`
	DocumentCount int     `json:"document_count"`
}

// CostHistoryParams represents query parameters for cost history.
type CostHistoryParams struct {
	StartDate   *string
	EndDate     *string
	Granularity *string
}

// UpdateBudgetRequest represents the request to update budget.
// Uses json.RawMessage-compatible fields matching BudgetInfo.
type UpdateBudgetRequest = BudgetInfo

// Ensure json is referenced
var _ = json.Marshal
