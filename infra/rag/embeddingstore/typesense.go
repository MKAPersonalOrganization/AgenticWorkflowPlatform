package embeddingstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"school-with-ai/domain"

	"github.com/avast/retry-go/v4"
	"github.com/rs/zerolog/log"
	"github.com/typesense/typesense-go/v2/typesense"
	"github.com/typesense/typesense-go/v2/typesense/api"
	"github.com/typesense/typesense-go/v2/typesense/api/pointer"
)

const (
	defaultCollection = "school-with-ai-documents"
	defaultModelName  = "ts/all-MiniLM-L12-v2"
)

// TypesenseConfig holds configuration for the Typesense implementation
type TypesenseConfig struct {
	URL        string // Typesense server URL (e.g., "http://typesense:8108")
	APIKey     string // Typesense API key
	Collection string // Collection name (optional, defaults to "school-with-ai-documents")
}

// Typesense implements the RAG interface using Typesense vector database
type Typesense struct {
	client     *typesense.Client
	collection string
	ready      chan struct{}
}

// Verify Typesense implements RAG interface at compile time
var _ domain.RAG = (*Typesense)(nil)

// NewTypesense creates a new Typesense instance
func NewTypesense(cfg *TypesenseConfig) (*Typesense, error) {
	client := typesense.NewClient(
		typesense.WithServer(cfg.URL),
		typesense.WithAPIKey(cfg.APIKey),
		typesense.WithNumRetries(3),
		typesense.WithConnectionTimeout(300*time.Second),
	)

	collection := cfg.Collection
	if collection == "" {
		collection = defaultCollection
	}

	t := &Typesense{
		client:     client,
		collection: collection,
		ready:      make(chan struct{}),
	}

	go t.waitForTypesense()

	return t, nil
}

// waitForTypesense waits for Typesense to be ready and ensures the collection exists
func (t *Typesense) waitForTypesense() {
	err := retry.Do(func() error {
		healthy, err := t.client.Health(context.Background(), 5*time.Second)
		if err != nil {
			return err
		}

		if !healthy {
			return fmt.Errorf("typesense is not healthy yet")
		}

		return t.ensureCollection(context.Background())
	},
		retry.Attempts(0),
		retry.Delay(2*time.Second),
		retry.MaxDelay(10*time.Second),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			log.Warn().
				Err(err).
				Uint("retries", n).
				Msg("waiting for typesense to come up")
		}),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to connect to typesense")
		return
	}

	log.Info().Msg("typesense is up and collection is ready")
	close(t.ready)
}

// ensureReady waits for Typesense to be ready or context to be cancelled
func (t *Typesense) ensureReady(ctx context.Context) error {
	select {
	case <-t.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Index adds or updates embeddings for the given chunks
func (t *Typesense) Index(ctx context.Context, indexReqs ...*domain.SessionRAGIndexChunk) error {
	if err := t.ensureReady(ctx); err != nil {
		return err
	}
	return t.index(ctx, indexReqs...)
}

// index performs the actual indexing operation
func (t *Typesense) index(ctx context.Context, indexReqs ...*domain.SessionRAGIndexChunk) error {
	if len(indexReqs) == 0 {
		return fmt.Errorf("no index requests provided")
	}

	// Convert our types to Typesense document format
	docs := make([]interface{}, 0, len(indexReqs))
	for _, indexReq := range indexReqs {
		doc := map[string]interface{}{
			"data_entity_id":    indexReq.DataEntityID,
			"document_id":       indexReq.DocumentID,
			"document_group_id": indexReq.DocumentGroupID,
			"content":           indexReq.Content,
			"content_offset":    indexReq.ContentOffset,
		}

		// Add optional fields if present
		if indexReq.Source != "" {
			doc["source"] = indexReq.Source
		}
		if indexReq.ToolID != "" {
			doc["tool_id"] = indexReq.ToolID
		}

		docs = append(docs, doc)
	}

	if len(indexReqs) == 1 {
		_, err := t.client.Collection(t.collection).Documents().Create(ctx, docs[0])
		return err
	}

	// For multiple index requests, use the import API
	params := &api.ImportDocumentsParams{
		Action:    pointer.String("create"),
		BatchSize: pointer.Int(len(indexReqs)),
	}

	_, err := t.client.Collection(t.collection).Documents().Import(ctx, docs, params)
	if err != nil {
		return fmt.Errorf("error importing documents: %w", err)
	}

	log.Info().
		Int("count", len(indexReqs)).
		Msg("successfully indexed chunks")

	return nil
}

// Query searches for similar chunks based on the query
func (t *Typesense) Query(ctx context.Context, q *domain.SessionRAGQuery) ([]*domain.SessionRAGResult, error) {
	if err := t.ensureReady(ctx); err != nil {
		return nil, err
	}

	searchParameters := &api.SearchCollectionParams{
		Q:       pointer.String(q.Prompt),
		QueryBy: pointer.String("content,embedding"),
		// Gives more weight to content matches
		QueryByWeights: pointer.String("2,1"),
		FilterBy:       pointer.String("data_entity_id:" + q.DataEntityID),
		SortBy:         pointer.String("_text_match:desc,_vector_distance:asc"),
		// Setting this to true will make Typesense consider all variations of prefixes and
		// typo corrections of the words in the query exhaustively, without stopping early
		// when enough results are found
		ExhaustiveSearch: pointer.True(),
		// Turn off prefix search for content and embedding
		Prefix:        pointer.String("false,false"),
		ExcludeFields: pointer.String("embedding"), // Don't return the raw floating point numbers in the vector field
	}

	// Handle tool ID filter if present
	if q.ToolID != "" {
		toolFilter := fmt.Sprintf("tool_id:%s", q.ToolID)
		searchParameters.FilterBy = pointer.String(fmt.Sprintf("%s && %s", *searchParameters.FilterBy, toolFilter))
	}

	// Handle document ID list filter if present
	if len(q.DocumentIDList) > 0 {
		documentIDFilter := fmt.Sprintf("document_id:[%s]", strings.Join(q.DocumentIDList, ","))
		searchParameters.FilterBy = pointer.String(fmt.Sprintf("%s && %s", *searchParameters.FilterBy, documentIDFilter))
	}

	if q.MaxResults > 0 {
		searchParameters.Limit = pointer.Int(q.MaxResults)
	}

	results, err := t.client.Collection(t.collection).Documents().Search(ctx, searchParameters)
	if err != nil {
		return nil, err
	}

	log.Info().Int("num_results", len(*results.Hits)).Msg("typesense results")

	ragResults := make([]*domain.SessionRAGResult, 0, len(*results.Hits))
	for _, hit := range *results.Hits {
		ragResult := &domain.SessionRAGResult{
			DataEntityID:    getStrVariable(&hit, "data_entity_id"),
			DocumentGroupID: getStrVariable(&hit, "document_group_id"),
			DocumentID:      getStrVariable(&hit, "document_id"),
			Source:          getStrVariable(&hit, "source"),
			Content:         getStrVariable(&hit, "content"),
			ContentOffset:   getIntVariable(&hit, "content_offset"),
			Distance:        0, // Typesense uses _text_match and _vector_distance, not exposed directly
		}
		ragResults = append(ragResults, ragResult)
	}

	return ragResults, nil
}

// Delete removes embeddings for the specified entity/document
func (t *Typesense) Delete(ctx context.Context, req *domain.DeleteIndexRequest) error {
	if err := t.ensureReady(ctx); err != nil {
		return err
	}

	entityID := req.EntityID
	if entityID == "" {
		entityID = req.DataEntityID
	}

	filterParts := []string{fmt.Sprintf("data_entity_id:%s", entityID)}

	if req.DocumentID != "" {
		filterParts = append(filterParts, fmt.Sprintf("document_id:%s", req.DocumentID))
	}

	if req.DocumentGroupID != "" {
		filterParts = append(filterParts, fmt.Sprintf("document_group_id:%s", req.DocumentGroupID))
	}

	params := &api.DeleteDocumentsParams{
		FilterBy: pointer.String(strings.Join(filterParts, " && ")),
	}

	_, err := t.client.Collection(t.collection).Documents().Delete(ctx, params)
	if err != nil {
		log.Error().
			Err(err).
			Str("entity_id", entityID).
			Str("document_id", req.DocumentID).
			Msg("failed to delete embeddings")
		return err
	}

	log.Info().
		Str("entity_id", entityID).
		Str("document_id", req.DocumentID).
		Msg("successfully deleted embeddings")

	return nil
}

// Helper functions to extract values from Typesense search results

func getStrVariable(hit *api.SearchResultHit, key string) string {
	val, ok := (*hit.Document)[key]
	if !ok {
		return ""
	}

	str, ok := val.(string)
	if !ok {
		return ""
	}

	return str
}

func getIntVariable(hit *api.SearchResultHit, key string) int {
	val, ok := (*hit.Document)[key]
	if !ok {
		return 0
	}

	return int(val.(float64))
}

// ensureCollection creates the Typesense collection if it doesn't exist
func (t *Typesense) ensureCollection(ctx context.Context) error {
	log.Info().Str("collection", t.collection).Msg("ensuring collection")

	// Check if collection exists
	collections, err := t.client.Collections().Retrieve(ctx)
	if err != nil {
		return err
	}

	for _, collection := range collections {
		if collection.Name == t.collection {
			log.Info().Str("collection", t.collection).Msg("collection already exists")
			return nil
		}
	}

	// Create collection schema with automatic embedding generation
	schema := &api.CollectionSchema{
		Name: t.collection,
		Fields: []api.Field{
			{
				Name: "data_entity_id",
				Type: "string",
			},
			{
				Name: "document_group_id",
				Type: "string",
			},
			{
				Name:  "document_id",
				Type:  "string",
				Facet: pointer.True(),
			},
			{
				Name:     "source",
				Type:     "string",
				Optional: pointer.True(),
			},
			{
				Name: "content",
				Type: "string",
			},
			{
				Name: "content_offset",
				Type: "int32",
			},
			{
				Name:     "tool_id",
				Type:     "string",
				Optional: pointer.True(),
			},
			{
				Name: "embedding",
				Type: "float[]",
				Embed: &struct {
					From        []string `json:"from"`
					ModelConfig struct {
						AccessToken  *string `json:"access_token,omitempty"`
						ApiKey       *string `json:"api_key,omitempty"`
						ClientId     *string `json:"client_id,omitempty"`
						ClientSecret *string `json:"client_secret,omitempty"`
						ModelName    string  `json:"model_name"`
						ProjectId    *string `json:"project_id,omitempty"`
					} `json:"model_config"`
				}{
					From: []string{"content"},
					ModelConfig: struct {
						AccessToken  *string `json:"access_token,omitempty"`
						ApiKey       *string `json:"api_key,omitempty"`
						ClientId     *string `json:"client_id,omitempty"`
						ClientSecret *string `json:"client_secret,omitempty"`
						ModelName    string  `json:"model_name"`
						ProjectId    *string `json:"project_id,omitempty"`
					}{
						ModelName: defaultModelName,
					},
				},
			},
		},
		DefaultSortingField: pointer.String("content_offset"),
	}

	_, err = t.client.Collections().Create(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	log.Info().Str("collection", t.collection).Msg("successfully created collection")

	return nil
}
