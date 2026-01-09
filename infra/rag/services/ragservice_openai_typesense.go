package services

import (
	"context"
	"fmt"

	"school-with-ai/infra/rag/embeddingproviders"
	"school-with-ai/infra/rag/embeddingstore"
	"school-with-ai/services/chatwrite/rag"

	"github.com/rs/zerolog/log"
)

// OpenAITypesenseRAG implements the RAG interface using OpenAI for embeddings
// and Typesense for vector storage and retrieval
type OpenAITypesenseRAG struct {
	embeddings *embeddingproviders.OpenAIEmbeddingProvider
	store      *embeddingstore.Typesense
}

// Verify OpenAITypesenseRAG implements RAG interface at compile time
var _ rag.RAG = (*OpenAITypesenseRAG)(nil)

// OpenAITypesenseConfig holds configuration for the OpenAI + Typesense RAG implementation
type OpenAITypesenseConfig struct {
	// OpenAI configuration
	OpenAIAPIKey  string
	OpenAIBaseURL string // Optional custom base URL
	OpenAIModel   string // e.g., "text-embedding-ada-002", "text-embedding-3-small"

	// Typesense configuration
	TypesenseURL        string // Typesense server URL (e.g., "http://typesense:8108")
	TypesenseAPIKey     string // Typesense API key
	TypesenseCollection string // Collection name (optional, defaults to "school-with-ai-documents")
}

// NewOpenAITypesenseRAG creates a new RAG implementation using OpenAI embeddings and Typesense
func NewOpenAITypesenseRAG(cfg *OpenAITypesenseConfig) (*OpenAITypesenseRAG, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	// Create OpenAI embedding provider
	embeddingProvider, err := embeddingproviders.NewOpenAIEmbeddingProvider(&embeddingproviders.OpenAIEmbeddingConfig{
		APIKey:  cfg.OpenAIAPIKey,
		BaseURL: cfg.OpenAIBaseURL,
		Model:   cfg.OpenAIModel,
		Logger:  log.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI embedding provider: %w", err)
	}

	// Create Typesense client
	typesenseStore, err := embeddingstore.NewTypesense(&embeddingstore.TypesenseConfig{
		URL:        cfg.TypesenseURL,
		APIKey:     cfg.TypesenseAPIKey,
		Collection: cfg.TypesenseCollection,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Typesense client: %w", err)
	}

	return &OpenAITypesenseRAG{
		embeddings: embeddingProvider,
		store:      typesenseStore,
	}, nil
}

// Index adds or updates embeddings for the given chunks
// This method generates embeddings using OpenAI and stores them in Typesense
func (r *OpenAITypesenseRAG) Index(ctx context.Context, chunks ...*rag.SessionRAGIndexChunk) error {
	if len(chunks) == 0 {
		return fmt.Errorf("no chunks provided for indexing")
	}

	log.Info().
		Int("count", len(chunks)).
		Msg("indexing chunks with OpenAI embeddings and Typesense")

	// Extract text content from chunks
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	// Generate embeddings using OpenAI
	embeddings, err := r.embeddings.CreateEmbeddings(ctx, texts, "document")
	if err != nil {
		return fmt.Errorf("failed to create embeddings: %w", err)
	}

	if len(embeddings) != len(chunks) {
		return fmt.Errorf("embedding count mismatch: expected %d, got %d", len(chunks), len(embeddings))
	}

	// Attach embeddings to chunks
	for i, chunk := range chunks {
		chunk.Embedding = embeddings[i]
	}

	// Index chunks in Typesense
	// Note: Typesense can auto-embed if configured, but we're providing pre-computed embeddings
	if err := r.store.Index(ctx, chunks...); err != nil {
		return fmt.Errorf("failed to index in Typesense: %w", err)
	}

	log.Info().
		Int("count", len(chunks)).
		Str("model", r.embeddings.GetModel()).
		Msg("successfully indexed chunks with OpenAI embeddings in Typesense")

	return nil
}

// Query searches for similar chunks based on the query text
// This method generates an embedding for the query using OpenAI and searches in Typesense
func (r *OpenAITypesenseRAG) Query(ctx context.Context, q *rag.SessionRAGQuery) ([]*rag.SessionRAGResult, error) {
	if q == nil {
		return nil, fmt.Errorf("query is required")
	}

	if q.Prompt == "" {
		return nil, fmt.Errorf("query prompt is required")
	}

	log.Info().
		Str("prompt", q.Prompt).
		Str("entity_id", q.DataEntityID).
		Msg("querying with OpenAI embeddings and Typesense")

	// Generate embedding for the query using OpenAI
	embeddings, err := r.embeddings.CreateEmbeddings(ctx, []string{q.Prompt}, "query")
	if err != nil {
		return nil, fmt.Errorf("failed to create query embedding: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings generated for query")
	}

	// Attach embedding to query
	q.Embedding = embeddings[0]

	// Search in Typesense using hybrid search (semantic + keyword)
	results, err := r.store.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("failed to query Typesense: %w", err)
	}

	log.Info().
		Int("result_count", len(results)).
		Str("model", r.embeddings.GetModel()).
		Msg("successfully queried with OpenAI embeddings from Typesense")

	return results, nil
}

// Delete removes indexed chunks from Typesense
func (r *OpenAITypesenseRAG) Delete(ctx context.Context, req *rag.DeleteIndexRequest) error {
	if req == nil {
		return fmt.Errorf("delete request is required")
	}

	log.Info().
		Str("entity_id", req.DataEntityID).
		Str("document_id", req.DocumentID).
		Str("document_group_id", req.DocumentGroupID).
		Msg("deleting chunks from Typesense")

	// Delete from Typesense
	if err := r.store.Delete(ctx, req); err != nil {
		return fmt.Errorf("failed to delete from Typesense: %w", err)
	}

	log.Info().
		Str("entity_id", req.DataEntityID).
		Msg("successfully deleted chunks from Typesense")

	return nil
}

// GetEmbeddingDimensions returns the dimensionality of the embeddings used
func (r *OpenAITypesenseRAG) GetEmbeddingDimensions() int {
	return rag.GetDimensions(r.embeddings.GetModel())
}

// GetEmbeddingModel returns the OpenAI model being used for embeddings
func (r *OpenAITypesenseRAG) GetEmbeddingModel() string {
	return r.embeddings.GetModel()
}
