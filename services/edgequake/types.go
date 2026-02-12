package edgequake

// ListDocumentsResponse represents the response from the Edgequake list documents endpoint.
type ListDocumentsResponse struct {
	Documents    []DocumentSummary `json:"documents"`
	Total        int               `json:"total"`
	Page         int               `json:"page"`
	PageSize     int               `json:"page_size"`
	TotalPages   int               `json:"total_pages"`
	HasMore      bool              `json:"has_more"`
	StatusCounts StatusCounts      `json:"status_counts"`
}

// DocumentSummary represents a summary of an Edgequake document.
type DocumentSummary struct {
	ID             string   `json:"id"`
	Title          *string  `json:"title,omitempty"`
	FileName       *string  `json:"file_name,omitempty"`
	ContentSummary *string  `json:"content_summary,omitempty"`
	ContentLength  *int     `json:"content_length,omitempty"`
	ChunkCount     int      `json:"chunk_count"`
	EntityCount    *int     `json:"entity_count,omitempty"`
	Status         *string  `json:"status,omitempty"` // legacy
	ErrorMessage   *string  `json:"error_message,omitempty"`
	TrackID        *string  `json:"track_id,omitempty"`
	CreatedAt      *string  `json:"created_at,omitempty"`
	UpdatedAt      *string  `json:"updated_at,omitempty"`
	CostUSD        *float64 `json:"cost_usd,omitempty"`
	InputTokens    *int     `json:"input_tokens,omitempty"`
	OutputTokens   *int     `json:"output_tokens,omitempty"`
	TotalTokens    *int     `json:"total_tokens,omitempty"`
	LLMModel       *string  `json:"llm_model,omitempty"`
	EmbeddingModel *string  `json:"embedding_model,omitempty"`
	SourceType     *string  `json:"source_type,omitempty"`
	CurrentStage   *string  `json:"current_stage,omitempty"`
	StageProgress  *float32 `json:"stage_progress,omitempty"`
	StageMessage   *string  `json:"stage_message,omitempty"`
	PDFID          *string  `json:"pdf_id,omitempty"`
}

// StatusCounts represents document status counts.
type StatusCounts struct {
	Pending        int `json:"pending"`
	Processing     int `json:"processing"`
	Completed      int `json:"completed"`
	PartialFailure int `json:"partial_failure"`
	Failed         int `json:"failed"`
	Cancelled      int `json:"cancelled"`
}

// FileUploadResponse represents the response from the Edgequake file upload endpoint.
type FileUploadResponse struct {
	DocumentID        string `json:"document_id"`
	FileName          string `json:"filename"`
	Size              int    `json:"size"`
	ContentHash       string `json:"content_hash"`
	Status            string `json:"status"`
	ChunkCount        int    `json:"chunk_count"`
	EntityCount       int    `json:"entity_count"`
	RelationshipCount int    `json:"relationship_count"`
	IsDuplicate       bool   `json:"is_duplicate"`
}

// ListPdfsResponse represents the response from the Edgequake list PDFs endpoint.
type ListPdfsResponse struct {
	Items      []PdfListItem     `json:"items"`
	Pagination PdfPaginationInfo `json:"pagination"`
}

// PdfListItem represents a single PDF in a listing.
type PdfListItem struct {
	PdfID       string  `json:"pdf_id"`
	Filename    string  `json:"filename"`
	Status      string  `json:"status"`
	FileSize    int64   `json:"file_size_bytes"`
	PageCount   *int32  `json:"page_count,omitempty"`
	CreatedAt   string  `json:"created_at"`
	ProcessedAt *string `json:"processed_at,omitempty"`
}

// PdfPaginationInfo represents pagination details for PDF listing.
type PdfPaginationInfo struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalCount int64 `json:"total_count"`
	TotalPages int   `json:"total_pages"`
}

// PdfUploadResponse represents the response from the Edgequake PDF upload endpoint.
type PdfUploadResponse struct {
	PdfID                string      `json:"pdf_id"`
	DocumentID           *string     `json:"document_id"`
	Status               string      `json:"status"`
	TaskID               string      `json:"task_id"`
	TrackID              *string     `json:"track_id"`
	Message              string      `json:"message"`
	EstimatedTimeSeconds uint64      `json:"estimated_time_seconds"`
	Metadata             PdfMetadata `json:"metadata"`
}

// PdfMetadata represents metadata for an uploaded PDF.
type PdfMetadata struct {
	Filename       string  `json:"filename"`
	FileSizeBytes  int64   `json:"file_size_bytes"`
	PageCount      *int32  `json:"page_count"`
	Sha256Checksum string  `json:"sha256_checksum"`
	VisionEnabled  bool    `json:"vision_enabled"`
	VisionModel    *string `json:"vision_model"`
}

// PdfUploadProgress represents the progress of a PDF upload.
type PdfUploadProgress struct {
	TrackID           string          `json:"track_id"`
	PdfID             string          `json:"pdf_id"`
	DocumentID        *string         `json:"document_id,omitempty"`
	Filename          string          `json:"filename"`
	Phases            []PhaseProgress `json:"phases"`
	OverallPercentage float32         `json:"overall_percentage"`
	EtaSeconds        *uint64         `json:"eta_seconds,omitempty"`
	IsComplete        bool            `json:"is_complete"`
	IsFailed          bool            `json:"is_failed"`
	StartedAt         string          `json:"started_at"`
	UpdatedAt         string          `json:"updated_at"`
	CompletedAt       *string         `json:"completed_at,omitempty"`
}

// PhaseProgress represents the progress of a single pipeline phase.
type PhaseProgress struct {
	Phase         string      `json:"phase"`
	Status        string      `json:"status"`
	Current       int         `json:"current"`
	Total         int         `json:"total"`
	Percentage    float32     `json:"percentage"`
	EtaSeconds    *uint64     `json:"eta_seconds,omitempty"`
	Message       string      `json:"message"`
	Error         *PhaseError `json:"error,omitempty"`
	StartedAt     *string     `json:"started_at,omitempty"`
	CompletedAt   *string     `json:"completed_at,omitempty"`
	AvgItemTimeMs float64     `json:"avg_item_time_ms"`
}

// PhaseError represents an error in a pipeline phase.
type PhaseError struct {
	Message      string  `json:"message"`
	Code         string  `json:"code"`
	Retryable    bool    `json:"retryable"`
	Suggestion   string  `json:"suggestion"`
	AffectedItem *string `json:"affected_item,omitempty"`
}

// PdfStatusResponse represents the status of a PDF processing task.
type PdfStatusResponse struct {
	PdfID                string            `json:"pdf_id"`
	DocumentID           *string           `json:"document_id,omitempty"`
	Status               string            `json:"status"`
	ProcessingDurationMs *int64            `json:"processing_duration_ms,omitempty"`
	Metadata             PdfStatusMetadata `json:"metadata"`
	Errors               interface{}       `json:"errors,omitempty"`
}

// PdfStatusMetadata represents metadata in the status response.
type PdfStatusMetadata struct {
	Filename      string  `json:"filename"`
	FileSizeBytes int64   `json:"file_size_bytes"`
	PageCount     *int32  `json:"page_count,omitempty"`
	VisionEnabled bool    `json:"vision_enabled"`
	VisionModel   *string `json:"vision_model,omitempty"`
}

// PdfContentResponse represents the content of a PDF.
type PdfContentResponse struct {
	PdfID           string  `json:"pdf_id"`
	Filename        string  `json:"filename"`
	FileSizeBytes   int64   `json:"file_size_bytes"`
	ContentType     string  `json:"content_type"`
	MarkdownContent *string `json:"markdown_content,omitempty"`
	IsProcessed     bool    `json:"is_processed"`
}
