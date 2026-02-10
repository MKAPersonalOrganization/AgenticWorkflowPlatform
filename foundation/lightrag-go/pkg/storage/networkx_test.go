package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestNetworkXStorage(t *testing.T) (*NetworkXStorage, string) {
	tempDir, err := os.MkdirTemp("", "networkx_test_*")
	require.NoError(t, err)

	config := &StorageConfig{
		Namespace: "test",
		Workspace: tempDir,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	storage := NewNetworkXStorage(config, logger)
	return storage, tempDir
}

func TestNetworkXStorage_Initialize(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	assert.NoError(t, err)

	// Idempotent
	err = storage.Initialize(ctx)
	assert.NoError(t, err)
}

func TestNetworkXStorage_InitializeWithExistingData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "networkx_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	existingData := `{
		"nodes": [
			{"id": "n1", "properties": {"name": "Node1"}},
			{"id": "n2", "properties": {"name": "Node2"}}
		],
		"edges": [
			{"source": "n1", "target": "n2", "properties": {"weight": "1.0"}}
		]
	}`
	filePath := filepath.Join(tempDir, "graph_test.json")
	err = os.WriteFile(filePath, []byte(existingData), 0o644)
	require.NoError(t, err)

	config := &StorageConfig{
		Namespace: "test",
		Workspace: tempDir,
	}
	storage := NewNetworkXStorage(config, nil)

	ctx := context.Background()
	err = storage.Initialize(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, storage.NodeCount())
	assert.Equal(t, 1, storage.EdgeCount())
}

func TestNetworkXStorage_UpsertNode(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Insert node
	err = storage.UpsertNode(ctx, "n1", map[string]string{"name": "Alice", "type": "Person"})
	assert.NoError(t, err)
	assert.Equal(t, 1, storage.NodeCount())

	// Update node
	err = storage.UpsertNode(ctx, "n1", map[string]string{"age": "30"})
	assert.NoError(t, err)
	assert.Equal(t, 1, storage.NodeCount())

	node, err := storage.GetNode(ctx, "n1")
	assert.NoError(t, err)
	assert.Equal(t, "Alice", node["name"])
	assert.Equal(t, "30", node["age"])
}

func TestNetworkXStorage_UpsertEdge(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Insert edge (should create nodes automatically)
	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{"type": "KNOWS"})
	assert.NoError(t, err)
	assert.Equal(t, 2, storage.NodeCount())
	assert.Equal(t, 1, storage.EdgeCount())

	// Update edge
	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{"weight": "0.8"})
	assert.NoError(t, err)
	assert.Equal(t, 1, storage.EdgeCount())

	edge, err := storage.GetEdge(ctx, "n1", "n2")
	assert.NoError(t, err)
	assert.Equal(t, "KNOWS", edge["type"])
	assert.Equal(t, "0.8", edge["weight"])
}

func TestNetworkXStorage_HasNode(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertNode(ctx, "n1", map[string]string{})
	require.NoError(t, err)

	exists, err := storage.HasNode(ctx, "n1")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = storage.HasNode(ctx, "n_not_exist")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestNetworkXStorage_HasEdge(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{})
	require.NoError(t, err)

	// Forward direction
	exists, err := storage.HasEdge(ctx, "n1", "n2")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Reverse direction (undirected)
	exists, err = storage.HasEdge(ctx, "n2", "n1")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Non-existing
	exists, err = storage.HasEdge(ctx, "n1", "n3")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestNetworkXStorage_NodeDegree(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Create a small graph
	err = storage.UpsertNode(ctx, "n1", map[string]string{})
	require.NoError(t, err)
	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{})
	require.NoError(t, err)
	err = storage.UpsertEdge(ctx, "n1", "n3", map[string]string{})
	require.NoError(t, err)
	err = storage.UpsertEdge(ctx, "n4", "n1", map[string]string{})
	require.NoError(t, err)

	degree, err := storage.NodeDegree(ctx, "n1")
	assert.NoError(t, err)
	assert.Equal(t, 3, degree) // 2 outgoing + 1 incoming

	// Non-existing node
	degree, err = storage.NodeDegree(ctx, "n_not_exist")
	assert.NoError(t, err)
	assert.Equal(t, 0, degree)
}

func TestNetworkXStorage_EdgeDegree(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Create edges
	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{})
	require.NoError(t, err)
	err = storage.UpsertEdge(ctx, "n1", "n3", map[string]string{})
	require.NoError(t, err)

	// n1 has degree 2, n2 has degree 1
	degree, err := storage.EdgeDegree(ctx, "n1", "n2")
	assert.NoError(t, err)
	assert.Equal(t, 3, degree) // 2 + 1
}

func TestNetworkXStorage_GetNode(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertNode(ctx, "n1", map[string]string{"name": "Test"})
	require.NoError(t, err)

	node, err := storage.GetNode(ctx, "n1")
	assert.NoError(t, err)
	assert.Equal(t, "Test", node["name"])

	// Non-existing
	node, err = storage.GetNode(ctx, "n_not_exist")
	assert.NoError(t, err)
	assert.Nil(t, node)
}

func TestNetworkXStorage_GetEdge(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{"type": "KNOWS"})
	require.NoError(t, err)

	edge, err := storage.GetEdge(ctx, "n1", "n2")
	assert.NoError(t, err)
	assert.Equal(t, "KNOWS", edge["type"])

	// Reverse (undirected)
	edge, err = storage.GetEdge(ctx, "n2", "n1")
	assert.NoError(t, err)
	assert.Equal(t, "KNOWS", edge["type"])

	// Non-existing
	edge, err = storage.GetEdge(ctx, "n1", "n3")
	assert.NoError(t, err)
	assert.Nil(t, edge)
}

func TestNetworkXStorage_GetNodeEdges(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{})
	require.NoError(t, err)
	err = storage.UpsertEdge(ctx, "n1", "n3", map[string]string{})
	require.NoError(t, err)
	err = storage.UpsertEdge(ctx, "n4", "n1", map[string]string{})
	require.NoError(t, err)

	edges, err := storage.GetNodeEdges(ctx, "n1")
	assert.NoError(t, err)
	assert.Len(t, edges, 3)

	// Non-existing node
	edges, err = storage.GetNodeEdges(ctx, "n_not_exist")
	assert.NoError(t, err)
	assert.Nil(t, edges)
}

func TestNetworkXStorage_GetNodesBatch(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertNode(ctx, "n1", map[string]string{"name": "A"})
	require.NoError(t, err)
	err = storage.UpsertNode(ctx, "n2", map[string]string{"name": "B"})
	require.NoError(t, err)

	nodes, err := storage.GetNodesBatch(ctx, []string{"n1", "n2", "n_not_exist"})
	assert.NoError(t, err)
	assert.Len(t, nodes, 2)
	assert.Equal(t, "A", nodes["n1"]["name"])
}

func TestNetworkXStorage_NodeDegreesBatch(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{})
	require.NoError(t, err)
	err = storage.UpsertEdge(ctx, "n1", "n3", map[string]string{})
	require.NoError(t, err)

	degrees, err := storage.NodeDegreesBatch(ctx, []string{"n1", "n2"})
	assert.NoError(t, err)
	assert.Equal(t, 2, degrees["n1"])
	assert.Equal(t, 1, degrees["n2"])
}

func TestNetworkXStorage_DeleteNode(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{})
	require.NoError(t, err)
	err = storage.UpsertEdge(ctx, "n2", "n3", map[string]string{})
	require.NoError(t, err)

	assert.Equal(t, 3, storage.NodeCount())
	assert.Equal(t, 2, storage.EdgeCount())

	err = storage.DeleteNode(ctx, "n2")
	assert.NoError(t, err)

	assert.Equal(t, 2, storage.NodeCount())
	assert.Equal(t, 0, storage.EdgeCount()) // All edges involving n2 removed
}

func TestNetworkXStorage_DeleteEdge(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{})
	require.NoError(t, err)
	err = storage.UpsertEdge(ctx, "n2", "n3", map[string]string{})
	require.NoError(t, err)

	assert.Equal(t, 2, storage.EdgeCount())

	err = storage.DeleteEdge(ctx, "n1", "n2")
	assert.NoError(t, err)

	assert.Equal(t, 1, storage.EdgeCount())
	assert.Equal(t, 3, storage.NodeCount()) // Nodes remain
}

func TestNetworkXStorage_IndexDoneCallback(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertNode(ctx, "n1", map[string]string{"name": "Test"})
	require.NoError(t, err)
	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{"type": "KNOWS"})
	require.NoError(t, err)

	err = storage.IndexDoneCallback(ctx)
	assert.NoError(t, err)

	// Verify file exists
	filePath := filepath.Join(tempDir, "graph_test.json")
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	// Load in new storage
	storage2 := NewNetworkXStorage(&StorageConfig{
		Namespace: "test",
		Workspace: tempDir,
	}, nil)

	err = storage2.Initialize(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, storage2.NodeCount())
	assert.Equal(t, 1, storage2.EdgeCount())
}

func TestNetworkXStorage_Drop(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertNode(ctx, "n1", map[string]string{})
	require.NoError(t, err)
	err = storage.IndexDoneCallback(ctx)
	require.NoError(t, err)

	result, err := storage.Drop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "success", result["status"])
	assert.Equal(t, 0, storage.NodeCount())

	// Verify file removed
	filePath := filepath.Join(tempDir, "graph_test.json")
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))
}

func TestNetworkXStorage_AllNodes(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertNode(ctx, "n1", map[string]string{})
	require.NoError(t, err)
	err = storage.UpsertNode(ctx, "n2", map[string]string{})
	require.NoError(t, err)

	nodes := storage.AllNodes()
	assert.Len(t, nodes, 2)
	assert.Contains(t, nodes, "n1")
	assert.Contains(t, nodes, "n2")
}

func TestNetworkXStorage_AllEdges(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{})
	require.NoError(t, err)
	err = storage.UpsertEdge(ctx, "n2", "n3", map[string]string{})
	require.NoError(t, err)

	edges := storage.AllEdges()
	assert.Len(t, edges, 2)
}

func TestNetworkXStorage_GetKnowledgeGraph(t *testing.T) {
	storage, tempDir := newTestNetworkXStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	err = storage.UpsertNode(ctx, "n1", map[string]string{"type": "entity"})
	require.NoError(t, err)
	err = storage.UpsertNode(ctx, "n2", map[string]string{"type": "entity"})
	require.NoError(t, err)
	err = storage.UpsertEdge(ctx, "n1", "n2", map[string]string{"relation": "related"})
	require.NoError(t, err)

	result, err := storage.GetKnowledgeGraph(ctx, "n1", 2, 10)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Nodes, 2)
	assert.Len(t, result.Edges, 1)
}
