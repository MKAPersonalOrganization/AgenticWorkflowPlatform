package domain

import (
	"context"

	"github.com/pgvector/pgvector-go"
)

// RAG defines the interface for RAG (Retrieval-Augmented Generation) operations.
type RAG interface {
	Index(ctx context.Context, req ...*SessionRAGIndexChunk) error
	Query(ctx context.Context, q *SessionRAGQuery) ([]*SessionRAGResult, error)
	Delete(ctx context.Context, req *DeleteIndexRequest) error
}

// EmbeddingProvider generates embeddings from text.
type EmbeddingProvider interface {
	CreateEmbeddings(ctx context.Context, texts []string, dataType string) ([]pgvector.Vector, error)
}

// EmbeddingsStore handles storage and retrieval of embeddings in PostgreSQL.
type EmbeddingsStore interface {
	CreateKnowledgeEmbedding(ctx context.Context, embeddings ...*SessionRAGIndexChunk) error
	QueryKnowledgeEmbeddings(ctx context.Context, query *SessionRAGQuery) ([]*SessionRAGResult, error)
	DeleteKnowledgeEmbedding(ctx context.Context, entityID, documentID, documentGroupID string) error
}

// SessionRAGIndexChunk represents a chunk of content to be indexed for RAG.
type SessionRAGIndexChunk struct {
	DataEntityID    string          `json:"data_entity_id"`
	DocumentID      string          `json:"document_id"`
	DocumentGroupID string          `json:"document_group_id"`
	ContentOffset   int             `json:"content_offset"`
	Content         string          `json:"content"`
	Source          string          `json:"source,omitempty"`
	ToolID          string          `json:"tool_id,omitempty"`
	Embedding       pgvector.Vector `json:"-"`
}

// SessionRAGQuery represents a query to the RAG system.
type SessionRAGQuery struct {
	Prompt            string          `json:"prompt"`
	DataEntityID      string          `json:"data_entity_id"`
	ToolID            string          `json:"tool_id,omitempty"`
	DocumentIDList    []string        `json:"document_id_list,omitempty"`
	Embedding         pgvector.Vector `json:"-"`
	DistanceMetric    string          `json:"distance_metric"`
	DistanceThreshold float64         `json:"distance_threshold"`
	MaxResults        int             `json:"max_results"`
}

// SessionRAGResult represents a result from a RAG query.
type SessionRAGResult struct {
	DataEntityID    string  `json:"data_entity_id"`
	DocumentID      string  `json:"document_id"`
	DocumentGroupID string  `json:"document_group_id"`
	ContentOffset   int     `json:"content_offset"`
	Content         string  `json:"content"`
	Source          string  `json:"source,omitempty"`
	Distance        float64 `json:"distance"`
}

// DeleteIndexRequest represents a request to delete indexed content.
type DeleteIndexRequest struct {
	DataEntityID    string `json:"data_entity_id"`
	DocumentID      string `json:"document_id,omitempty"`
	DocumentGroupID string `json:"document_group_id,omitempty"`
	EntityID        string `json:"entity_id,omitempty"` // Alias for DataEntityID
}

// GetEmbeddingDimensions returns the expected dimensions for a given embedding model.
func GetEmbeddingDimensions(model string) int {
	dimensionMap := map[string]int{
		"text-embedding-3-small":                 1536,
		"text-embedding-3-large":                 3072,
		"text-embedding-ada-002":                 1536,
		"thenlper/gte-small":                     384,
		"thenlper/gte-base":                      768,
		"thenlper/gte-large":                     1024,
		"sentence-transformers/all-MiniLM-L6-v2": 384,
	}

	if dim, ok := dimensionMap[model]; ok {
		return dim
	}

	// Default to OpenAI's standard dimension
	return 1536
}
