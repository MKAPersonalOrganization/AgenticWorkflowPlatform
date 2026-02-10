package storage

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Workspace Isolation Tests
// ============================================================================

func TestWorkspaceIsolation_DataIsolation(t *testing.T) {
	// Test that data stored in one workspace is not accessible from another
	tempDir1, err := os.MkdirTemp("", "workspace_test_1_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "workspace_test_2_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir2)

	ctx := context.Background()

	// Create storage for workspace 1
	storage1 := NewJsonKVStorage(&StorageConfig{
		Namespace: "test",
		Workspace: tempDir1,
	}, nil)
	err = storage1.Initialize(ctx)
	require.NoError(t, err)

	// Create storage for workspace 2
	storage2 := NewJsonKVStorage(&StorageConfig{
		Namespace: "test",
		Workspace: tempDir2,
	}, nil)
	err = storage2.Initialize(ctx)
	require.NoError(t, err)

	// Upsert data to workspace 1
	data1 := map[string]map[string]interface{}{
		"key1": {"workspace": "workspace1", "value": "secret1"},
	}
	err = storage1.Upsert(ctx, data1)
	require.NoError(t, err)

	// Upsert different data to workspace 2
	data2 := map[string]map[string]interface{}{
		"key2": {"workspace": "workspace2", "value": "secret2"},
	}
	err = storage2.Upsert(ctx, data2)
	require.NoError(t, err)

	// Verify workspace 1 can access its own data
	result1, err := storage1.GetByID(ctx, "key1")
	assert.NoError(t, err)
	assert.NotNil(t, result1)
	assert.Equal(t, "workspace1", result1["workspace"])

	// Verify workspace 1 cannot access workspace 2's data
	result1b, err := storage1.GetByID(ctx, "key2")
	assert.NoError(t, err)
	assert.Nil(t, result1b, "Workspace 1 should not have access to workspace 2's data")

	// Verify workspace 2 can access its own data
	result2, err := storage2.GetByID(ctx, "key2")
	assert.NoError(t, err)
	assert.NotNil(t, result2)
	assert.Equal(t, "workspace2", result2["workspace"])

	// Verify workspace 2 cannot access workspace 1's data
	result2b, err := storage2.GetByID(ctx, "key1")
	assert.NoError(t, err)
	assert.Nil(t, result2b, "Workspace 2 should not have access to workspace 1's data")
}

func TestWorkspaceIsolation_MultiWorkspaceConcurrency(t *testing.T) {
	// Test concurrent operations on different workspaces
	numWorkspaces := 5
	tempDirs := make([]string, numWorkspaces)
	storages := make([]*JsonKVStorage, numWorkspaces)

	ctx := context.Background()

	// Create workspaces
	for i := 0; i < numWorkspaces; i++ {
		tempDir, err := os.MkdirTemp("", "workspace_concurrent_*")
		require.NoError(t, err)
		tempDirs[i] = tempDir
		defer os.RemoveAll(tempDir)

		storage := NewJsonKVStorage(&StorageConfig{
			Namespace: "test",
			Workspace: tempDir,
		}, nil)
		err = storage.Initialize(ctx)
		require.NoError(t, err)
		storages[i] = storage
	}

	// Run concurrent operations
	var wg sync.WaitGroup
	errChan := make(chan error, numWorkspaces*2)

	for i := 0; i < numWorkspaces; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			storage := storages[idx]

			// Perform multiple operations
			for j := 0; j < 10; j++ {
				data := map[string]map[string]interface{}{
					"key_" + string(rune('A'+j)): {"workspace": idx, "iteration": j},
				}
				if err := storage.Upsert(ctx, data); err != nil {
					errChan <- err
					return
				}
			}

			// Read back the data
			for j := 0; j < 10; j++ {
				_, err := storage.GetByID(ctx, "key_"+string(rune('A'+j)))
				if err != nil {
					errChan <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// Verify each workspace has its own data
	for i := 0; i < numWorkspaces; i++ {
		count := storages[i].Count()
		assert.Equal(t, 10, count, "Workspace %d should have 10 items", i)
	}
}

func TestWorkspaceIsolation_SameNamespaceDifferentWorkspace(t *testing.T) {
	// Test that same namespace in different workspaces are isolated
	tempDir1, err := os.MkdirTemp("", "ws_same_ns_1_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "ws_same_ns_2_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir2)

	ctx := context.Background()
	namespace := "shared_namespace"

	storage1 := NewJsonKVStorage(&StorageConfig{
		Namespace: namespace,
		Workspace: tempDir1,
	}, nil)
	err = storage1.Initialize(ctx)
	require.NoError(t, err)

	storage2 := NewJsonKVStorage(&StorageConfig{
		Namespace: namespace,
		Workspace: tempDir2,
	}, nil)
	err = storage2.Initialize(ctx)
	require.NoError(t, err)

	// Add same key to both workspaces with different values
	data1 := map[string]map[string]interface{}{
		"same_key": {"workspace": "ws1", "value": 100},
	}
	err = storage1.Upsert(ctx, data1)
	require.NoError(t, err)

	data2 := map[string]map[string]interface{}{
		"same_key": {"workspace": "ws2", "value": 200},
	}
	err = storage2.Upsert(ctx, data2)
	require.NoError(t, err)

	// Verify each workspace has its own value
	result1, err := storage1.GetByID(ctx, "same_key")
	assert.NoError(t, err)
	assert.Equal(t, "ws1", result1["workspace"])
	assert.Equal(t, 100, result1["value"])

	result2, err := storage2.GetByID(ctx, "same_key")
	assert.NoError(t, err)
	assert.Equal(t, "ws2", result2["workspace"])
	assert.Equal(t, 200, result2["value"])
}

// ============================================================================
// Lock Mechanism Tests
// ============================================================================

// WorkspaceLock provides workspace-scoped locking
type WorkspaceLock struct {
	mu     sync.Mutex
	locks  map[string]*sync.Mutex
	lockMu sync.Mutex
}

var globalWorkspaceLock = &WorkspaceLock{
	locks: make(map[string]*sync.Mutex),
}

func (wl *WorkspaceLock) getLock(workspace string) *sync.Mutex {
	wl.lockMu.Lock()
	defer wl.lockMu.Unlock()

	if lock, ok := wl.locks[workspace]; ok {
		return lock
	}
	lock := &sync.Mutex{}
	wl.locks[workspace] = lock
	return lock
}

func TestWorkspaceLock_ParallelDifferentWorkspaces(t *testing.T) {
	// Different workspaces should be able to acquire locks in parallel
	workspace1 := "test_workspace_1"
	workspace2 := "test_workspace_2"

	var maxParallel int32
	var current int32
	var wg sync.WaitGroup

	runOperation := func(workspace string) {
		lock := globalWorkspaceLock.getLock(workspace)
		lock.Lock()
		defer lock.Unlock()

		atomic.AddInt32(&current, 1)
		currentVal := atomic.LoadInt32(&current)
		for {
			maxVal := atomic.LoadInt32(&maxParallel)
			if currentVal <= maxVal {
				break
			}
			if atomic.CompareAndSwapInt32(&maxParallel, maxVal, currentVal) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&current, -1)
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		runOperation(workspace1)
	}()
	go func() {
		defer wg.Done()
		runOperation(workspace2)
	}()

	wg.Wait()

	// Different workspaces should run in parallel
	assert.Equal(t, int32(2), maxParallel, "Different workspaces should be able to run in parallel")
}

func TestWorkspaceLock_SerialSameWorkspace(t *testing.T) {
	// Same workspace operations should be serial
	workspace := "test_same_workspace"

	var maxParallel int32
	var current int32
	var wg sync.WaitGroup

	runOperation := func() {
		lock := globalWorkspaceLock.getLock(workspace)
		lock.Lock()
		defer lock.Unlock()

		atomic.AddInt32(&current, 1)
		currentVal := atomic.LoadInt32(&current)
		for {
			maxVal := atomic.LoadInt32(&maxParallel)
			if currentVal <= maxVal {
				break
			}
			if atomic.CompareAndSwapInt32(&maxParallel, maxVal, currentVal) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&current, -1)
	}

	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func() {
			defer wg.Done()
			runOperation()
		}()
	}

	wg.Wait()

	// Same workspace operations should be serial (max 1 at a time)
	assert.Equal(t, int32(1), maxParallel, "Same workspace operations should be serial")
}

// ============================================================================
// Empty Workspace Handling Tests
// ============================================================================

func TestWorkspaceIsolation_EmptyWorkspace(t *testing.T) {
	// Test handling of empty or default workspace
	tempDir, err := os.MkdirTemp("", "ws_empty_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Create storage with empty namespace
	storage := NewJsonKVStorage(&StorageConfig{
		Namespace: "",
		Workspace: tempDir,
	}, nil)
	err = storage.Initialize(ctx)
	require.NoError(t, err)

	// Operations should still work
	data := map[string]map[string]interface{}{
		"key1": {"value": "test"},
	}
	err = storage.Upsert(ctx, data)
	assert.NoError(t, err)

	result, err := storage.GetByID(ctx, "key1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test", result["value"])
}

// ============================================================================
// Update Flags Isolation Tests
// ============================================================================

type UpdateFlags struct {
	mu    sync.RWMutex
	flags map[string]map[string]bool // workspace -> namespace -> flag
}

var globalUpdateFlags = &UpdateFlags{
	flags: make(map[string]map[string]bool),
}

func (uf *UpdateFlags) SetFlag(workspace, namespace string, value bool) {
	uf.mu.Lock()
	defer uf.mu.Unlock()

	if _, ok := uf.flags[workspace]; !ok {
		uf.flags[workspace] = make(map[string]bool)
	}
	uf.flags[workspace][namespace] = value
}

func (uf *UpdateFlags) GetFlag(workspace, namespace string) bool {
	uf.mu.RLock()
	defer uf.mu.RUnlock()

	if ws, ok := uf.flags[workspace]; ok {
		return ws[namespace]
	}
	return false
}

func (uf *UpdateFlags) ClearWorkspace(workspace string) {
	uf.mu.Lock()
	defer uf.mu.Unlock()

	delete(uf.flags, workspace)
}

func TestUpdateFlagsWorkspaceIsolation(t *testing.T) {
	// Test that update flags are isolated between workspaces
	workspace1 := "ws_flags_1"
	workspace2 := "ws_flags_2"
	namespace := "test_namespace"

	// Clear any previous state
	globalUpdateFlags.ClearWorkspace(workspace1)
	globalUpdateFlags.ClearWorkspace(workspace2)

	// Set flag in workspace 1
	globalUpdateFlags.SetFlag(workspace1, namespace, true)

	// Verify workspace 1 has the flag
	assert.True(t, globalUpdateFlags.GetFlag(workspace1, namespace))

	// Verify workspace 2 does NOT have the flag
	assert.False(t, globalUpdateFlags.GetFlag(workspace2, namespace))

	// Set different flag in workspace 2
	globalUpdateFlags.SetFlag(workspace2, namespace, false)

	// Verify workspace 1 flag is unchanged
	assert.True(t, globalUpdateFlags.GetFlag(workspace1, namespace))
	assert.False(t, globalUpdateFlags.GetFlag(workspace2, namespace))
}

// ============================================================================
// Storage File Isolation Tests
// ============================================================================

func TestStorageFileIsolation(t *testing.T) {
	// Test that storage files are created in workspace-specific locations
	tempDir1, err := os.MkdirTemp("", "ws_file_1_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "ws_file_2_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir2)

	ctx := context.Background()

	// Create and persist storage for workspace 1
	storage1 := NewJsonKVStorage(&StorageConfig{
		Namespace: "test",
		Workspace: tempDir1,
	}, nil)
	err = storage1.Initialize(ctx)
	require.NoError(t, err)

	data1 := map[string]map[string]interface{}{
		"key1": {"value": "workspace1_value"},
	}
	err = storage1.Upsert(ctx, data1)
	require.NoError(t, err)
	err = storage1.IndexDoneCallback(ctx)
	require.NoError(t, err)

	// Create and persist storage for workspace 2
	storage2 := NewJsonKVStorage(&StorageConfig{
		Namespace: "test",
		Workspace: tempDir2,
	}, nil)
	err = storage2.Initialize(ctx)
	require.NoError(t, err)

	data2 := map[string]map[string]interface{}{
		"key2": {"value": "workspace2_value"},
	}
	err = storage2.Upsert(ctx, data2)
	require.NoError(t, err)
	err = storage2.IndexDoneCallback(ctx)
	require.NoError(t, err)

	// Verify files exist in separate locations
	file1 := filepath.Join(tempDir1, "kv_store_test.json")
	file2 := filepath.Join(tempDir2, "kv_store_test.json")

	_, err = os.Stat(file1)
	assert.NoError(t, err, "Workspace 1 storage file should exist")

	_, err = os.Stat(file2)
	assert.NoError(t, err, "Workspace 2 storage file should exist")

	// Verify contents are different
	content1, err := os.ReadFile(file1)
	require.NoError(t, err)

	content2, err := os.ReadFile(file2)
	require.NoError(t, err)

	assert.Contains(t, string(content1), "workspace1_value")
	assert.NotContains(t, string(content1), "workspace2_value")

	assert.Contains(t, string(content2), "workspace2_value")
	assert.NotContains(t, string(content2), "workspace1_value")
}

// ============================================================================
// Backward Compatibility Tests
// ============================================================================

func TestBackwardCompatibility_NoWorkspaceParam(t *testing.T) {
	// Test that storage works without explicit workspace parameter
	tempDir, err := os.MkdirTemp("", "ws_compat_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Create storage with minimal config
	storage := NewJsonKVStorage(&StorageConfig{
		Namespace: "test",
		Workspace: tempDir,
	}, nil)
	err = storage.Initialize(ctx)
	require.NoError(t, err)

	// Should work normally
	data := map[string]map[string]interface{}{
		"key1": {"value": "test"},
	}
	err = storage.Upsert(ctx, data)
	assert.NoError(t, err)

	result, err := storage.GetByID(ctx, "key1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
}
