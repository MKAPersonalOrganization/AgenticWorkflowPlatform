package documents

import "encoding/json"

// ListDocumentsResponse matches Rust ListDocumentsResponse exactly.
type ListDocumentsResponse struct {
	Documents    []DocumentSummary `json:"documents"`
	Total        int               `json:"total"`
	Page         int               `json:"page"`
	PageSize     int               `json:"page_size"`
	TotalPages   int               `json:"total_pages"`
	HasMore      bool              `json:"has_more"`
	StatusCounts StatusCounts      `json:"status_counts"`
}

// DocumentSummary matches Rust DocumentSummary exactly.
type DocumentSummary struct {
	ID             string   `json:"id"`
	Title          *string  `json:"title,omitempty"`
	FileName       *string  `json:"file_name,omitempty"`
	ContentSummary *string  `json:"content_summary,omitempty"`
	ContentLength  *int     `json:"content_length,omitempty"`
	ChunkCount     int      `json:"chunk_count"`
	EntityCount    *int     `json:"entity_count,omitempty"`
	Status         *string  `json:"status,omitempty"`
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

// DocumentDetail matches Rust DocumentDetailResponse exactly.
type DocumentDetail struct {
	ID                string           `json:"id"`
	Title             *string          `json:"title,omitempty"`
	FileName          *string          `json:"file_name,omitempty"`
	Content           *string          `json:"content,omitempty"`
	ContentSummary    *string          `json:"content_summary,omitempty"`
	ContentLength     *int             `json:"content_length,omitempty"`
	ContentHash       *string          `json:"content_hash,omitempty"`
	ChunkCount        int              `json:"chunk_count"`
	EntityCount       *int             `json:"entity_count,omitempty"`
	RelationshipCount *int             `json:"relationship_count,omitempty"`
	Status            string           `json:"status"`
	ErrorMessage      *string          `json:"error_message,omitempty"`
	SourceType        *string          `json:"source_type,omitempty"`
	MimeType          *string          `json:"mime_type,omitempty"`
	FileSize          *int             `json:"file_size,omitempty"`
	TrackID           *string          `json:"track_id,omitempty"`
	TenantID          *string          `json:"tenant_id,omitempty"`
	WorkspaceID       *string          `json:"workspace_id,omitempty"`
	CreatedAt         *string          `json:"created_at,omitempty"`
	UpdatedAt         *string          `json:"updated_at,omitempty"`
	ProcessedAt       *string          `json:"processed_at,omitempty"`
	Lineage           *DocumentLineage `json:"lineage,omitempty"`
	Metadata          json.RawMessage  `json:"metadata,omitempty"`
	PDFID             *string          `json:"pdf_id,omitempty"`
}

// DocumentLineage matches Rust DocumentLineage exactly.
type DocumentLineage struct {
	LLMModel             *string  `json:"llm_model,omitempty"`
	EmbeddingModel       *string  `json:"embedding_model,omitempty"`
	EmbeddingDimensions  *int     `json:"embedding_dimensions,omitempty"`
	Keywords             []string `json:"keywords,omitempty"`
	EntityTypes          []string `json:"entity_types,omitempty"`
	RelationshipTypes    []string `json:"relationship_types,omitempty"`
	ChunkingStrategy     *string  `json:"chunking_strategy,omitempty"`
	AvgChunkSize         *int     `json:"avg_chunk_size,omitempty"`
	ProcessingDurationMs *uint64  `json:"processing_duration_ms,omitempty"`
	InputTokens          *int     `json:"input_tokens,omitempty"`
	OutputTokens         *int     `json:"output_tokens,omitempty"`
	TotalTokens          *int     `json:"total_tokens,omitempty"`
	CostUSD              *float64 `json:"cost_usd,omitempty"`
}

// StatusCounts matches Rust StatusCounts exactly.
type StatusCounts struct {
	Pending        int `json:"pending"`
	Processing     int `json:"processing"`
	Completed      int `json:"completed"`
	PartialFailure int `json:"partial_failure"`
	Failed         int `json:"failed"`
	Cancelled      int `json:"cancelled"`
}

// UploadDocumentResponse matches Rust UploadDocumentResponse exactly.
type UploadDocumentResponse struct {
	DocumentID        string            `json:"document_id"`
	Status            string            `json:"status"`
	TaskID            *string           `json:"task_id,omitempty"`
	TrackID           string            `json:"track_id"`
	DuplicateOf       *string           `json:"duplicate_of,omitempty"`
	ChunkCount        *int              `json:"chunk_count,omitempty"`
	EntityCount       *int              `json:"entity_count,omitempty"`
	RelationshipCount *int              `json:"relationship_count,omitempty"`
	Cost              *DocumentCostInfo `json:"cost,omitempty"`
}

// DocumentCostInfo matches Rust DocumentCostInfo exactly.
type DocumentCostInfo struct {
	TotalCostUSD   float64 `json:"total_cost_usd"`
	FormattedCost  string  `json:"formatted_cost"`
	InputTokens    int     `json:"input_tokens"`
	OutputTokens   int     `json:"output_tokens"`
	TotalTokens    int     `json:"total_tokens"`
	LLMModel       *string `json:"llm_model,omitempty"`
	EmbeddingModel *string `json:"embedding_model,omitempty"`
}

// FileUploadResponse matches Rust FileUploadResponse exactly.
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

// DeleteDocumentResponse matches Rust DeleteDocumentResponse exactly.
type DeleteDocumentResponse struct {
	DocumentID            string `json:"document_id"`
	Deleted               bool   `json:"deleted"`
	ChunksDeleted         int    `json:"chunks_deleted"`
	EntitiesAffected      int    `json:"entities_affected"`
	RelationshipsAffected int    `json:"relationships_affected"`
}

// DeleteAllDocumentsResponse matches Rust DeleteAllDocumentsResponse exactly.
type DeleteAllDocumentsResponse struct {
	DeletedCount              int      `json:"deleted_count"`
	TotalChunksDeleted        int      `json:"total_chunks_deleted"`
	TotalEntitiesRemoved      int      `json:"total_entities_removed"`
	TotalRelationshipsRemoved int      `json:"total_relationships_removed"`
	TotalPdfsDeleted          int      `json:"total_pdfs_deleted"`
	SkippedCount              int      `json:"skipped_count"`
	SkippedDocuments          []string `json:"skipped_documents"`
}

// DeletionImpactResponse matches Rust DeletionImpactResponse exactly.
type DeletionImpactResponse struct {
	DocumentID            string `json:"document_id"`
	ChunksToDelete        int    `json:"chunks_to_delete"`
	EntitiesToRemove      int    `json:"entities_to_remove"`
	EntitiesToUpdate      int    `json:"entities_to_update"`
	RelationshipsToRemove int    `json:"relationships_to_remove"`
	RelationshipsToUpdate int    `json:"relationships_to_update"`
	PreviewOnly           bool   `json:"preview_only"`
}

// BatchUploadResponse matches Rust BatchUploadResponse exactly.
type BatchUploadResponse struct {
	TotalFiles int               `json:"total_files"`
	Processed  int               `json:"processed"`
	Duplicates int               `json:"duplicates"`
	Failed     int               `json:"failed"`
	Results    []BatchFileResult `json:"results"`
}

// BatchFileResult matches Rust BatchFileResult exactly.
type BatchFileResult struct {
	Filename   string  `json:"filename"`
	DocumentID *string `json:"document_id,omitempty"`
	Status     string  `json:"status"`
	Error      *string `json:"error,omitempty"`
}

// TrackStatusResponse matches Rust TrackStatusResponse exactly.
type TrackStatusResponse struct {
	TrackID       string            `json:"track_id"`
	CreatedAt     *string           `json:"created_at,omitempty"`
	Documents     []DocumentSummary `json:"documents"`
	TotalCount    int               `json:"total_count"`
	StatusSummary StatusCounts      `json:"status_summary"`
	IsComplete    bool              `json:"is_complete"`
	LatestMessage *string           `json:"latest_message,omitempty"`
}

// ScanDocumentsRequest matches Rust ScanDirectoryRequest exactly.
type ScanDocumentsRequest struct {
	Path            string   `json:"path"`
	Extensions      []string `json:"extensions,omitempty"`
	Recursive       bool     `json:"recursive"`
	MaxFiles        int      `json:"max_files"`
	AsyncProcessing bool     `json:"async_processing"`
	TrackID         *string  `json:"track_id,omitempty"`
}

// ScanDocumentsResponse matches Rust ScanDirectoryResponse exactly.
type ScanDocumentsResponse struct {
	TrackID      string        `json:"track_id"`
	FilesFound   int           `json:"files_found"`
	FilesQueued  int           `json:"files_queued"`
	FilesSkipped int           `json:"files_skipped"`
	QueuedFiles  []string      `json:"queued_files"`
	SkippedFiles []SkippedFile `json:"skipped_files"`
}

// SkippedFile matches Rust SkippedFile exactly.
type SkippedFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// RecoverStuckResponse matches Rust RecoverStuckResponse exactly.
type RecoverStuckResponse struct {
	TrackID        string   `json:"track_id"`
	StuckFound     int      `json:"stuck_found"`
	Requeued       int      `json:"requeued"`
	DocumentIDs    []string `json:"document_ids"`
	DocumentTitles []string `json:"document_titles"`
}

// ReprocessFailedResponse matches Rust ReprocessFailedResponse exactly.
type ReprocessFailedResponse struct {
	TrackID     string   `json:"track_id"`
	FailedFound int      `json:"failed_found"`
	Requeued    int      `json:"requeued"`
	DocumentIDs []string `json:"document_ids"`
}

// RetryChunksResponse matches Rust RetryChunksResponse exactly.
type RetryChunksResponse struct {
	DocumentID   string `json:"document_id"`
	ChunksQueued int    `json:"chunks_queued"`
	ChunkIndices []int  `json:"chunk_indices"`
	Message      string `json:"message"`
	Implemented  bool   `json:"implemented"`
}

// FailedChunkInfo matches Rust FailedChunkInfo exactly.
type FailedChunkInfo struct {
	ChunkIndex    int    `json:"chunk_index"`
	ChunkID       string `json:"chunk_id"`
	ErrorMessage  string `json:"error_message"`
	WasTimeout    bool   `json:"was_timeout"`
	RetryAttempts int    `json:"retry_attempts"`
	Status        string `json:"status"`
}

// ListFailedChunksResponse matches Rust ListFailedChunksResponse exactly.
type ListFailedChunksResponse struct {
	DocumentID       string            `json:"document_id"`
	FailedChunks     []FailedChunkInfo `json:"failed_chunks"`
	TotalChunks      int               `json:"total_chunks"`
	SuccessfulChunks int               `json:"successful_chunks"`
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

// TaskRetryResponse represents the response from retrying a task.
type TaskRetryResponse struct {
	TrackID string `json:"track_id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// PdfRetryResponse represents the response from retrying PDF processing.
type PdfRetryResponse struct {
	Success bool   `json:"success"`
	PdfID   string `json:"pdf_id"`
	Message string `json:"message"`
	TaskID  string `json:"task_id,omitempty"`
}

// ReprocessResponse represents the response from reprocessing documents.
// Uses the same structure as ReprocessFailedResponse.
type ReprocessResponse = ReprocessFailedResponse
