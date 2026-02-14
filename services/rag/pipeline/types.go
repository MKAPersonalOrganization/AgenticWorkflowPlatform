package pipeline

// EnhancedPipelineStatusResponse represents the pipeline status.
type EnhancedPipelineStatusResponse struct {
IsBusy                bool                      `json:"is_busy"`
JobName               *string                   `json:"job_name,omitempty"`
JobStart              *string                   `json:"job_start,omitempty"`
TotalDocuments        int                       `json:"total_documents"`
ProcessedDocuments    int                       `json:"processed_documents"`
CurrentBatch          int                       `json:"current_batch"`
TotalBatches          int                       `json:"total_batches"`
LatestMessage         *string                   `json:"latest_message,omitempty"`
HistoryMessages       []PipelineMessageResponse `json:"history_messages"`
CancellationRequested bool                      `json:"cancellation_requested"`
PendingTasks          int                       `json:"pending_tasks"`
ProcessingTasks       int                       `json:"processing_tasks"`
CompletedTasks        int                       `json:"completed_tasks"`
FailedTasks           int                       `json:"failed_tasks"`
}

// PipelineMessageResponse represents a pipeline message.
type PipelineMessageResponse struct {
Timestamp string `json:"timestamp"`
Level     string `json:"level"`
Message   string `json:"message"`
}

// CancelPipelineResponse represents the cancel response.
type CancelPipelineResponse struct {
Status  string `json:"status"`
Message string `json:"message"`
}

// QueueMetricsResponse represents queue metrics.
type QueueMetricsResponse struct {
PendingCount              int     `json:"pending_count"`
ProcessingCount           int     `json:"processing_count"`
ActiveWorkers             int     `json:"active_workers"`
MaxWorkers                int     `json:"max_workers"`
WorkerUtilization         int     `json:"worker_utilization"`
AvgWaitTimeSeconds        float64 `json:"avg_wait_time_seconds"`
MaxWaitTimeSeconds        float64 `json:"max_wait_time_seconds"`
ThroughputPerMinute       float64 `json:"throughput_per_minute"`
EstimatedQueueTimeSeconds float64 `json:"estimated_queue_time_seconds"`
RateLimited               bool    `json:"rate_limited"`
Timestamp                 string  `json:"timestamp"`
}

// EstimateCostRequest represents a cost estimate request.
type EstimateCostRequest struct {
Model        string `json:"model"`
InputTokens  int    `json:"input_tokens"`
OutputTokens int    `json:"output_tokens"`
}

// EstimateCostResponse represents a cost estimate response.
type EstimateCostResponse struct {
Model            string  `json:"model"`
InputTokens      int     `json:"input_tokens"`
OutputTokens     int     `json:"output_tokens"`
EstimatedCostUSD float64 `json:"estimated_cost_usd"`
FormattedCost    string  `json:"formatted_cost"`
}

// CostHistoryPoint represents a single cost history data point.
type CostHistoryPoint struct {
Timestamp     string  `json:"timestamp"`
TotalCost     float64 `json:"total_cost"`
TotalTokens   int     `json:"total_tokens"`
DocumentCount int     `json:"document_count"`
}

// PipelineHistoryParams represents query params for pipeline history.
type PipelineHistoryParams struct {
StartDate   *string
EndDate     *string
Granularity *string
}
