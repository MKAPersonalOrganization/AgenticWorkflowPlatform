package rag

import "school-with-ai/domain"

// Re-export domain types for backward compatibility
type (
	RAG                  = domain.RAG
	EmbeddingProvider    = domain.EmbeddingProvider
	EmbeddingsStore      = domain.EmbeddingsStore
	SessionRAGIndexChunk = domain.SessionRAGIndexChunk
	SessionRAGQuery      = domain.SessionRAGQuery
	SessionRAGResult     = domain.SessionRAGResult
	DeleteIndexRequest   = domain.DeleteIndexRequest
)

// GetDimensions returns the expected dimensions for a given embedding model.
// Deprecated: Use domain.GetEmbeddingDimensions instead.
func GetDimensions(model string) int {
	return domain.GetEmbeddingDimensions(model)
}
