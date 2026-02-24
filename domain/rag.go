package domain

import (
	"context"

	"github.com/pgvector/pgvector-go"
)

// EmbeddingProvider generates embeddings from text.
type EmbeddingProvider interface {
	CreateEmbeddings(ctx context.Context, texts []string, dataType string) ([]pgvector.Vector, error)
}
