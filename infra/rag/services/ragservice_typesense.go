package services

import (
	"context"
	"school-with-ai/infra/rag/embeddingstore"
	"school-with-ai/services/chatwrite/rag"
)

// TypesenseRAG implements the RAG interface using Typesense vector database
type TypesenseRAG struct {
	store *embeddingstore.Typesense
}

// Verify TypesenseRAG implements RAG interface at compile time
var _ rag.RAG = (*TypesenseRAG)(nil)

// TypesenseConfig holds configuration for the Typesense RAG implementation
type TypesenseConfig struct {
	URL        string // Typesense server URL (e.g., "http://typesense:8108")
	APIKey     string // Typesense API key
	Collection string // Collection name (optional, defaults to "school-with-ai-documents")
}

// NewTypesenseRAG creates a new Typesense RAG instance
func NewTypesenseRAG(cfg *TypesenseConfig) (*TypesenseRAG, error) {
	store, err := embeddingstore.NewTypesense(&embeddingstore.TypesenseConfig{
		URL:        cfg.URL,
		APIKey:     cfg.APIKey,
		Collection: cfg.Collection,
	})
	if err != nil {
		return nil, err
	}

	return &TypesenseRAG{
		store: store,
	}, nil
}

// Index adds or updates embeddings for the given chunks
func (t *TypesenseRAG) Index(ctx context.Context, indexReqs ...*rag.SessionRAGIndexChunk) error {
	return t.store.Index(ctx, indexReqs...)
}

// Query searches for similar chunks based on the query
func (t *TypesenseRAG) Query(ctx context.Context, q *rag.SessionRAGQuery) ([]*rag.SessionRAGResult, error) {
	return t.store.Query(ctx, q)
}

// Delete removes chunks by various criteria
func (t *TypesenseRAG) Delete(ctx context.Context, req *rag.DeleteIndexRequest) error {
	return t.store.Delete(ctx, req)
}
