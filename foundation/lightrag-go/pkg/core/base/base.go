// Package base provides core interfaces and types for LightRAG storage backends.
package base

import (
	"context"
	"errors"
)

// Common errors for storage operations.
var (
	ErrNotFound       = errors.New("not found")
	ErrNotImplemented = errors.New("not implemented")
	ErrInvalidInput   = errors.New("invalid input")
)

// TextChunkSchema represents a text chunk with metadata.
type TextChunkSchema struct {
	Tokens          int    `json:"tokens"`
	Content         string `json:"content"`
	FullDocID       string `json:"full_doc_id"`
	ChunkOrderIndex int    `json:"chunk_order_index"`
}

// StorageNameSpace is the base interface for all storage types.
type StorageNameSpace interface {
	// GetNamespace returns the namespace of this storage.
	GetNamespace() string

	// GetWorkspace returns the workspace of this storage.
	GetWorkspace() string

	// Initialize initializes the storage.
	Initialize(ctx context.Context) error

	// Finalize finalizes the storage and releases resources.
	Finalize(ctx context.Context) error

	// IndexDoneCallback commits storage operations after indexing.
	IndexDoneCallback(ctx context.Context) error

	// Drop drops all data from storage and cleans up resources.
	Drop(ctx context.Context) (map[string]string, error)
}

// BaseStorageConfig holds common configuration for all storage types.
type BaseStorageConfig struct {
	Namespace    string
	Workspace    string
	GlobalConfig map[string]interface{}
}

// BaseVectorStorage defines the interface for vector storage backends.
type BaseVectorStorage interface {
	StorageNameSpace

	// Query searches the vector storage and retrieves top_k results.
	// If queryEmbedding is provided, it skips embedding computation.
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

	// Delete deletes vectors with specified IDs.
	Delete(ctx context.Context, ids []string) error

	// GetVectorsByIDs gets vectors by their IDs, returning only ID and vector data.
	GetVectorsByIDs(ctx context.Context, ids []string) (map[string][]float64, error)

	// GetCosineBetterThanThreshold returns the cosine similarity threshold.
	GetCosineBetterThanThreshold() float64

	// GetMetaFields returns the set of meta fields.
	GetMetaFields() map[string]struct{}
}

// BaseKVStorage defines the interface for key-value storage backends.
type BaseKVStorage interface {
	StorageNameSpace

	// GetByID retrieves a value by its ID.
	GetByID(ctx context.Context, id string) (map[string]interface{}, error)

	// GetByIDs retrieves multiple values by their IDs.
	GetByIDs(ctx context.Context, ids []string) ([]map[string]interface{}, error)

	// FilterKeys returns keys that don't exist in storage.
	FilterKeys(ctx context.Context, keys map[string]struct{}) (map[string]struct{}, error)

	// Upsert inserts or updates data.
	Upsert(ctx context.Context, data map[string]map[string]interface{}) error

	// Delete removes specific records by their IDs.
	Delete(ctx context.Context, ids []string) error

	// IsEmpty checks if the storage is empty.
	IsEmpty(ctx context.Context) (bool, error)

	// GetAll returns all stored data.
	GetAll(ctx context.Context) (map[string]map[string]interface{}, error)
}

// BaseGraphStorage defines the interface for graph storage backends.
// All edge operations are undirected unless explicitly stated.
type BaseGraphStorage interface {
	StorageNameSpace

	// HasNode checks if a node exists in the graph.
	HasNode(ctx context.Context, nodeID string) (bool, error)

	// HasEdge checks if an edge exists between two nodes.
	HasEdge(ctx context.Context, sourceNodeID, targetNodeID string) (bool, error)

	// NodeDegree returns the degree (number of connected edges) of a node.
	NodeDegree(ctx context.Context, nodeID string) (int, error)

	// EdgeDegree returns the total degree of an edge (sum of source and target node degrees).
	EdgeDegree(ctx context.Context, srcID, tgtID string) (int, error)

	// GetNode retrieves a node by its ID.
	GetNode(ctx context.Context, nodeID string) (map[string]string, error)

	// GetEdge retrieves edge properties between two nodes.
	GetEdge(ctx context.Context, sourceNodeID, targetNodeID string) (map[string]string, error)

	// GetNodeEdges retrieves all edges connected to a node.
	GetNodeEdges(ctx context.Context, sourceNodeID string) ([][2]string, error)

	// GetNodesBatch retrieves multiple nodes at once.
	GetNodesBatch(ctx context.Context, nodeIDs []string) (map[string]map[string]string, error)

	// NodeDegreesBatch retrieves degrees for multiple nodes.
	NodeDegreesBatch(ctx context.Context, nodeIDs []string) (map[string]int, error)

	// EdgeDegreesBatch retrieves degrees for multiple edges.
	EdgeDegreesBatch(ctx context.Context, edgePairs [][2]string) (map[[2]string]int, error)

	// GetEdgesBatch retrieves multiple edges at once.
	GetEdgesBatch(ctx context.Context, pairs []map[string]string) (map[[2]string]map[string]string, error)

	// GetNodesEdgesBatch retrieves edges for multiple nodes.
	GetNodesEdgesBatch(ctx context.Context, nodeIDs []string) (map[string][][2]string, error)

	// UpsertNode inserts or updates a node.
	UpsertNode(ctx context.Context, nodeID string, nodeData map[string]string) error

	// UpsertEdge inserts or updates an edge.
	UpsertEdge(ctx context.Context, sourceNodeID, targetNodeID string, edgeData map[string]string) error

	// DeleteNode deletes a node from the graph.
	DeleteNode(ctx context.Context, nodeID string) error

	// RemoveNodes deletes multiple nodes.
	RemoveNodes(ctx context.Context, nodes []string) error

	// RemoveEdges deletes multiple edges.
	RemoveEdges(ctx context.Context, edges [][2]string) error

	// GetAllLabels returns all labels (entity names) in the graph.
	GetAllLabels(ctx context.Context) ([]string, error)

	// GetKnowledgeGraph retrieves a connected subgraph.
	GetKnowledgeGraph(ctx context.Context, nodeLabel string, maxDepth, maxNodes int) (*KnowledgeGraph, error)

	// GetAllNodes returns all nodes in the graph.
	GetAllNodes(ctx context.Context) ([]map[string]interface{}, error)

	// GetAllEdges returns all edges in the graph.
	GetAllEdges(ctx context.Context) ([]map[string]interface{}, error)

	// GetPopularLabels returns popular labels by node degree.
	GetPopularLabels(ctx context.Context, limit int) ([]string, error)

	// SearchLabels searches labels with fuzzy matching.
	SearchLabels(ctx context.Context, query string, limit int) ([]string, error)
}

// KnowledgeGraph represents a subgraph with nodes and edges.
type KnowledgeGraph struct {
	Nodes       []KnowledgeGraphNode `json:"nodes"`
	Edges       []KnowledgeGraphEdge `json:"edges"`
	IsTruncated bool                 `json:"is_truncated"`
}

// KnowledgeGraphNode represents a node in the knowledge graph.
type KnowledgeGraphNode struct {
	ID          string            `json:"id"`
	Labels      []string          `json:"labels,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
	Description string            `json:"description,omitempty"`
	SourceID    string            `json:"source_id,omitempty"`
}

// KnowledgeGraphEdge represents an edge in the knowledge graph.
type KnowledgeGraphEdge struct {
	ID          string            `json:"id,omitempty"`
	Source      string            `json:"source"`
	Target      string            `json:"target"`
	Description string            `json:"description,omitempty"`
	Keywords    string            `json:"keywords,omitempty"`
	Weight      float64           `json:"weight,omitempty"`
	SourceID    string            `json:"source_id,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
}

// DocStatus represents document processing status.
type DocStatus string

const (
	DocStatusPending      DocStatus = "pending"
	DocStatusProcessing   DocStatus = "processing"
	DocStatusPreprocessed DocStatus = "preprocessed"
	DocStatusProcessed    DocStatus = "processed"
	DocStatusFailed       DocStatus = "failed"
)

// IsValid checks if the status is valid.
func (s DocStatus) IsValid() bool {
	switch s {
	case DocStatusPending, DocStatusProcessing, DocStatusPreprocessed, DocStatusProcessed, DocStatusFailed:
		return true
	}
	return false
}

// DocProcessingStatus represents document processing status data.
type DocProcessingStatus struct {
	ContentSummary      string                 `json:"content_summary"`
	ContentLength       int                    `json:"content_length"`
	FilePath            string                 `json:"file_path"`
	Status              DocStatus              `json:"status"`
	CreatedAt           string                 `json:"created_at"`
	UpdatedAt           string                 `json:"updated_at"`
	TrackID             string                 `json:"track_id,omitempty"`
	ChunksCount         *int                   `json:"chunks_count,omitempty"`
	ChunksList          []string               `json:"chunks_list,omitempty"`
	ErrorMsg            string                 `json:"error_msg,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	MultimodalProcessed *bool                  `json:"-"` // Internal field
}

// DocStatusStorage defines the interface for document status storage.
type DocStatusStorage interface {
	BaseKVStorage

	// GetStatusCounts returns counts of documents in each status.
	GetStatusCounts(ctx context.Context) (map[string]int, error)

	// GetDocsByStatus returns all documents with a specific status.
	GetDocsByStatus(ctx context.Context, status DocStatus) (map[string]*DocProcessingStatus, error)

	// GetDocsByTrackID returns all documents with a specific track ID.
	GetDocsByTrackID(ctx context.Context, trackID string) (map[string]*DocProcessingStatus, error)

	// GetDocsPaginated returns documents with pagination support.
	GetDocsPaginated(ctx context.Context, statusFilter *DocStatus, page, pageSize int, sortField, sortDirection string) ([]DocWithID, int, error)

	// GetAllStatusCounts returns counts of documents in each status for all documents.
	GetAllStatusCounts(ctx context.Context) (map[string]int, error)

	// GetDocByFilePath returns a document by its file path.
	GetDocByFilePath(ctx context.Context, filePath string) (map[string]interface{}, error)
}

// DocWithID pairs a document ID with its processing status.
type DocWithID struct {
	ID     string
	Status *DocProcessingStatus
}

// StoragesStatus represents the status of storage initialization.
type StoragesStatus string

const (
	StoragesStatusNotCreated  StoragesStatus = "not_created"
	StoragesStatusCreated     StoragesStatus = "created"
	StoragesStatusInitialized StoragesStatus = "initialized"
	StoragesStatusFinalized   StoragesStatus = "finalized"
)

// DeletionResult represents the result of a deletion operation.
type DeletionResult struct {
	Status     string `json:"status"` // "success", "not_found", "fail"
	DocID      string `json:"doc_id"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
	FilePath   string `json:"file_path,omitempty"`
}

// QueryResult represents a unified query result.
type QueryResult struct {
	Answer           string                   `json:"answer,omitempty"`
	Content          string                   `json:"content,omitempty"`
	RawData          map[string]interface{}   `json:"raw_data,omitempty"`
	IsStreaming      bool                     `json:"is_streaming"`
	ResponseIterator <-chan string            `json:"-"` // For streaming responses
	Context          *QueryContextResult      `json:"context,omitempty"`
	Status           string                   `json:"status,omitempty"`
	ContextChunks    []map[string]interface{} `json:"context_chunks,omitempty"`
}

// GetReferenceList extracts reference list from raw data.
func (r *QueryResult) GetReferenceList() []map[string]string {
	if r.RawData == nil {
		return nil
	}
	data, ok := r.RawData["data"].(map[string]interface{})
	if !ok {
		return nil
	}
	refs, ok := data["references"].([]map[string]string)
	if !ok {
		return nil
	}
	return refs
}

// GetMetadata extracts metadata from raw data.
func (r *QueryResult) GetMetadata() map[string]interface{} {
	if r.RawData == nil {
		return nil
	}
	metadata, _ := r.RawData["metadata"].(map[string]interface{})
	return metadata
}

// QueryContextResult represents a query context result.
type QueryContextResult struct {
	Context       string                   `json:"context"`
	RawData       map[string]interface{}   `json:"raw_data"`
	Entities      []map[string]interface{} `json:"entities,omitempty"`
	Relationships []map[string]interface{} `json:"relationships,omitempty"`
	TextChunks    []map[string]interface{} `json:"text_chunks,omitempty"`
}

// GetReferenceList extracts reference list from raw data.
func (r *QueryContextResult) GetReferenceList() []map[string]string {
	if r.RawData == nil {
		return nil
	}
	data, ok := r.RawData["data"].(map[string]interface{})
	if !ok {
		return nil
	}
	refs, ok := data["references"].([]map[string]string)
	if !ok {
		return nil
	}
	return refs
}

// EmbeddingFunc represents an embedding function with metadata.
type EmbeddingFunc struct {
	Func         func(ctx context.Context, texts []string) ([][]float64, error)
	EmbeddingDim int
	MaxTokenSize int
	ModelName    string
}

// Embed calls the underlying embedding function.
func (e *EmbeddingFunc) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if e.Func == nil {
		return nil, errors.New("embedding function not set")
	}
	return e.Func(ctx, texts)
}

// QueryParam holds parameters for query operations.
type QueryParam struct {
	Mode              string              `json:"mode,omitempty"`
	OnlyNeedContext   bool                `json:"only_need_context,omitempty"`
	OnlyNeedPrompt    bool                `json:"only_need_prompt,omitempty"`
	ResponseType      string              `json:"response_type,omitempty"`
	TopK              int                 `json:"top_k,omitempty"`
	ChunkTopK         int                 `json:"chunk_top_k,omitempty"`
	MaxTotalTokens    int                 `json:"max_total_tokens,omitempty"`
	MaxEntityTokens   int                 `json:"max_entity_tokens,omitempty"`
	MaxRelationTokens int                 `json:"max_relation_tokens,omitempty"`
	EnableRerank      bool                `json:"enable_rerank,omitempty"`
	CosineThreshold   float64             `json:"cosine_threshold,omitempty"`
	HistoryMessages   []map[string]string `json:"history_messages,omitempty"`
	IDS               []string            `json:"ids,omitempty"`
}

// NewQueryParam creates a new QueryParam with default values.
func NewQueryParam() *QueryParam {
	return &QueryParam{
		Mode:              "hybrid",
		ResponseType:      "markdown",
		TopK:              40,
		ChunkTopK:         20,
		MaxTotalTokens:    30000,
		MaxEntityTokens:   6000,
		MaxRelationTokens: 8000,
		CosineThreshold:   0.2,
	}
}
