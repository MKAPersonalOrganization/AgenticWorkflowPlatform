// Package storage provides vector storage implementations.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// VectorRecord represents a single vector with its metadata.
type VectorRecord struct {
	ID       string                 `json:"id"`
	Vector   []float64              `json:"vector"`
	Metadata map[string]interface{} `json:"metadata"`
}

// NanoVectorDBStorage implements BaseVectorStorage using an in-memory vector database.
type NanoVectorDBStorage struct {
	namespace       string
	workspace       string
	globalConfig    map[string]interface{}
	embeddingFunc   EmbeddingFunc
	embeddingDim    int
	cosineThreshold float64
	metaFields      []string

	records     map[string]*VectorRecord
	mu          sync.RWMutex
	dirty       bool
	initialized bool

	logger *logrus.Logger
}

// NewNanoVectorDBStorage creates a new NanoVectorDBStorage instance.
func NewNanoVectorDBStorage(config *StorageConfig, logger *logrus.Logger) *NanoVectorDBStorage {
	if logger == nil {
		logger = logrus.New()
	}
	cosineThreshold := config.CosineThreshold
	if cosineThreshold == 0 {
		cosineThreshold = 0.2
	}
	return &NanoVectorDBStorage{
		namespace:       config.Namespace,
		workspace:       config.Workspace,
		globalConfig:    config.GlobalConfig,
		embeddingFunc:   config.EmbeddingFunc,
		embeddingDim:    config.EmbeddingDim,
		cosineThreshold: cosineThreshold,
		metaFields:      config.MetaFields,
		records:         make(map[string]*VectorRecord),
		logger:          logger,
	}
}

// Namespace returns the namespace for this storage.
func (s *NanoVectorDBStorage) Namespace() string {
	return s.namespace
}

// GetNamespace returns the namespace for this storage.
func (s *NanoVectorDBStorage) GetNamespace() string {
	return s.namespace
}

// Workspace returns the workspace path.
func (s *NanoVectorDBStorage) Workspace() string {
	return s.workspace
}

// GetWorkspace returns the workspace path.
func (s *NanoVectorDBStorage) GetWorkspace() string {
	return s.workspace
}

// getFilePath returns the path to the vector storage file.
func (s *NanoVectorDBStorage) getFilePath() string {
	return filepath.Join(s.workspace, fmt.Sprintf("vdb_%s.json", s.namespace))
}

// Initialize sets up the storage by loading existing data from disk.
func (s *NanoVectorDBStorage) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	filePath := s.getFilePath()

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		s.records = make(map[string]*VectorRecord)
		s.initialized = true
		s.logger.WithField("namespace", s.namespace).Debug("NanoVectorDBStorage initialized with empty data")
		return nil
	}

	// Load existing data
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %w", err)
	}

	var records []*VectorRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("failed to parse storage file: %w", err)
	}

	s.records = make(map[string]*VectorRecord)
	for _, record := range records {
		s.records[record.ID] = record
	}

	s.initialized = true
	s.logger.WithFields(logrus.Fields{
		"namespace": s.namespace,
		"count":     len(s.records),
	}).Debug("NanoVectorDBStorage initialized with existing data")

	return nil
}

// Finalize cleans up storage resources.
func (s *NanoVectorDBStorage) Finalize(ctx context.Context) error {
	return s.IndexDoneCallback(ctx)
}

// IndexDoneCallback commits storage operations by persisting data to disk.
func (s *NanoVectorDBStorage) IndexDoneCallback(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.dirty {
		return nil
	}

	if err := os.MkdirAll(s.workspace, 0o755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	filePath := s.getFilePath()

	records := make([]*VectorRecord, 0, len(s.records))
	for _, record := range s.records {
		records = append(records, record)
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write storage file: %w", err)
	}

	s.dirty = false
	s.logger.WithFields(logrus.Fields{
		"namespace": s.namespace,
		"count":     len(s.records),
	}).Debug("NanoVectorDBStorage persisted to disk")

	return nil
}

// Drop removes all data from storage.
func (s *NanoVectorDBStorage) Drop(ctx context.Context) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = make(map[string]*VectorRecord)
	s.dirty = false

	filePath := s.getFilePath()
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return map[string]string{
			"status":  "error",
			"message": fmt.Sprintf("failed to remove file: %v", err),
		}, nil
	}

	s.logger.WithField("namespace", s.namespace).Debug("NanoVectorDBStorage dropped")

	return map[string]string{
		"status":  "success",
		"message": "data dropped",
	}, nil
}

// cosineSimilarity calculates the cosine similarity between two vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// QueryResult represents a single query result with similarity score.
type QueryResult struct {
	ID       string
	Score    float64
	Metadata map[string]interface{}
	Vector   []float64
}

// Query searches the vector storage and retrieves top_k results.
func (s *NanoVectorDBStorage) Query(ctx context.Context, query string, topK int, queryEmbedding []float64) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.records) == 0 {
		return []map[string]interface{}{}, nil
	}

	// Get query embedding if not provided
	var embedding []float64
	var err error
	if queryEmbedding != nil {
		embedding = queryEmbedding
	} else if s.embeddingFunc != nil {
		embeddings, err := s.embeddingFunc(ctx, []string{query})
		if err != nil {
			return nil, fmt.Errorf("failed to compute query embedding: %w", err)
		}
		if len(embeddings) == 0 {
			return nil, fmt.Errorf("empty embedding returned for query")
		}
		embedding = embeddings[0]
	} else {
		return nil, fmt.Errorf("no embedding function configured and no query embedding provided")
	}

	// Calculate similarities
	results := make([]QueryResult, 0, len(s.records))
	for _, record := range s.records {
		if len(record.Vector) != len(embedding) {
			continue
		}
		score := cosineSimilarity(embedding, record.Vector)
		if score >= s.cosineThreshold {
			results = append(results, QueryResult{
				ID:       record.ID,
				Score:    score,
				Metadata: record.Metadata,
				Vector:   record.Vector,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit to topK
	if len(results) > topK {
		results = results[:topK]
	}

	// Convert to return format
	output := make([]map[string]interface{}, len(results))
	for i, r := range results {
		item := make(map[string]interface{})
		item["id"] = r.ID
		item["distance"] = 1 - r.Score // Convert similarity to distance
		item["score"] = r.Score
		for k, v := range r.Metadata {
			item[k] = v
		}
		output[i] = item
	}

	return output, err
}

// Upsert inserts or updates vectors in the storage.
func (s *NanoVectorDBStorage) Upsert(ctx context.Context, data map[string]map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, item := range data {
		record := &VectorRecord{
			ID:       id,
			Metadata: make(map[string]interface{}),
		}

		// Extract vector
		if vec, ok := item["vector"]; ok {
			switch v := vec.(type) {
			case []float64:
				record.Vector = v
			case []interface{}:
				record.Vector = make([]float64, len(v))
				for i, val := range v {
					if f, ok := val.(float64); ok {
						record.Vector[i] = f
					}
				}
			}
			delete(item, "vector")
		}

		// Copy remaining metadata
		for k, v := range item {
			record.Metadata[k] = v
		}

		s.records[id] = record
	}

	s.dirty = true
	return nil
}

// DeleteEntity deletes a single entity by its name.
func (s *NanoVectorDBStorage) DeleteEntity(ctx context.Context, entityName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find and delete records matching entity name
	toDelete := make([]string, 0)
	normalizedName := strings.ToLower(entityName)

	for id, record := range s.records {
		if name, ok := record.Metadata["entity_name"].(string); ok {
			if strings.ToLower(name) == normalizedName {
				toDelete = append(toDelete, id)
			}
		}
	}

	for _, id := range toDelete {
		delete(s.records, id)
	}

	if len(toDelete) > 0 {
		s.dirty = true
	}

	return nil
}

// DeleteEntityRelation deletes relations for a given entity.
func (s *NanoVectorDBStorage) DeleteEntityRelation(ctx context.Context, entityName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	toDelete := make([]string, 0)
	normalizedName := strings.ToLower(entityName)

	for id, record := range s.records {
		// Check source entity
		if srcName, ok := record.Metadata["src_id"].(string); ok {
			if strings.ToLower(srcName) == normalizedName {
				toDelete = append(toDelete, id)
				continue
			}
		}
		// Check target entity
		if tgtName, ok := record.Metadata["tgt_id"].(string); ok {
			if strings.ToLower(tgtName) == normalizedName {
				toDelete = append(toDelete, id)
			}
		}
	}

	for _, id := range toDelete {
		delete(s.records, id)
	}

	if len(toDelete) > 0 {
		s.dirty = true
	}

	return nil
}

// GetByID retrieves vector data by its ID.
func (s *NanoVectorDBStorage) GetByID(ctx context.Context, id string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if record, ok := s.records[id]; ok {
		result := make(map[string]interface{})
		result["id"] = record.ID
		result["vector"] = record.Vector
		for k, v := range record.Metadata {
			result[k] = v
		}
		return result, nil
	}

	return nil, nil
}

// GetByIDs retrieves multiple vector data by their IDs.
func (s *NanoVectorDBStorage) GetByIDs(ctx context.Context, ids []string) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]map[string]interface{}, 0, len(ids))

	for _, id := range ids {
		if record, ok := s.records[id]; ok {
			result := make(map[string]interface{})
			result["id"] = record.ID
			result["vector"] = record.Vector
			for k, v := range record.Metadata {
				result[k] = v
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// Delete removes vectors with specified IDs.
func (s *NanoVectorDBStorage) Delete(ctx context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		delete(s.records, id)
	}

	s.dirty = true
	return nil
}

// GetVectorsByIDs retrieves vectors by their IDs.
func (s *NanoVectorDBStorage) GetVectorsByIDs(ctx context.Context, ids []string) (map[string][]float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string][]float64)

	for _, id := range ids {
		if record, ok := s.records[id]; ok {
			result[id] = record.Vector
		}
	}

	return result, nil
}

// Count returns the number of records in storage.
func (s *NanoVectorDBStorage) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.records)
}

// SetEmbeddingFunc sets the embedding function.
func (s *NanoVectorDBStorage) SetEmbeddingFunc(fn EmbeddingFunc) {
	s.embeddingFunc = fn
}

// SetCosineThreshold sets the cosine similarity threshold.
func (s *NanoVectorDBStorage) SetCosineThreshold(threshold float64) {
	s.cosineThreshold = threshold
}

// GetCosineBetterThanThreshold returns the cosine similarity threshold.
func (s *NanoVectorDBStorage) GetCosineBetterThanThreshold() float64 {
	return s.cosineThreshold
}

// GetMetaFields returns the set of meta fields.
func (s *NanoVectorDBStorage) GetMetaFields() map[string]struct{} {
	result := make(map[string]struct{}, len(s.metaFields))
	for _, field := range s.metaFields {
		result[field] = struct{}{}
	}
	return result
}
