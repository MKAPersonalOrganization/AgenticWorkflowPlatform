package workspaces

import (
	"time"

	"github.com/google/uuid"
)

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
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
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
}
