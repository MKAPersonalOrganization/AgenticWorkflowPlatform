// Package models provides request/response models for the LightRAG API.
package models

import (
	"time"

	"github.com/lightrag/lightrag-go/pkg/types"
)

// QueryRequest represents a query request.
type QueryRequest struct {
	Query               string                      `json:"query" binding:"required,min=3"`
	Mode                types.QueryMode             `json:"mode"`
	OnlyNeedContext     *bool                       `json:"only_need_context,omitempty"`
	OnlyNeedPrompt      *bool                       `json:"only_need_prompt,omitempty"`
	ResponseType        *string                     `json:"response_type,omitempty"`
	TopK                *int                        `json:"top_k,omitempty" binding:"omitempty,min=1"`
	ChunkTopK           *int                        `json:"chunk_top_k,omitempty" binding:"omitempty,min=1"`
	MaxEntityTokens     *int                        `json:"max_entity_tokens,omitempty" binding:"omitempty,min=1"`
	MaxRelationTokens   *int                        `json:"max_relation_tokens,omitempty" binding:"omitempty,min=1"`
	MaxTotalTokens      *int                        `json:"max_total_tokens,omitempty" binding:"omitempty,min=1"`
	HLKeywords          []string                    `json:"hl_keywords,omitempty"`
	LLKeywords          []string                    `json:"ll_keywords,omitempty"`
	ConversationHistory []types.ConversationMessage `json:"conversation_history,omitempty"`
	UserPrompt          *string                     `json:"user_prompt,omitempty"`
	EnableRerank        *bool                       `json:"enable_rerank,omitempty"`
	IncludeReferences   *bool                       `json:"include_references,omitempty"`
	IncludeChunkContent *bool                       `json:"include_chunk_content,omitempty"`
	Stream              *bool                       `json:"stream,omitempty"`
}

// ToQueryParams converts QueryRequest to QueryParam.
func (r *QueryRequest) ToQueryParams(isStream bool) *types.QueryParam {
	param := types.NewQueryParam()

	if r.Mode != "" {
		param.Mode = r.Mode
	}
	if r.OnlyNeedContext != nil {
		param.OnlyNeedContext = *r.OnlyNeedContext
	}
	if r.OnlyNeedPrompt != nil {
		param.OnlyNeedPrompt = *r.OnlyNeedPrompt
	}
	if r.ResponseType != nil {
		param.ResponseType = *r.ResponseType
	}
	if r.TopK != nil {
		param.TopK = *r.TopK
	}
	if r.ChunkTopK != nil {
		param.ChunkTopK = *r.ChunkTopK
	}
	if r.MaxEntityTokens != nil {
		param.MaxEntityTokens = *r.MaxEntityTokens
	}
	if r.MaxRelationTokens != nil {
		param.MaxRelationTokens = *r.MaxRelationTokens
	}
	if r.MaxTotalTokens != nil {
		param.MaxTotalTokens = *r.MaxTotalTokens
	}
	if r.HLKeywords != nil {
		param.HLKeywords = r.HLKeywords
	}
	if r.LLKeywords != nil {
		param.LLKeywords = r.LLKeywords
	}
	if r.ConversationHistory != nil {
		param.ConversationHistory = r.ConversationHistory
	}
	if r.UserPrompt != nil {
		param.UserPrompt = r.UserPrompt
	}
	if r.EnableRerank != nil {
		param.EnableRerank = *r.EnableRerank
	}
	if r.IncludeReferences != nil {
		param.IncludeReferences = *r.IncludeReferences
	}

	param.Stream = isStream

	return param
}

// ReferenceItem represents a reference in query responses.
type ReferenceItem struct {
	ReferenceID string   `json:"reference_id"`
	FilePath    string   `json:"file_path"`
	Content     []string `json:"content,omitempty"`
}

// QueryResponse represents a query response.
type QueryResponse struct {
	Response   string          `json:"response"`
	References []ReferenceItem `json:"references,omitempty"`
}

// QueryDataResponse represents a detailed query response.
type QueryDataResponse struct {
	Status   string                 `json:"status"`
	Message  string                 `json:"message"`
	Data     map[string]interface{} `json:"data"`
	Metadata map[string]interface{} `json:"metadata"`
}

// StreamChunkResponse represents a streaming chunk response.
type StreamChunkResponse struct {
	References []ReferenceItem `json:"references,omitempty"`
	Response   *string         `json:"response,omitempty"`
	Error      *string         `json:"error,omitempty"`
}

// InsertTextRequest represents a request to insert text.
type InsertTextRequest struct {
	Text       string `json:"text" binding:"required,min=1"`
	FileSource string `json:"file_source,omitempty"`
}

// InsertTextsRequest represents a request to insert multiple texts.
type InsertTextsRequest struct {
	Texts       []string `json:"texts" binding:"required,min=1,dive,min=1"`
	FileSources []string `json:"file_sources,omitempty"`
}

// InsertResponse represents the response for insert operations.
type InsertResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	TrackID string `json:"track_id"`
}

// ScanResponse represents the response for scan operations.
type ScanResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	TrackID string `json:"track_id"`
}

// ReprocessResponse represents the response for reprocess operations.
type ReprocessResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	TrackID string `json:"track_id"`
}

// CancelPipelineResponse represents the response for pipeline cancellation.
type CancelPipelineResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// DocumentStatus represents the status of a document.
type DocumentStatus struct {
	ID              string          `json:"id"`
	FileName        string          `json:"file_name"`
	FilePath        string          `json:"file_path"`
	Status          types.DocStatus `json:"status"`
	CreatedAt       *time.Time      `json:"created_at,omitempty"`
	UpdatedAt       *time.Time      `json:"updated_at,omitempty"`
	ChunksTotal     int             `json:"chunks_total"`
	ChunksProcessed int             `json:"chunks_processed"`
	Error           string          `json:"error,omitempty"`
	TrackID         string          `json:"track_id,omitempty"`
}

// PaginatedDocumentsResponse represents a paginated list of documents.
type PaginatedDocumentsResponse struct {
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
	Documents  []DocumentStatus `json:"documents"`
}

// PipelineStatusResponse represents the pipeline status.
type PipelineStatusResponse struct {
	IsBusy         bool   `json:"is_busy"`
	DocsInProgress int    `json:"docs_in_progress"`
	DocsRemaining  int    `json:"docs_remaining"`
	CurrentTrackID string `json:"current_track_id,omitempty"`
	LatestTrackID  string `json:"latest_track_id,omitempty"`
	JobsInQueue    int    `json:"jobs_in_queue"`
	HistoryCount   int    `json:"history_count"`
}

// DeleteDocumentResponse represents the response for document deletion.
type DeleteDocumentResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Deleted int    `json:"deleted"`
}

// EntityUpdateRequest represents a request to update an entity.
type EntityUpdateRequest struct {
	EntityName  string                 `json:"entity_name" binding:"required"`
	UpdatedData map[string]interface{} `json:"updated_data" binding:"required"`
	AllowRename bool                   `json:"allow_rename"`
	AllowMerge  bool                   `json:"allow_merge"`
}

// RelationUpdateRequest represents a request to update a relation.
type RelationUpdateRequest struct {
	SourceID    string                 `json:"source_id" binding:"required"`
	TargetID    string                 `json:"target_id" binding:"required"`
	UpdatedData map[string]interface{} `json:"updated_data" binding:"required"`
}

// EntityMergeRequest represents a request to merge entities.
type EntityMergeRequest struct {
	EntitiesToChange   []string `json:"entities_to_change" binding:"required,min=1"`
	EntityToChangeInto string   `json:"entity_to_change_into" binding:"required,min=1"`
}

// EntityCreateRequest represents a request to create an entity.
type EntityCreateRequest struct {
	EntityName string                 `json:"entity_name" binding:"required,min=1"`
	EntityData map[string]interface{} `json:"entity_data" binding:"required"`
}

// RelationCreateRequest represents a request to create a relation.
type RelationCreateRequest struct {
	SourceEntity string                 `json:"source_entity" binding:"required,min=1"`
	TargetEntity string                 `json:"target_entity" binding:"required,min=1"`
	RelationData map[string]interface{} `json:"relation_data" binding:"required"`
}

// GraphLabelsResponse represents the response for graph labels.
type GraphLabelsResponse struct {
	Labels []string `json:"labels"`
}

// EntityExistsResponse represents the response for entity existence check.
type EntityExistsResponse struct {
	Exists bool `json:"exists"`
}

// OperationSummary represents the summary of an operation.
type OperationSummary struct {
	Merged          bool    `json:"merged"`
	MergeStatus     string  `json:"merge_status"`
	MergeError      *string `json:"merge_error,omitempty"`
	OperationStatus string  `json:"operation_status"`
	TargetEntity    *string `json:"target_entity,omitempty"`
	FinalEntity     string  `json:"final_entity"`
	Renamed         bool    `json:"renamed"`
}

// EntityUpdateResponse represents the response for entity update.
type EntityUpdateResponse struct {
	Status           string                 `json:"status"`
	Message          string                 `json:"message"`
	Data             map[string]interface{} `json:"data"`
	OperationSummary OperationSummary       `json:"operation_summary"`
}

// OllamaMessage represents a message in Ollama format.
type OllamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

// OllamaChatRequest represents an Ollama chat request.
type OllamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []OllamaMessage        `json:"messages"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
	System   *string                `json:"system,omitempty"`
}

// OllamaChatResponse represents an Ollama chat response.
type OllamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// OllamaGenerateRequest represents an Ollama generate request.
type OllamaGenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	System  *string                `json:"system,omitempty"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// OllamaGenerateResponse represents an Ollama generate response.
type OllamaGenerateResponse struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	Context            []int  `json:"context,omitempty"`
	TotalDuration      *int64 `json:"total_duration,omitempty"`
	LoadDuration       *int64 `json:"load_duration,omitempty"`
	PromptEvalCount    *int   `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration *int64 `json:"prompt_eval_duration,omitempty"`
	EvalCount          *int   `json:"eval_count,omitempty"`
	EvalDuration       *int64 `json:"eval_duration,omitempty"`
}

// OllamaVersionResponse represents an Ollama version response.
type OllamaVersionResponse struct {
	Version string `json:"version"`
}

// OllamaModelDetails represents Ollama model details.
type OllamaModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// OllamaModel represents an Ollama model.
type OllamaModel struct {
	Name       string             `json:"name"`
	Model      string             `json:"model"`
	Size       int64              `json:"size"`
	Digest     string             `json:"digest"`
	ModifiedAt string             `json:"modified_at"`
	Details    OllamaModelDetails `json:"details"`
}

// OllamaTagResponse represents an Ollama tag response.
type OllamaTagResponse struct {
	Models []OllamaModel `json:"models"`
}

// OllamaRunningModel represents a running Ollama model.
type OllamaRunningModel struct {
	Name      string             `json:"name"`
	Model     string             `json:"model"`
	Size      int64              `json:"size"`
	Digest    string             `json:"digest"`
	Details   OllamaModelDetails `json:"details"`
	ExpiresAt string             `json:"expires_at"`
	SizeVRAM  int64              `json:"size_vram"`
}

// OllamaPsResponse represents an Ollama ps response.
type OllamaPsResponse struct {
	Models []OllamaRunningModel `json:"models"`
}

// HealthResponse represents a health check response.
type HealthResponse struct {
	Status  string                 `json:"status"`
	Version string                 `json:"version"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Username string `json:"username" form:"username" binding:"required"`
	Password string `json:"password" form:"password" binding:"required"`
}

// LoginResponse represents a login response.
type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Detail string `json:"detail"`
}

// ValidationErrorResponse represents a validation error response.
type ValidationErrorResponse struct {
	Detail []ValidationErrorDetail `json:"detail"`
}

// ValidationErrorDetail represents a validation error detail.
type ValidationErrorDetail struct {
	Loc  []string `json:"loc"`
	Msg  string   `json:"msg"`
	Type string   `json:"type"`
}
