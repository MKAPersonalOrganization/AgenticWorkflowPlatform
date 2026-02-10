// Package storage provides storage interfaces and implementations for LightRAG.
package storage

import (
	"context"
)

// StorageNameSpace represents the base interface for all storage types.
type StorageNameSpace interface {
	// Namespace returns the namespace for this storage.
	Namespace() string

	// Workspace returns the workspace path for this storage.
	Workspace() string

	// Initialize sets up the storage.
	Initialize(ctx context.Context) error

	// Finalize cleans up the storage resources.
	Finalize(ctx context.Context) error

	// IndexDoneCallback commits storage operations after indexing.
	IndexDoneCallback(ctx context.Context) error

	// Drop removes all data from storage and cleans up resources.
	Drop(ctx context.Context) (map[string]string, error)
}

// DropResult represents the result of a drop operation.
type DropResult struct {
	Status  string `json:"status"`  // "success" or "error"
	Message string `json:"message"` // "data dropped" on success, error details on failure
}

// BaseKVStorage defines the interface for key-value storage.
type BaseKVStorage interface {
	StorageNameSpace

	// GetByID retrieves a value by its ID.
	GetByID(ctx context.Context, id string) (map[string]interface{}, error)

	// GetByIDs retrieves multiple values by their IDs.
	GetByIDs(ctx context.Context, ids []string) ([]map[string]interface{}, error)

	// FilterKeys returns keys that don't exist in the storage.
	FilterKeys(ctx context.Context, keys []string) ([]string, error)

	// Upsert inserts or updates data.
	Upsert(ctx context.Context, data map[string]map[string]interface{}) error

	// Delete removes specific records by their IDs.
	Delete(ctx context.Context, ids []string) error

	// IsEmpty checks if the storage is empty.
	IsEmpty(ctx context.Context) (bool, error)
}

// BaseVectorStorage defines the interface for vector storage.
type BaseVectorStorage interface {
	StorageNameSpace

	// Query searches the vector storage and retrieves top_k results.
	Query(ctx context.Context, query string, topK int, queryEmbedding []float64) ([]map[string]interface{}, error)

	// Upsert inserts or updates vectors in the storage.
	Upsert(ctx context.Context, data map[string]map[string]interface{}) error

	// DeleteEntity deletes a single entity by its name.
	DeleteEntity(ctx context.Context, entityName string) error

	// DeleteEntityRelation deletes relations for a given entity.
	DeleteEntityRelation(ctx context.Context, entityName string) error

	// GetByID retrieves vector data by its ID.
	GetByID(ctx context.Context, id string) (map[string]interface{}, error)

	// GetByIDs retrieves multiple vector data by their IDs.
	GetByIDs(ctx context.Context, ids []string) ([]map[string]interface{}, error)

	// Delete removes vectors with specified IDs.
	Delete(ctx context.Context, ids []string) error

	// GetVectorsByIDs retrieves vectors by their IDs, returning only ID and vector data.
	GetVectorsByIDs(ctx context.Context, ids []string) (map[string][]float64, error)
}

// BaseGraphStorage defines the interface for graph storage.
type BaseGraphStorage interface {
	StorageNameSpace

	// HasNode checks if a node exists in the graph.
	HasNode(ctx context.Context, nodeID string) (bool, error)

	// HasEdge checks if an edge exists between two nodes.
	HasEdge(ctx context.Context, sourceNodeID, targetNodeID string) (bool, error)

	// NodeDegree gets the degree of a node.
	NodeDegree(ctx context.Context, nodeID string) (int, error)

	// EdgeDegree gets the total degree of an edge.
	EdgeDegree(ctx context.Context, srcID, tgtID string) (int, error)

	// GetNode retrieves a node by its ID.
	GetNode(ctx context.Context, nodeID string) (map[string]string, error)

	// GetEdge retrieves edge properties between two nodes.
	GetEdge(ctx context.Context, sourceNodeID, targetNodeID string) (map[string]string, error)

	// GetNodeEdges retrieves all edges connected to a node.
	GetNodeEdges(ctx context.Context, sourceNodeID string) ([][2]string, error)

	// GetNodesBatch retrieves multiple nodes by their IDs.
	GetNodesBatch(ctx context.Context, nodeIDs []string) (map[string]map[string]string, error)

	// NodeDegreesBatch retrieves degrees for multiple nodes.
	NodeDegreesBatch(ctx context.Context, nodeIDs []string) (map[string]int, error)

	// EdgeDegreesBatch retrieves degrees for multiple edges.
	EdgeDegreesBatch(ctx context.Context, edgePairs [][2]string) (map[[2]string]int, error)

	// GetEdgesBatch retrieves multiple edges.
	GetEdgesBatch(ctx context.Context, pairs []EdgePair) (map[[2]string]map[string]string, error)

	// GetNodesEdgesBatch retrieves edges for multiple nodes.
	GetNodesEdgesBatch(ctx context.Context, nodeIDs []string) (map[string][][2]string, error)

	// UpsertNode inserts or updates a node.
	UpsertNode(ctx context.Context, nodeID string, nodeData map[string]string) error

	// UpsertEdge inserts or updates an edge.
	UpsertEdge(ctx context.Context, sourceNodeID, targetNodeID string, edgeData map[string]string) error

	// DeleteNode removes a node from the graph.
	DeleteNode(ctx context.Context, nodeID string) error

	// DeleteEdge removes an edge from the graph.
	DeleteEdge(ctx context.Context, sourceNodeID, targetNodeID string) error

	// GetKnowledgeGraph retrieves a subgraph centered on specified nodes.
	GetKnowledgeGraph(ctx context.Context, nodeLabel string, maxDepth int) (interface{}, error)
}

// EdgePair represents a source-target node pair for edge operations.
type EdgePair struct {
	Src string `json:"src"`
	Tgt string `json:"tgt"`
}

// EmbeddingFunc is a function type for computing embeddings.
type EmbeddingFunc func(ctx context.Context, texts []string) ([][]float64, error)

// StorageConfig holds common configuration for storage implementations.
type StorageConfig struct {
	Namespace       string
	Workspace       string
	GlobalConfig    map[string]interface{}
	EmbeddingFunc   EmbeddingFunc
	EmbeddingDim    int
	CosineThreshold float64
	MetaFields      []string
}

// NewStorageConfig creates a new StorageConfig with default values.
func NewStorageConfig(namespace, workspace string) *StorageConfig {
	return &StorageConfig{
		Namespace:       namespace,
		Workspace:       workspace,
		GlobalConfig:    make(map[string]interface{}),
		CosineThreshold: 0.2,
		MetaFields:      make([]string, 0),
	}
}
