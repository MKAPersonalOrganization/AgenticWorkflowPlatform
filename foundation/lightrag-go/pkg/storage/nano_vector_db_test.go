package storage

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestNanoVectorDBStorage(t *testing.T) (*NanoVectorDBStorage, string) {
	tempDir, err := os.MkdirTemp("", "nanovdb_test_*")
	require.NoError(t, err)

	config := &StorageConfig{
		Namespace:       "test",
		Workspace:       tempDir,
		CosineThreshold: 0.2,
		EmbeddingDim:    3,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	storage := NewNanoVectorDBStorage(config, logger)
	return storage, tempDir
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float64
		b        []float64
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{-1, 0, 0},
			expected: -1.0,
		},
		{
			name:     "similar vectors",
			a:        []float64{1, 1, 0},
			b:        []float64{1, 0, 0},
			expected: 1 / math.Sqrt(2),
		},
		{
			name:     "different length",
			a:        []float64{1, 0},
			b:        []float64{1, 0, 0},
			expected: 0.0,
		},
		{
			name:     "zero vector",
			a:        []float64{0, 0, 0},
			b:        []float64{1, 0, 0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestNanoVectorDBStorage_Initialize(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	assert.NoError(t, err)

	// Initialize again should be idempotent
	err = storage.Initialize(ctx)
	assert.NoError(t, err)
}

func TestNanoVectorDBStorage_InitializeWithExistingData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nanovdb_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create existing data file
	existingData := `[
		{"id": "v1", "vector": [1.0, 0.0, 0.0], "metadata": {"name": "vec1"}},
		{"id": "v2", "vector": [0.0, 1.0, 0.0], "metadata": {"name": "vec2"}}
	]`
	filePath := filepath.Join(tempDir, "vdb_test.json")
	err = os.WriteFile(filePath, []byte(existingData), 0o644)
	require.NoError(t, err)

	config := &StorageConfig{
		Namespace: "test",
		Workspace: tempDir,
	}

	storage := NewNanoVectorDBStorage(config, nil)

	ctx := context.Background()
	err = storage.Initialize(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, storage.Count())
}

func TestNanoVectorDBStorage_Upsert(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"v1": {
			"vector": []float64{1.0, 0.0, 0.0},
			"name":   "vector1",
		},
		"v2": {
			"vector": []float64{0.0, 1.0, 0.0},
			"name":   "vector2",
		},
	}

	err = storage.Upsert(ctx, data)
	assert.NoError(t, err)
	assert.Equal(t, 2, storage.Count())
}

func TestNanoVectorDBStorage_Query(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"v1": {
			"vector": []float64{1.0, 0.0, 0.0},
			"name":   "vector1",
		},
		"v2": {
			"vector": []float64{0.9, 0.1, 0.0},
			"name":   "vector2",
		},
		"v3": {
			"vector": []float64{0.0, 1.0, 0.0},
			"name":   "vector3",
		},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	// Query with provided embedding
	queryEmbedding := []float64{1.0, 0.0, 0.0}
	results, err := storage.Query(ctx, "", 2, queryEmbedding)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// First result should be v1 (exact match)
	assert.Equal(t, "v1", results[0]["id"])
	assert.InDelta(t, 1.0, results[0]["score"], 0.0001)

	// Second result should be v2 (similar)
	assert.Equal(t, "v2", results[1]["id"])
}

func TestNanoVectorDBStorage_QueryEmpty(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	queryEmbedding := []float64{1.0, 0.0, 0.0}
	results, err := storage.Query(ctx, "", 10, queryEmbedding)
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestNanoVectorDBStorage_GetByID(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"v1": {
			"vector": []float64{1.0, 0.0, 0.0},
			"name":   "vector1",
		},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	// Get existing
	result, err := storage.GetByID(ctx, "v1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "v1", result["id"])
	assert.Equal(t, "vector1", result["name"])

	// Get non-existing
	result, err = storage.GetByID(ctx, "v_not_exist")
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestNanoVectorDBStorage_GetByIDs(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"v1": {"vector": []float64{1.0, 0.0, 0.0}},
		"v2": {"vector": []float64{0.0, 1.0, 0.0}},
		"v3": {"vector": []float64{0.0, 0.0, 1.0}},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	results, err := storage.GetByIDs(ctx, []string{"v1", "v3", "v_not_exist"})
	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestNanoVectorDBStorage_Delete(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"v1": {"vector": []float64{1.0, 0.0, 0.0}},
		"v2": {"vector": []float64{0.0, 1.0, 0.0}},
		"v3": {"vector": []float64{0.0, 0.0, 1.0}},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	err = storage.Delete(ctx, []string{"v1", "v3"})
	assert.NoError(t, err)
	assert.Equal(t, 1, storage.Count())

	result, err := storage.GetByID(ctx, "v2")
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNanoVectorDBStorage_DeleteEntity(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data with entity names
	data := map[string]map[string]interface{}{
		"v1": {"vector": []float64{1.0, 0.0, 0.0}, "entity_name": "Alice"},
		"v2": {"vector": []float64{0.0, 1.0, 0.0}, "entity_name": "Bob"},
		"v3": {"vector": []float64{0.0, 0.0, 1.0}, "entity_name": "ALICE"}, // Case-insensitive
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	err = storage.DeleteEntity(ctx, "alice")
	assert.NoError(t, err)
	assert.Equal(t, 1, storage.Count())
}

func TestNanoVectorDBStorage_DeleteEntityRelation(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data with relations
	data := map[string]map[string]interface{}{
		"r1": {"vector": []float64{1.0, 0.0, 0.0}, "src_id": "Alice", "tgt_id": "Bob"},
		"r2": {"vector": []float64{0.0, 1.0, 0.0}, "src_id": "Bob", "tgt_id": "Charlie"},
		"r3": {"vector": []float64{0.0, 0.0, 1.0}, "src_id": "Charlie", "tgt_id": "Alice"},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	err = storage.DeleteEntityRelation(ctx, "alice")
	assert.NoError(t, err)
	assert.Equal(t, 1, storage.Count()) // Only r2 remains
}

func TestNanoVectorDBStorage_GetVectorsByIDs(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"v1": {"vector": []float64{1.0, 0.0, 0.0}},
		"v2": {"vector": []float64{0.0, 1.0, 0.0}},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	vectors, err := storage.GetVectorsByIDs(ctx, []string{"v1", "v2", "v_not_exist"})
	assert.NoError(t, err)
	assert.Len(t, vectors, 2)
	assert.Equal(t, []float64{1.0, 0.0, 0.0}, vectors["v1"])
	assert.Equal(t, []float64{0.0, 1.0, 0.0}, vectors["v2"])
}

func TestNanoVectorDBStorage_IndexDoneCallback(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"v1": {"vector": []float64{1.0, 0.0, 0.0}, "name": "vec1"},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	// Persist
	err = storage.IndexDoneCallback(ctx)
	assert.NoError(t, err)

	// Verify file exists
	filePath := filepath.Join(tempDir, "vdb_test.json")
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	// Load in new storage
	storage2 := NewNanoVectorDBStorage(&StorageConfig{
		Namespace: "test",
		Workspace: tempDir,
	}, nil)

	err = storage2.Initialize(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, storage2.Count())
}

func TestNanoVectorDBStorage_Drop(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert and persist
	data := map[string]map[string]interface{}{
		"v1": {"vector": []float64{1.0, 0.0, 0.0}},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)
	err = storage.IndexDoneCallback(ctx)
	require.NoError(t, err)

	// Drop
	result, err := storage.Drop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "success", result["status"])
	assert.Equal(t, 0, storage.Count())

	// Verify file removed
	filePath := filepath.Join(tempDir, "vdb_test.json")
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))
}

func TestNanoVectorDBStorage_QueryWithThreshold(t *testing.T) {
	storage, tempDir := newTestNanoVectorDBStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Set high threshold
	storage.SetCosineThreshold(0.95)

	// Upsert data
	data := map[string]map[string]interface{}{
		"v1": {"vector": []float64{1.0, 0.0, 0.0}},
		"v2": {"vector": []float64{0.9, 0.1, 0.0}}, // similarity ~0.994
		"v3": {"vector": []float64{0.5, 0.5, 0.0}}, // similarity ~0.707
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	queryEmbedding := []float64{1.0, 0.0, 0.0}
	results, err := storage.Query(ctx, "", 10, queryEmbedding)
	assert.NoError(t, err)

	// Only v1 and v2 should pass the threshold
	assert.Len(t, results, 2)
}
