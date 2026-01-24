package embeddingproviders

import (
	"context"
	"fmt"

	"school-with-ai/domain"

	"github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAIEmbeddingProvider implements the EmbeddingProvider interface using OpenAI
type OpenAIEmbeddingProvider struct {
	client *openai.Client
	model  string
	logger zerolog.Logger
}

// Verify OpenAIEmbeddingProvider implements EmbeddingProvider interface at compile time
var _ domain.EmbeddingProvider = (*OpenAIEmbeddingProvider)(nil)

// OpenAIEmbeddingConfig holds configuration for the OpenAI embedding provider
type OpenAIEmbeddingConfig struct {
	APIKey  string // OpenAI API key
	BaseURL string // Optional custom base URL
	Model   string // e.g., "text-embedding-ada-002", "text-embedding-3-small"
	Logger  zerolog.Logger
}

// NewOpenAIEmbeddingProvider creates a new OpenAI embedding provider
func NewOpenAIEmbeddingProvider(cfg *OpenAIEmbeddingConfig) (*OpenAIEmbeddingProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if cfg.Model == "" {
		cfg.Model = "text-embedding-ada-002" // Default embedding model
	}

	// Create OpenAI client
	config := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		config.BaseURL = cfg.BaseURL
	}
	client := openai.NewClientWithConfig(config)

	return &OpenAIEmbeddingProvider{
		client: client,
		model:  cfg.Model,
		logger: cfg.Logger,
	}, nil
}

// CreateEmbeddings generates embeddings for the given texts using OpenAI
func (p *OpenAIEmbeddingProvider) CreateEmbeddings(ctx context.Context, texts []string, dataType string) ([]pgvector.Vector, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided for embedding")
	}

	p.logger.Debug().
		Int("count", len(texts)).
		Str("model", p.model).
		Str("data_type", dataType).
		Msg("creating embeddings")

	// Create embedding request
	req := openai.EmbeddingRequest{
		Input: texts,
		Model: openai.EmbeddingModel(p.model),
	}

	// Generate embeddings
	resp, err := p.client.CreateEmbeddings(ctx, req)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("model", p.model).
			Msg("failed to create embeddings")
		return nil, fmt.Errorf("failed to create embeddings: %w", err)
	}

	if len(resp.Data) == 0 {
		p.logger.Error().
			Str("model", p.model).
			Msg("no embeddings returned")
		return nil, fmt.Errorf("no embeddings returned")
	}

	// Convert to pgvector format
	vectors := make([]pgvector.Vector, len(resp.Data))
	for i, data := range resp.Data {
		if len(data.Embedding) == 0 {
			p.logger.Error().
				Int("index", i).
				Str("model", p.model).
				Msg("empty embedding returned")
			return nil, fmt.Errorf("empty embedding returned for text %d", i)
		}

		vector := pgvector.NewVector(data.Embedding)
		vectors[i] = vector

		p.logger.Debug().
			Int("index", i).
			Int("embedding_dimensions", len(data.Embedding)).
			Str("model", p.model).
			Msg("successfully created embedding")
	}

	p.logger.Info().
		Int("count", len(vectors)).
		Str("model", p.model).
		Str("data_type", dataType).
		Msg("successfully created embeddings")

	return vectors, nil
}

// GetModel returns the model name being used
func (p *OpenAIEmbeddingProvider) GetModel() string {
	return p.model
}
