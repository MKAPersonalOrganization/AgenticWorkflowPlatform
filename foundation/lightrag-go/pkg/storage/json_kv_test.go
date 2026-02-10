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

func newTestJsonKVStorage(t *testing.T) (*JsonKVStorage, string) {
	tempDir, err := os.MkdirTemp("", "jsonkv_test_*")
	require.NoError(t, err)

	config := &StorageConfig{
		Namespace: "test",
		Workspace: tempDir,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	storage := NewJsonKVStorage(config, logger)
	return storage, tempDir
}

func TestJsonKVStorage_Initialize(t *testing.T) {
	storage, tempDir := newTestJsonKVStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	assert.NoError(t, err)

	// Initialize again should be idempotent
	err = storage.Initialize(ctx)
	assert.NoError(t, err)
}

func TestJsonKVStorage_InitializeWithExistingData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "jsonkv_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create existing data file
	existingData := `{"key1": {"field1": "value1"}, "key2": {"field2": "value2"}}`
	filePath := filepath.Join(tempDir, "kv_store_test.json")
	err = os.WriteFile(filePath, []byte(existingData), 0o644)
	require.NoError(t, err)

	config := &StorageConfig{
		Namespace: "test",
		Workspace: tempDir,
	}

	storage := NewJsonKVStorage(config, nil)

	ctx := context.Background()
	err = storage.Initialize(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, storage.Count())
}

func TestJsonKVStorage_Upsert(t *testing.T) {
	storage, tempDir := newTestJsonKVStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"key1": {"name": "Alice", "age": 30},
		"key2": {"name": "Bob", "age": 25},
	}

	err = storage.Upsert(ctx, data)
	assert.NoError(t, err)
	assert.Equal(t, 2, storage.Count())
}

func TestJsonKVStorage_GetByID(t *testing.T) {
	storage, tempDir := newTestJsonKVStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"key1": {"name": "Alice", "age": float64(30)},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	// Get existing key
	result, err := storage.GetByID(ctx, "key1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Alice", result["name"])
	assert.Equal(t, float64(30), result["age"])

	// Get non-existing key
	result, err = storage.GetByID(ctx, "key_not_exist")
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestJsonKVStorage_GetByIDs(t *testing.T) {
	storage, tempDir := newTestJsonKVStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"key1": {"name": "Alice"},
		"key2": {"name": "Bob"},
		"key3": {"name": "Charlie"},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	// Get existing keys
	results, err := storage.GetByIDs(ctx, []string{"key1", "key3", "key_not_exist"})
	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestJsonKVStorage_FilterKeys(t *testing.T) {
	storage, tempDir := newTestJsonKVStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"key1": {"name": "Alice"},
		"key2": {"name": "Bob"},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	// Filter keys
	nonExistent, err := storage.FilterKeys(ctx, map[string]struct{}{"key1": {}, "key2": {}, "key3": {}, "key4": {}})
	assert.NoError(t, err)
	assert.Len(t, nonExistent, 2)
	_, hasKey3 := nonExistent["key3"]
	_, hasKey4 := nonExistent["key4"]
	assert.True(t, hasKey3)
	assert.True(t, hasKey4)
}

func TestJsonKVStorage_Delete(t *testing.T) {
	storage, tempDir := newTestJsonKVStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"key1": {"name": "Alice"},
		"key2": {"name": "Bob"},
		"key3": {"name": "Charlie"},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)
	assert.Equal(t, 3, storage.Count())

	// Delete keys
	err = storage.Delete(ctx, []string{"key1", "key3"})
	assert.NoError(t, err)
	assert.Equal(t, 1, storage.Count())

	// Verify remaining key
	result, err := storage.GetByID(ctx, "key2")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Bob", result["name"])
}

func TestJsonKVStorage_IsEmpty(t *testing.T) {
	storage, tempDir := newTestJsonKVStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Should be empty initially
	isEmpty, err := storage.IsEmpty(ctx)
	assert.NoError(t, err)
	assert.True(t, isEmpty)

	// Add data
	data := map[string]map[string]interface{}{
		"key1": {"name": "Alice"},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	// Should not be empty now
	isEmpty, err = storage.IsEmpty(ctx)
	assert.NoError(t, err)
	assert.False(t, isEmpty)
}

func TestJsonKVStorage_IndexDoneCallback(t *testing.T) {
	storage, tempDir := newTestJsonKVStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"key1": {"name": "Alice"},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	// Persist to disk
	err = storage.IndexDoneCallback(ctx)
	assert.NoError(t, err)

	// Verify file exists
	filePath := filepath.Join(tempDir, "kv_store_test.json")
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	// Create new storage and load data
	storage2 := NewJsonKVStorage(&StorageConfig{
		Namespace: "test",
		Workspace: tempDir,
	}, nil)

	err = storage2.Initialize(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, storage2.Count())

	result, err := storage2.GetByID(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "Alice", result["name"])
}

func TestJsonKVStorage_Drop(t *testing.T) {
	storage, tempDir := newTestJsonKVStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data and persist
	data := map[string]map[string]interface{}{
		"key1": {"name": "Alice"},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)
	err = storage.IndexDoneCallback(ctx)
	require.NoError(t, err)

	// Verify file exists
	filePath := filepath.Join(tempDir, "kv_store_test.json")
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	// Drop storage
	result, err := storage.Drop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "success", result["status"])
	assert.Equal(t, "data dropped", result["message"])

	// Verify storage is empty
	isEmpty, err := storage.IsEmpty(ctx)
	assert.NoError(t, err)
	assert.True(t, isEmpty)

	// Verify file is removed
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))
}

func TestJsonKVStorage_Keys(t *testing.T) {
	storage, tempDir := newTestJsonKVStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"key1": {"name": "Alice"},
		"key2": {"name": "Bob"},
		"key3": {"name": "Charlie"},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	keys := storage.Keys()
	assert.Len(t, keys, 3)
	assert.Contains(t, keys, "key1")
	assert.Contains(t, keys, "key2")
	assert.Contains(t, keys, "key3")
}

func TestJsonKVStorage_Finalize(t *testing.T) {
	storage, tempDir := newTestJsonKVStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data
	data := map[string]map[string]interface{}{
		"key1": {"name": "Alice"},
	}
	err = storage.Upsert(ctx, data)
	require.NoError(t, err)

	// Finalize should persist data
	err = storage.Finalize(ctx)
	assert.NoError(t, err)

	// Verify file exists
	filePath := filepath.Join(tempDir, "kv_store_test.json")
	_, err = os.Stat(filePath)
	assert.NoError(t, err)
}

func TestJsonKVStorage_ConcurrentAccess(t *testing.T) {
	storage, tempDir := newTestJsonKVStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	err := storage.Initialize(ctx)
	require.NoError(t, err)

	// Test concurrent reads and writes
	done := make(chan bool, 10)

	// Writers
	for i := 0; i < 5; i++ {
		go func(idx int) {
			data := map[string]map[string]interface{}{
				"concurrent_key_" + string(rune('A'+idx)): {"value": idx},
			}
			storage.Upsert(ctx, data)
			done <- true
		}(i)
	}

	// Readers
	for i := 0; i < 5; i++ {
		go func() {
			storage.GetByID(ctx, "concurrent_key_A")
			storage.Keys()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify data integrity
	assert.GreaterOrEqual(t, storage.Count(), 1)
}
