package workspaces

import "github.com/google/uuid"

// CreateWorkspaceRequest represents the payload to create a new workspace.
type CreateWorkspaceRequest struct {
	Name               string  `json:"name"`
	Slug               *string `json:"slug,omitempty"`
	Description        *string `json:"description,omitempty"`
	MaxDocuments       *int    `json:"max_documents,omitempty"`
	LlmModel           *string `json:"llm_model,omitempty"`
	LlmProvider        *string `json:"llm_provider,omitempty"`
	EmbeddingModel     *string `json:"embedding_model,omitempty"`
	EmbeddingProvider  *string `json:"embedding_provider,omitempty"`
	EmbeddingDimension *int    `json:"embedding_dimension,omitempty"`
	VisionProvider     *string `json:"vision_provider,omitempty"`
	VisionModel        *string `json:"vision_model,omitempty"`
}

// WorkspaceResponse represents a single workspace returned by the API.
type WorkspaceResponse struct {
	ID                 uuid.UUID `json:"id"`
	TenantID           uuid.UUID `json:"tenant_id"`
	Name               string    `json:"name"`
	Slug               string    `json:"slug"`
	Description        *string   `json:"description,omitempty"`
	IsActive           bool      `json:"is_active"`
	MaxDocuments       *int      `json:"max_documents,omitempty"`
	LlmModel           string    `json:"llm_model"`
	LlmProvider        string    `json:"llm_provider"`
	LlmFullID          string    `json:"llm_full_id"`
	EmbeddingModel     string    `json:"embedding_model"`
	EmbeddingProvider  string    `json:"embedding_provider"`
	EmbeddingDimension int       `json:"embedding_dimension"`
	EmbeddingFullID    string    `json:"embedding_full_id"`
	VisionProvider     *string   `json:"vision_provider,omitempty"`
	VisionModel        *string   `json:"vision_model,omitempty"`
	CreatedAt          string    `json:"created_at"`
	UpdatedAt          string    `json:"updated_at"`
}

// WorkspaceListResponse represents a paginated list of workspaces.
type WorkspaceListResponse struct {
	Items  []WorkspaceResponse `json:"items"`
	Total  int                 `json:"total"`
	Offset int                 `json:"offset"`
	Limit  int                 `json:"limit"`
}

// WorkspaceStatsResponse represents the statistics for a workspace.
type WorkspaceStatsResponse struct {
	WorkspaceID       uuid.UUID `json:"workspace_id"`
	DocumentCount     int       `json:"document_count"`
	EntityCount       int       `json:"entity_count"`
	RelationshipCount int       `json:"relationship_count"`
	EntityTypeCount   int       `json:"entity_type_count"`
	ChunkCount        int       `json:"chunk_count"`
	EmbeddingCount    int       `json:"embedding_count"`
	StorageBytes      uint64    `json:"storage_bytes"`
}

// UpdateWorkspaceRequest represents the payload to update an existing workspace.
type UpdateWorkspaceRequest struct {
	Name               *string `json:"name,omitempty"`
	Description        *string `json:"description,omitempty"`
	IsActive           *bool   `json:"is_active,omitempty"`
	MaxDocuments       *int    `json:"max_documents,omitempty"`
	LlmModel           *string `json:"llm_model,omitempty"`
	LlmProvider        *string `json:"llm_provider,omitempty"`
	EmbeddingModel     *string `json:"embedding_model,omitempty"`
	EmbeddingProvider  *string `json:"embedding_provider,omitempty"`
	EmbeddingDimension *int    `json:"embedding_dimension,omitempty"`
	VisionProvider     *string `json:"vision_provider,omitempty"`
	VisionModel        *string `json:"vision_model,omitempty"`
}

// RebuildKnowledgeGraphRequest represents a request to rebuild the knowledge graph.
type RebuildKnowledgeGraphRequest struct {
	LlmModel          *string `json:"llm_model,omitempty"`
	LlmProvider       *string `json:"llm_provider,omitempty"`
	Force             bool    `json:"force"`
	RebuildEmbeddings bool    `json:"rebuild_embeddings"`
	MaxDocuments      int     `json:"max_documents,omitempty"`
}

// RebuildKnowledgeGraphResponse represents the response from rebuilding the graph.
type RebuildKnowledgeGraphResponse struct {
	WorkspaceID          uuid.UUID `json:"workspace_id"`
	Status               string    `json:"status"`
	NodesCleared         int       `json:"nodes_cleared"`
	EdgesCleared         int       `json:"edges_cleared"`
	VectorsCleared       int       `json:"vectors_cleared"`
	DocumentsToProcess   int       `json:"documents_to_process"`
	ChunksToProcess      int       `json:"chunks_to_process"`
	LlmModel             string    `json:"llm_model"`
	LlmProvider          string    `json:"llm_provider"`
	EstimatedTimeSeconds *uint64   `json:"estimated_time_seconds,omitempty"`
	TrackID              *string   `json:"track_id,omitempty"`
}

// RebuildEmbeddingsRequest represents a request to rebuild embeddings.
type RebuildEmbeddingsRequest struct {
	EmbeddingModel     *string `json:"embedding_model,omitempty"`
	EmbeddingProvider  *string `json:"embedding_provider,omitempty"`
	EmbeddingDimension *int    `json:"embedding_dimension,omitempty"`
	Force              bool    `json:"force"`
}

// RebuildEmbeddingsResponse represents the response from rebuilding embeddings.
type RebuildEmbeddingsResponse struct {
	WorkspaceID          uuid.UUID `json:"workspace_id"`
	Status               string    `json:"status"`
	DocumentsToProcess   int       `json:"documents_to_process"`
	ChunksToProcess      int       `json:"chunks_to_process"`
	VectorsCleared       int       `json:"vectors_cleared"`
	EmbeddingModel       string    `json:"embedding_model"`
	EmbeddingProvider    string    `json:"embedding_provider"`
	EmbeddingDimension   int       `json:"embedding_dimension"`
	ModelContextLength   int       `json:"model_context_length"`
	EstimatedTimeSeconds *uint64   `json:"estimated_time_seconds,omitempty"`
	JobID                *string   `json:"job_id,omitempty"`
	CompatibilityWarning *string   `json:"compatibility_warning,omitempty"`
}

// ReprocessAllRequest represents a request to reprocess all documents in a workspace.
type ReprocessAllRequest struct {
	IncludeCompleted bool `json:"include_completed"`
	MaxDocuments     int  `json:"max_documents,omitempty"`
}

// ReprocessAllResponse represents the response from reprocessing all documents.
type ReprocessAllResponse struct {
	TrackID              string    `json:"track_id"`
	WorkspaceID          uuid.UUID `json:"workspace_id"`
	Status               string    `json:"status"`
	DocumentsFound       int       `json:"documents_found"`
	DocumentsQueued      int       `json:"documents_queued"`
	DocumentsSkipped     int       `json:"documents_skipped"`
	EstimatedTimeSeconds *uint64   `json:"estimated_time_seconds,omitempty"`
}

// MetricsSnapshotDTO represents a single metrics snapshot for historical data.
type MetricsSnapshotDTO struct {
	ID                uuid.UUID `json:"id"`
	RecordedAt        string    `json:"recorded_at"`
	TriggerType       string    `json:"trigger_type"`
	DocumentCount     int64     `json:"document_count"`
	ChunkCount        int64     `json:"chunk_count"`
	EntityCount       int64     `json:"entity_count"`
	RelationshipCount int64     `json:"relationship_count"`
	EmbeddingCount    int64     `json:"embedding_count"`
	StorageBytes      int64     `json:"storage_bytes"`
}

// MetricsHistoryResponse represents workspace metrics history with pagination.
type MetricsHistoryResponse struct {
	WorkspaceID uuid.UUID            `json:"workspace_id"`
	Snapshots   []MetricsSnapshotDTO `json:"snapshots"`
	Count       int                  `json:"count"`
	Offset      int                  `json:"offset"`
	Limit       int                  `json:"limit"`
}
