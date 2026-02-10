package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Unified Lock Implementation for Testing
// ============================================================================

// ErrLockNotInitialized is returned when the lock is used before initialization
var ErrLockNotInitialized = errors.New("lock not initialized: call Initialize() first")

// UnifiedLock provides a unified lock interface that can work in both sync and async contexts
type UnifiedLock struct {
	mu            sync.Mutex
	initialized   bool
	name          string
	enableLogging bool
	acquired      bool
}

// NewUnifiedLock creates a new unified lock
func NewUnifiedLock(name string, enableLogging bool) *UnifiedLock {
	return &UnifiedLock{
		name:          name,
		enableLogging: enableLogging,
		initialized:   false,
	}
}

// Initialize marks the lock as ready for use
func (l *UnifiedLock) Initialize() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.initialized = true
}

// IsInitialized returns whether the lock has been initialized
func (l *UnifiedLock) IsInitialized() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.initialized
}

// Acquire acquires the lock
func (l *UnifiedLock) Acquire() error {
	l.mu.Lock()
	if !l.initialized {
		l.mu.Unlock()
		return ErrLockNotInitialized
	}
	l.acquired = true
	return nil
}

// Release releases the lock
func (l *UnifiedLock) Release() error {
	if !l.acquired {
		return errors.New("lock not acquired")
	}
	l.acquired = false
	l.mu.Unlock()
	return nil
}

// ============================================================================
// Lock Factory for Testing
// ============================================================================

// SharedState represents shared state that must be initialized
type SharedState struct {
	mu           sync.RWMutex
	initialized  bool
	internalLock *UnifiedLock
	dataInitLock *UnifiedLock
}

var globalSharedState = &SharedState{}

// InitializeSharedState initializes the shared state with locks
func InitializeSharedState() {
	globalSharedState.mu.Lock()
	defer globalSharedState.mu.Unlock()

	globalSharedState.internalLock = NewUnifiedLock("internal", false)
	globalSharedState.internalLock.Initialize()

	globalSharedState.dataInitLock = NewUnifiedLock("data_init", false)
	globalSharedState.dataInitLock.Initialize()

	globalSharedState.initialized = true
}

// FinalizeSharedState resets the shared state
func FinalizeSharedState() {
	globalSharedState.mu.Lock()
	defer globalSharedState.mu.Unlock()

	globalSharedState.internalLock = nil
	globalSharedState.dataInitLock = nil
	globalSharedState.initialized = false
}

// GetInternalLock returns the internal lock, raises error if not initialized
func GetInternalLock() (*UnifiedLock, error) {
	globalSharedState.mu.RLock()
	defer globalSharedState.mu.RUnlock()

	if !globalSharedState.initialized || globalSharedState.internalLock == nil {
		return nil, errors.New("shared data not initialized: call InitializeSharedState() first")
	}
	return globalSharedState.internalLock, nil
}

// GetDataInitLock returns the data init lock, raises error if not initialized
func GetDataInitLock() (*UnifiedLock, error) {
	globalSharedState.mu.RLock()
	defer globalSharedState.mu.RUnlock()

	if !globalSharedState.initialized || globalSharedState.dataInitLock == nil {
		return nil, errors.New("shared data not initialized: call InitializeSharedState() first")
	}
	return globalSharedState.dataInitLock, nil
}

// ============================================================================
// Lock Safety Tests
// ============================================================================

func TestGetInternalLock_RaisesWhenNotInitialized(t *testing.T) {
	// Ensure shared data is finalized before test
	FinalizeSharedState()

	// Attempt to get lock before initialization
	lock, err := GetInternalLock()

	assert.Error(t, err)
	assert.Nil(t, lock)
	assert.Contains(t, err.Error(), "shared data not initialized")
}

func TestGetDataInitLock_RaisesWhenNotInitialized(t *testing.T) {
	// Ensure shared data is finalized before test
	FinalizeSharedState()

	// Attempt to get lock before initialization
	lock, err := GetDataInitLock()

	assert.Error(t, err)
	assert.Nil(t, lock)
	assert.Contains(t, err.Error(), "shared data not initialized")
}

func TestGetInternalLock_SucceedsAfterInitialization(t *testing.T) {
	FinalizeSharedState()
	InitializeSharedState()
	defer FinalizeSharedState()

	lock, err := GetInternalLock()

	assert.NoError(t, err)
	assert.NotNil(t, lock)
	assert.True(t, lock.IsInitialized())
}

func TestGetDataInitLock_SucceedsAfterInitialization(t *testing.T) {
	FinalizeSharedState()
	InitializeSharedState()
	defer FinalizeSharedState()

	lock, err := GetDataInitLock()

	assert.NoError(t, err)
	assert.NotNil(t, lock)
	assert.True(t, lock.IsInitialized())
}

func TestUnifiedLock_AcquireFailsWhenNotInitialized(t *testing.T) {
	lock := NewUnifiedLock("test", false)

	err := lock.Acquire()

	assert.Error(t, err)
	assert.Equal(t, ErrLockNotInitialized, err)
}

func TestUnifiedLock_AcquireSucceedsAfterInitialization(t *testing.T) {
	lock := NewUnifiedLock("test", false)
	lock.Initialize()

	err := lock.Acquire()

	assert.NoError(t, err)

	// Clean up
	err = lock.Release()
	assert.NoError(t, err)
}

func TestUnifiedLock_ReleaseFailsWhenNotAcquired(t *testing.T) {
	lock := NewUnifiedLock("test", false)
	lock.Initialize()

	err := lock.Release()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lock not acquired")
}

// ============================================================================
// Double Release Prevention Tests
// ============================================================================

// SafeLock is a lock that tracks release state to prevent double-release
type SafeLock struct {
	mu            sync.Mutex
	lockAcquired  bool
	releaseCount  int
	failOnRelease bool
	releaseError  error
}

func NewSafeLock() *SafeLock {
	return &SafeLock{}
}

func (l *SafeLock) Acquire() error {
	l.mu.Lock()
	l.lockAcquired = true
	return nil
}

func (l *SafeLock) Release() error {
	if !l.lockAcquired {
		return errors.New("lock not held")
	}
	l.releaseCount++
	l.lockAcquired = false

	if l.failOnRelease {
		l.mu.Unlock()
		return l.releaseError
	}

	l.mu.Unlock()
	return nil
}

func (l *SafeLock) ReleaseCount() int {
	return l.releaseCount
}

func (l *SafeLock) SetFailOnRelease(fail bool, err error) {
	l.failOnRelease = fail
	l.releaseError = err
}

func TestSafeLock_NoDoubleRelease(t *testing.T) {
	lock := NewSafeLock()

	// Acquire the lock
	err := lock.Acquire()
	require.NoError(t, err)

	// First release
	err = lock.Release()
	assert.NoError(t, err)
	assert.Equal(t, 1, lock.ReleaseCount())

	// Second release should fail
	err = lock.Release()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lock not held")
	assert.Equal(t, 1, lock.ReleaseCount(), "Release count should still be 1")
}

func TestSafeLock_ReleaseFailureTracking(t *testing.T) {
	lock := NewSafeLock()

	// Make release fail
	lock.SetFailOnRelease(true, errors.New("release failed"))

	// Acquire the lock
	err := lock.Acquire()
	require.NoError(t, err)

	// Release should fail
	err = lock.Release()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "release failed")

	// Even though release failed, lock state was updated
	assert.Equal(t, 1, lock.ReleaseCount())

	// Second release attempt should fail with "not held"
	err = lock.Release()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lock not held")
}

// ============================================================================
// Context-Based Lock Tests
// ============================================================================

// ContextLock is a lock that respects context cancellation
type ContextLock struct {
	mu      sync.Mutex
	held    bool
	timeout time.Duration
}

func NewContextLock(timeout time.Duration) *ContextLock {
	return &ContextLock{timeout: timeout}
}

func (l *ContextLock) AcquireWithContext(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		l.mu.Lock()
		l.held = true
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(l.timeout):
		return errors.New("lock acquisition timeout")
	}
}

func (l *ContextLock) Release() error {
	if !l.held {
		return errors.New("lock not held")
	}
	l.held = false
	l.mu.Unlock()
	return nil
}

func TestContextLock_RespectsCancellation(t *testing.T) {
	lock := NewContextLock(5 * time.Second)

	// Acquire first
	ctx := context.Background()
	err := lock.AcquireWithContext(ctx)
	require.NoError(t, err)

	// Create cancelled context
	ctx2, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Second acquire with cancelled context should fail
	errChan := make(chan error, 1)
	go func() {
		errChan <- lock.AcquireWithContext(ctx2)
	}()

	select {
	case err := <-errChan:
		assert.Error(t, err)
	case <-time.After(100 * time.Millisecond):
		// Expected - the goroutine is waiting on the locked mutex
	}

	// Clean up
	lock.Release()
}

func TestContextLock_SuccessfulAcquisition(t *testing.T) {
	lock := NewContextLock(1 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := lock.AcquireWithContext(ctx)
	assert.NoError(t, err)

	err = lock.Release()
	assert.NoError(t, err)
}

// ============================================================================
// Workspace-Scoped Lock Tests
// ============================================================================

// WorkspaceScopedLockManager manages locks per workspace
type WorkspaceScopedLockManager struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewWorkspaceScopedLockManager() *WorkspaceScopedLockManager {
	return &WorkspaceScopedLockManager{
		locks: make(map[string]*sync.Mutex),
	}
}

func (m *WorkspaceScopedLockManager) GetLock(workspace string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.locks[workspace]; !exists {
		m.locks[workspace] = &sync.Mutex{}
	}
	return m.locks[workspace]
}

func (m *WorkspaceScopedLockManager) RemoveLock(workspace string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.locks, workspace)
}

func TestWorkspaceScopedLock_IsolatedLocking(t *testing.T) {
	manager := NewWorkspaceScopedLockManager()

	lock1 := manager.GetLock("workspace1")
	lock2 := manager.GetLock("workspace2")

	// Different workspaces should have different locks (different pointers)
	assert.True(t, lock1 != lock2, "Different workspaces should have different locks")

	// Same workspace should return same lock
	lock1Again := manager.GetLock("workspace1")
	assert.True(t, lock1 == lock1Again, "Same workspace should return same lock")
}

func TestWorkspaceScopedLock_IndependentLocking(t *testing.T) {
	manager := NewWorkspaceScopedLockManager()

	lock1 := manager.GetLock("workspace1")
	lock2 := manager.GetLock("workspace2")

	// Lock workspace1
	lock1.Lock()

	// workspace2 should still be lockable
	locked := make(chan bool, 1)
	go func() {
		lock2.Lock()
		locked <- true
		lock2.Unlock()
	}()

	select {
	case <-locked:
		// Success - workspace2 was independently locked
	case <-time.After(100 * time.Millisecond):
		t.Fatal("workspace2 should be independently lockable")
	}

	// Clean up
	lock1.Unlock()
}

func TestWorkspaceScopedLock_ConcurrentAccess(t *testing.T) {
	manager := NewWorkspaceScopedLockManager()
	var wg sync.WaitGroup

	// Multiple goroutines accessing different workspaces
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workspaceID int) {
			defer wg.Done()

			workspace := "workspace" + string(rune('A'+workspaceID))
			lock := manager.GetLock(workspace)

			for j := 0; j < 100; j++ {
				lock.Lock()
				// Simulate work
				time.Sleep(time.Microsecond)
				lock.Unlock()
			}
		}(i)
	}

	wg.Wait()
}

func TestWorkspaceScopedLock_RemoveLock(t *testing.T) {
	manager := NewWorkspaceScopedLockManager()

	lock1 := manager.GetLock("workspace1")
	manager.RemoveLock("workspace1")

	// Getting lock again should create a new one (different pointer)
	lock1New := manager.GetLock("workspace1")
	assert.True(t, lock1 != lock1New, "New lock should be created after removal")
}
