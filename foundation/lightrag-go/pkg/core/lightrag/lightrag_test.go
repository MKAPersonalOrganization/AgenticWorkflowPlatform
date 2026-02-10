package lightrag

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lightrag/lightrag-go/pkg/core/base"
	"github.com/lightrag/lightrag-go/pkg/core/coreconstants"
	"github.com/lightrag/lightrag-go/pkg/core/operate"
)

// MockKVStorage implements base.BaseKVStorage for testing
type MockKVStorage struct {
	namespace string
	workspace string
	data      map[string]map[string]interface{}
}

func NewMockKVStorage() *MockKVStorage {
	return &MockKVStorage{
		data: make(map[string]map[string]interface{}),
	}
}

func (m *MockKVStorage) GetNamespace() string                        { return m.namespace }
func (m *MockKVStorage) GetWorkspace() string                        { return m.workspace }
func (m *MockKVStorage) Initialize(ctx context.Context) error        { return nil }
func (m *MockKVStorage) Finalize(ctx context.Context) error          { return nil }
func (m *MockKVStorage) IndexDoneCallback(ctx context.Context) error { return nil }
func (m *MockKVStorage) Drop(ctx context.Context) (map[string]string, error) {
	m.data = make(map[string]map[string]interface{})
	return nil, nil
}

func (m *MockKVStorage) GetByID(ctx context.Context, id string) (map[string]interface{}, error) {
	if data, ok := m.data[id]; ok {
		return data, nil
	}
	return nil, nil
}

func (m *MockKVStorage) GetByIDs(ctx context.Context, ids []string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	for _, id := range ids {
		if data, ok := m.data[id]; ok {
			results = append(results, data)
		}
	}
	return results, nil
}

func (m *MockKVStorage) FilterKeys(ctx context.Context, keys map[string]struct{}) (map[string]struct{}, error) {
	notFound := make(map[string]struct{})
	for key := range keys {
		if _, ok := m.data[key]; !ok {
			notFound[key] = struct{}{}
		}
	}
	return notFound, nil
}

func (m *MockKVStorage) Upsert(ctx context.Context, data map[string]map[string]interface{}) error {
	for id, d := range data {
		m.data[id] = d
	}
	return nil
}

func (m *MockKVStorage) Delete(ctx context.Context, ids []string) error {
	for _, id := range ids {
		delete(m.data, id)
	}
	return nil
}

func (m *MockKVStorage) IsEmpty(ctx context.Context) (bool, error) {
	return len(m.data) == 0, nil
}

func (m *MockKVStorage) GetAll(ctx context.Context) (map[string]map[string]interface{}, error) {
	result := make(map[string]map[string]interface{}, len(m.data))
	for k, v := range m.data {
		result[k] = v
	}
	return result, nil
}

// MockVectorStorage implements base.BaseVectorStorage for testing
type MockVectorStorage struct {
	namespace string
	workspace string
	data      map[string]map[string]interface{}
}

func NewMockVectorStorage() *MockVectorStorage {
	return &MockVectorStorage{
		data: make(map[string]map[string]interface{}),
	}
}

func (m *MockVectorStorage) GetNamespace() string                        { return m.namespace }
func (m *MockVectorStorage) GetWorkspace() string                        { return m.workspace }
func (m *MockVectorStorage) Initialize(ctx context.Context) error        { return nil }
func (m *MockVectorStorage) Finalize(ctx context.Context) error          { return nil }
func (m *MockVectorStorage) IndexDoneCallback(ctx context.Context) error { return nil }
func (m *MockVectorStorage) Drop(ctx context.Context) (map[string]string, error) {
	m.data = make(map[string]map[string]interface{})
	return nil, nil
}

func (m *MockVectorStorage) Query(ctx context.Context, query string, topK int, queryEmbedding []float64) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	count := 0
	for _, data := range m.data {
		if count >= topK {
			break
		}
		results = append(results, data)
		count++
	}
	return results, nil
}

func (m *MockVectorStorage) GetByID(ctx context.Context, id string) (map[string]interface{}, error) {
	if data, ok := m.data[id]; ok {
		return data, nil
	}
	return nil, nil
}

func (m *MockVectorStorage) GetByIDs(ctx context.Context, ids []string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	for _, id := range ids {
		if data, ok := m.data[id]; ok {
			results = append(results, data)
		}
	}
	return results, nil
}

func (m *MockVectorStorage) Upsert(ctx context.Context, data map[string]map[string]interface{}) error {
	for id, d := range data {
		m.data[id] = d
	}
	return nil
}

func (m *MockVectorStorage) Delete(ctx context.Context, ids []string) error {
	for _, id := range ids {
		delete(m.data, id)
	}
	return nil
}

func (m *MockVectorStorage) DeleteEntity(ctx context.Context, entityName string) error {
	return nil
}

func (m *MockVectorStorage) DeleteEntityRelation(ctx context.Context, entityName string) error {
	return nil
}

func (m *MockVectorStorage) GetVectorsByIDs(ctx context.Context, ids []string) (map[string][]float64, error) {
	return nil, nil
}

func (m *MockVectorStorage) GetCosineBetterThanThreshold() float64 {
	return 0.2
}

func (m *MockVectorStorage) GetMetaFields() map[string]struct{} {
	return nil
}

// MockGraphStorage implements base.BaseGraphStorage for testing
type MockGraphStorage struct {
	namespace string
	workspace string
	nodes     map[string]map[string]string
	edges     map[string]map[string]string
}

func NewMockGraphStorage() *MockGraphStorage {
	return &MockGraphStorage{
		nodes: make(map[string]map[string]string),
		edges: make(map[string]map[string]string),
	}
}

func (m *MockGraphStorage) GetNamespace() string                        { return m.namespace }
func (m *MockGraphStorage) GetWorkspace() string                        { return m.workspace }
func (m *MockGraphStorage) Initialize(ctx context.Context) error        { return nil }
func (m *MockGraphStorage) Finalize(ctx context.Context) error          { return nil }
func (m *MockGraphStorage) IndexDoneCallback(ctx context.Context) error { return nil }
func (m *MockGraphStorage) Drop(ctx context.Context) (map[string]string, error) {
	m.nodes = make(map[string]map[string]string)
	m.edges = make(map[string]map[string]string)
	return nil, nil
}

func (m *MockGraphStorage) HasNode(ctx context.Context, nodeID string) (bool, error) {
	_, ok := m.nodes[nodeID]
	return ok, nil
}

func (m *MockGraphStorage) HasEdge(ctx context.Context, srcID, tgtID string) (bool, error) {
	key := srcID + "||" + tgtID
	_, ok := m.edges[key]
	return ok, nil
}

func (m *MockGraphStorage) NodeDegree(ctx context.Context, nodeID string) (int, error) {
	count := 0
	for key := range m.edges {
		if len(key) >= len(nodeID) && (key[:len(nodeID)] == nodeID || key[len(key)-len(nodeID):] == nodeID) {
			count++
		}
	}
	return count, nil
}

func (m *MockGraphStorage) EdgeDegree(ctx context.Context, srcID, tgtID string) (int, error) {
	return 1, nil
}

func (m *MockGraphStorage) GetNode(ctx context.Context, nodeID string) (map[string]string, error) {
	if data, ok := m.nodes[nodeID]; ok {
		return data, nil
	}
	return nil, nil
}

func (m *MockGraphStorage) GetEdge(ctx context.Context, srcID, tgtID string) (map[string]string, error) {
	key := srcID + "||" + tgtID
	if data, ok := m.edges[key]; ok {
		return data, nil
	}
	return nil, nil
}

func (m *MockGraphStorage) GetNodeEdges(ctx context.Context, sourceNodeID string) ([][2]string, error) {
	var edges [][2]string
	for range m.edges {
		// Simple check - in real impl would parse the key
		edges = append(edges, [2]string{sourceNodeID, "target"})
	}
	return edges, nil
}

func (m *MockGraphStorage) GetNodesBatch(ctx context.Context, nodeIDs []string) (map[string]map[string]string, error) {
	result := make(map[string]map[string]string)
	for _, id := range nodeIDs {
		if data, ok := m.nodes[id]; ok {
			result[id] = data
		}
	}
	return result, nil
}

func (m *MockGraphStorage) NodeDegreesBatch(ctx context.Context, nodeIDs []string) (map[string]int, error) {
	result := make(map[string]int)
	for _, id := range nodeIDs {
		result[id] = 1
	}
	return result, nil
}

func (m *MockGraphStorage) EdgeDegreesBatch(ctx context.Context, edgePairs [][2]string) (map[[2]string]int, error) {
	result := make(map[[2]string]int)
	for _, pair := range edgePairs {
		result[pair] = 1
	}
	return result, nil
}

func (m *MockGraphStorage) GetEdgesBatch(ctx context.Context, pairs []map[string]string) (map[[2]string]map[string]string, error) {
	return nil, nil
}

func (m *MockGraphStorage) GetNodesEdgesBatch(ctx context.Context, nodeIDs []string) (map[string][][2]string, error) {
	return nil, nil
}

func (m *MockGraphStorage) UpsertNode(ctx context.Context, nodeID string, nodeData map[string]string) error {
	m.nodes[nodeID] = nodeData
	return nil
}

func (m *MockGraphStorage) UpsertEdge(ctx context.Context, srcID, tgtID string, edgeData map[string]string) error {
	key := srcID + "||" + tgtID
	m.edges[key] = edgeData
	return nil
}

func (m *MockGraphStorage) DeleteNode(ctx context.Context, nodeID string) error {
	delete(m.nodes, nodeID)
	return nil
}

func (m *MockGraphStorage) RemoveNodes(ctx context.Context, nodes []string) error {
	for _, n := range nodes {
		delete(m.nodes, n)
	}
	return nil
}

func (m *MockGraphStorage) RemoveEdges(ctx context.Context, edges [][2]string) error {
	for _, e := range edges {
		delete(m.edges, e[0]+"||"+e[1])
	}
	return nil
}

func (m *MockGraphStorage) GetAllLabels(ctx context.Context) ([]string, error) {
	var labels []string
	for k := range m.nodes {
		labels = append(labels, k)
	}
	return labels, nil
}

func (m *MockGraphStorage) GetKnowledgeGraph(ctx context.Context, nodeLabel string, maxDepth, maxNodes int) (*base.KnowledgeGraph, error) {
	return &base.KnowledgeGraph{}, nil
}

func (m *MockGraphStorage) GetAllNodes(ctx context.Context) ([]map[string]interface{}, error) {
	return nil, nil
}

func (m *MockGraphStorage) GetAllEdges(ctx context.Context) ([]map[string]interface{}, error) {
	return nil, nil
}

func (m *MockGraphStorage) GetPopularLabels(ctx context.Context, limit int) ([]string, error) {
	return nil, nil
}

func (m *MockGraphStorage) SearchLabels(ctx context.Context, query string, limit int) ([]string, error) {
	return nil, nil
}

// Mock LLM function for testing
func mockLLMFunc(ctx context.Context, prompt string, opts ...operate.LLMOption) (string, error) {
	return "Mock LLM response", nil
}

// Mock embedding function for testing
func mockEmbedFunc(ctx context.Context, texts []string) ([][]float64, error) {
	results := make([][]float64, len(texts))
	for i := range texts {
		results[i] = []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	}
	return results, nil
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "./rag_storage", config.WorkingDir)
	assert.Equal(t, "JsonKVStorage", config.KVStorageType)
	assert.Equal(t, "NanoVectorDBStorage", config.VectorStorageType)
	assert.Equal(t, "NetworkXStorage", config.GraphStorageType)
	assert.Equal(t, coreconstants.DefaultTopK, config.TopK)
	assert.Equal(t, coreconstants.DefaultChunkTopK, config.ChunkTopK)
	assert.Equal(t, coreconstants.DefaultChunkSize, config.ChunkTokenSize)
	assert.True(t, config.EnableLLMCache)
}

func TestNewLightRAG_RequiresLLMFunc(t *testing.T) {
	config := DefaultConfig()
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func: mockEmbedFunc,
	}
	// No LLM function set

	_, err := New(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LLMModelFunc is required")
}

func TestNewLightRAG_RequiresEmbeddingFunc(t *testing.T) {
	config := DefaultConfig()
	config.LLMModelFunc = mockLLMFunc
	// No embedding function set

	_, err := New(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EmbeddingFunc is required")
}

func TestNewLightRAG_Success(t *testing.T) {
	config := DefaultConfig()
	config.WorkingDir = t.TempDir()
	config.LLMModelFunc = mockLLMFunc
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func:         mockEmbedFunc,
		EmbeddingDim: 5,
	}

	rag, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, rag)

	assert.Equal(t, StoragesCreated, rag.GetStatus())
	assert.Equal(t, config.WorkingDir, rag.GetWorkingDir())
}

func TestStoragesStatus_String(t *testing.T) {
	tests := []struct {
		status   StoragesStatus
		expected string
	}{
		{StoragesNotCreated, "NOT_CREATED"},
		{StoragesCreated, "CREATED"},
		{StoragesInitialized, "INITIALIZED"},
		{StoragesFinalized, "FINALIZED"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestInitializeAndFinalizeStorages(t *testing.T) {
	config := DefaultConfig()
	config.WorkingDir = t.TempDir()
	config.LLMModelFunc = mockLLMFunc
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func:         mockEmbedFunc,
		EmbeddingDim: 5,
	}

	rag, err := New(config)
	require.NoError(t, err)

	// Set mock storages (12 storages total)
	rag.SetStorages(
		NewMockKVStorage(),     // fullDocs
		NewMockKVStorage(),     // textChunks
		NewMockKVStorage(),     // fullEntities
		NewMockKVStorage(),     // fullRelations
		NewMockKVStorage(),     // entityChunks
		NewMockKVStorage(),     // relationChunks
		NewMockVectorStorage(), // entitiesVDB
		NewMockVectorStorage(), // relationshipsVDB
		NewMockVectorStorage(), // chunksVDB
		NewMockGraphStorage(),  // chunkEntityRelGraph
		NewMockKVStorage(),     // llmResponseCache
		NewMockKVStorage(),     // docStatus
	)

	// Initialize
	ctx := context.Background()
	err = rag.InitializeStorages(ctx)
	require.NoError(t, err)
	assert.Equal(t, StoragesInitialized, rag.GetStatus())

	// Finalize
	err = rag.FinalizeStorages(ctx)
	require.NoError(t, err)
	assert.Equal(t, StoragesFinalized, rag.GetStatus())
}

func TestInsert_WithoutInitialization(t *testing.T) {
	config := DefaultConfig()
	config.WorkingDir = t.TempDir()
	config.LLMModelFunc = mockLLMFunc
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func:         mockEmbedFunc,
		EmbeddingDim: 5,
	}

	rag, err := New(config)
	require.NoError(t, err)

	// Try to insert without initializing storages
	ctx := context.Background()
	err = rag.Insert(ctx, "test content")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storages not initialized")
}

func TestInsert_WithInitializedStorages(t *testing.T) {
	config := DefaultConfig()
	config.WorkingDir = t.TempDir()
	config.LLMModelFunc = mockLLMFunc
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func:         mockEmbedFunc,
		EmbeddingDim: 5,
	}

	rag, err := New(config)
	require.NoError(t, err)

	// Set mock storages (12 storages total)
	rag.SetStorages(
		NewMockKVStorage(),     // fullDocs
		NewMockKVStorage(),     // textChunks
		NewMockKVStorage(),     // fullEntities
		NewMockKVStorage(),     // fullRelations
		NewMockKVStorage(),     // entityChunks
		NewMockKVStorage(),     // relationChunks
		NewMockVectorStorage(), // entitiesVDB
		NewMockVectorStorage(), // relationshipsVDB
		NewMockVectorStorage(), // chunksVDB
		NewMockGraphStorage(),  // chunkEntityRelGraph
		NewMockKVStorage(),     // llmResponseCache
		NewMockKVStorage(),     // docStatus
	)

	// Initialize
	ctx := context.Background()
	err = rag.InitializeStorages(ctx)
	require.NoError(t, err)

	// Insert
	err = rag.Insert(ctx, "This is a test document with some content for testing the insert functionality.")
	require.NoError(t, err)
}

func TestQuery_WithoutInitialization(t *testing.T) {
	config := DefaultConfig()
	config.WorkingDir = t.TempDir()
	config.LLMModelFunc = mockLLMFunc
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func:         mockEmbedFunc,
		EmbeddingDim: 5,
	}

	rag, err := New(config)
	require.NoError(t, err)

	// Try to query without initializing storages
	ctx := context.Background()
	_, err = rag.Query(ctx, "test query", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storages not initialized")
}

func TestDelete_WithoutInitialization(t *testing.T) {
	config := DefaultConfig()
	config.WorkingDir = t.TempDir()
	config.LLMModelFunc = mockLLMFunc
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func:         mockEmbedFunc,
		EmbeddingDim: 5,
	}

	rag, err := New(config)
	require.NoError(t, err)

	// Try to delete without initializing storages
	ctx := context.Background()
	err = rag.Delete(ctx, "test-doc-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storages not initialized")
}

func TestInsertOptions(t *testing.T) {
	options := DefaultInsertOptions()

	assert.True(t, options.CheckDuplicate)
	assert.True(t, options.ExtractEntities)
	assert.Empty(t, options.SplitByCharacter)
	assert.False(t, options.SplitByCharacterOnly)
	assert.Empty(t, options.FilePath)

	// Test with options
	options = DefaultInsertOptions()
	WithExtractEntities(false)(options)
	WithCheckDuplicate(false)(options)
	WithSplitByCharacter("\n")(options)
	WithFilePath("/path/to/file.txt")(options)

	assert.False(t, options.ExtractEntities)
	assert.False(t, options.CheckDuplicate)
	assert.Equal(t, "\n", options.SplitByCharacter)
	assert.Equal(t, "/path/to/file.txt", options.FilePath)
}

func TestGetFullStoragePath(t *testing.T) {
	config := DefaultConfig()
	config.WorkingDir = "/test/rag_storage"

	rag := &LightRAG{config: config}

	path := rag.GetFullStoragePath("namespace1")
	assert.Contains(t, path, "/test/rag_storage")
	assert.Contains(t, path, "namespace1")
}

func TestGetConfig(t *testing.T) {
	config := DefaultConfig()
	config.WorkingDir = t.TempDir()
	config.LLMModelFunc = mockLLMFunc
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func:         mockEmbedFunc,
		EmbeddingDim: 5,
	}

	rag, err := New(config)
	require.NoError(t, err)

	retrievedConfig := rag.GetConfig()
	assert.Equal(t, config.WorkingDir, retrievedConfig.WorkingDir)
	assert.Equal(t, config.TopK, retrievedConfig.TopK)
}

func TestGetWorkingDir(t *testing.T) {
	config := DefaultConfig()
	config.WorkingDir = t.TempDir()
	config.LLMModelFunc = mockLLMFunc
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func:         mockEmbedFunc,
		EmbeddingDim: 5,
	}

	rag, err := New(config)
	require.NoError(t, err)

	assert.Equal(t, config.WorkingDir, rag.GetWorkingDir())
}
