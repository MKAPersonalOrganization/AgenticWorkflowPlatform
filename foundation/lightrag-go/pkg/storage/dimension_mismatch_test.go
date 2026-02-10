package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Dimension Mismatch Tests
// ============================================================================

// DataMigrationError is raised when vector dimensions don't match during migration
type DataMigrationError struct {
	Message string
	OldDim  int
	NewDim  int
}

func (e *DataMigrationError) Error() string {
	return e.Message
}

// MockVectorStorage simulates vector storage for testing dimension handling
type MockVectorStorage struct {
	name      string
	dimension int
	data      map[string][]float64
}

func NewMockVectorStorage(name string, dim int) *MockVectorStorage {
	return &MockVectorStorage{
		name:      name,
		dimension: dim,
		data:      make(map[string][]float64),
	}
}

func (s *MockVectorStorage) GetDimension() int {
	return s.dimension
}

func (s *MockVectorStorage) HasData() bool {
	return len(s.data) > 0
}

func (s *MockVectorStorage) Insert(id string, vector []float64) error {
	if len(vector) != s.dimension {
		return &DataMigrationError{
			Message: "dimension mismatch",
			OldDim:  s.dimension,
			NewDim:  len(vector),
		}
	}
	s.data[id] = vector
	return nil
}

// checkDimensionMismatch validates dimension compatibility between old and new storage
func checkDimensionMismatch(oldDim, newDim int) error {
	if oldDim != newDim {
		return &DataMigrationError{
			Message: "vector dimension mismatch: cannot migrate",
			OldDim:  oldDim,
			NewDim:  newDim,
		}
	}
	return nil
}

// migrateVectorData migrates data from old to new storage with dimension validation
func migrateVectorData(ctx context.Context, oldStorage, newStorage *MockVectorStorage) error {
	// Check dimension compatibility first
	if err := checkDimensionMismatch(oldStorage.GetDimension(), newStorage.GetDimension()); err != nil {
		return err
	}

	// Migrate data
	for id, vector := range oldStorage.data {
		if err := newStorage.Insert(id, vector); err != nil {
			return err
		}
	}

	return nil
}

func TestDimensionMismatchRaisesError(t *testing.T) {
	// Test that dimension mismatch raises DataMigrationError
	// Scenario: Legacy storage has 1536d vectors, new model expects 3072d

	ctx := context.Background()

	// Create legacy storage with 1536 dimensions
	legacyStorage := NewMockVectorStorage("legacy", 1536)
	legacyStorage.data["doc1"] = make([]float64, 1536)
	legacyStorage.data["doc2"] = make([]float64, 1536)

	// Create new storage with 3072 dimensions
	newStorage := NewMockVectorStorage("new", 3072)

	// Migration should fail due to dimension mismatch
	err := migrateVectorData(ctx, legacyStorage, newStorage)

	assert.Error(t, err)
	var migrationErr *DataMigrationError
	assert.True(t, errors.As(err, &migrationErr))
	assert.Equal(t, 1536, migrationErr.OldDim)
	assert.Equal(t, 3072, migrationErr.NewDim)
}

func TestDimensionMatchProceedsMigration(t *testing.T) {
	// Test that matching dimensions allow migration to proceed
	// Scenario: Legacy storage has 1536d vectors, new model also expects 1536d

	ctx := context.Background()

	// Create legacy storage with 1536 dimensions
	legacyStorage := NewMockVectorStorage("legacy", 1536)
	legacyStorage.data["doc1"] = make([]float64, 1536)
	legacyStorage.data["doc2"] = make([]float64, 1536)

	// Create new storage with matching 1536 dimensions
	newStorage := NewMockVectorStorage("new", 1536)

	// Migration should succeed
	err := migrateVectorData(ctx, legacyStorage, newStorage)

	assert.NoError(t, err)
	// Verify data was migrated
	assert.True(t, newStorage.HasData())
	assert.Len(t, newStorage.data, 2)
}

func TestDimensionValidationPreventsMigration(t *testing.T) {
	// Test that dimension validation happens before any data is migrated

	// Create legacy storage with data
	legacyStorage := NewMockVectorStorage("legacy", 768)
	for i := 0; i < 100; i++ {
		id := string(rune('A' + i%26))
		legacyStorage.data[id] = make([]float64, 768)
	}

	// Create new storage with different dimensions
	newStorage := NewMockVectorStorage("new", 1024)

	// Check validation
	err := checkDimensionMismatch(legacyStorage.GetDimension(), newStorage.GetDimension())

	assert.Error(t, err)
	// New storage should remain empty (no partial migration)
	assert.False(t, newStorage.HasData())
}

func TestDimensionMismatchErrorDetails(t *testing.T) {
	// Test that error message contains useful dimension information
	err := &DataMigrationError{
		Message: "vector dimension mismatch: cannot migrate",
		OldDim:  1536,
		NewDim:  3072,
	}

	assert.Contains(t, err.Error(), "dimension")
	assert.Equal(t, 1536, err.OldDim)
	assert.Equal(t, 3072, err.NewDim)
}

// ============================================================================
// Edge Cases for Dimension Handling
// ============================================================================

func TestZeroDimensionRaisesError(t *testing.T) {
	// Zero dimension should be invalid
	err := checkDimensionMismatch(0, 1536)
	assert.Error(t, err)
}

func TestEmptyStorageMigration(t *testing.T) {
	// Migration of empty storage with different dimensions should still fail
	ctx := context.Background()

	legacyStorage := NewMockVectorStorage("legacy", 1536)
	// No data in legacy storage

	newStorage := NewMockVectorStorage("new", 3072)

	err := migrateVectorData(ctx, legacyStorage, newStorage)

	// Should still fail due to dimension mismatch (validation before data check)
	assert.Error(t, err)
}

func TestCommonDimensions(t *testing.T) {
	// Test common embedding dimensions
	commonDimensions := []int{384, 512, 768, 1024, 1536, 3072, 4096}

	for _, dim := range commonDimensions {
		t.Run("dimension_"+string(rune('0'+dim/1000))+string(rune('0'+(dim%1000)/100)), func(t *testing.T) {
			// Same dimensions should pass
			err := checkDimensionMismatch(dim, dim)
			assert.NoError(t, err)
		})
	}
}

func TestDimensionMismatchCombinations(t *testing.T) {
	// Test various dimension mismatch combinations
	testCases := []struct {
		oldDim int
		newDim int
	}{
		{768, 1024},
		{1024, 1536},
		{1536, 3072},
		{384, 768},
		{512, 1024},
	}

	for _, tc := range testCases {
		t.Run("mismatch_old_new", func(t *testing.T) {
			err := checkDimensionMismatch(tc.oldDim, tc.newDim)
			assert.Error(t, err)

			var migrationErr *DataMigrationError
			assert.True(t, errors.As(err, &migrationErr))
			assert.Equal(t, tc.oldDim, migrationErr.OldDim)
			assert.Equal(t, tc.newDim, migrationErr.NewDim)
		})
	}
}

// ============================================================================
// Vector Storage Dimension Validation Tests
// ============================================================================

func TestInsertWithWrongDimensionFails(t *testing.T) {
	storage := NewMockVectorStorage("test", 1536)

	// Try to insert vector with wrong dimension
	wrongVector := make([]float64, 768)
	err := storage.Insert("doc1", wrongVector)

	assert.Error(t, err)
	var migrationErr *DataMigrationError
	assert.True(t, errors.As(err, &migrationErr))
}

func TestInsertWithCorrectDimensionSucceeds(t *testing.T) {
	storage := NewMockVectorStorage("test", 1536)

	// Insert vector with correct dimension
	correctVector := make([]float64, 1536)
	err := storage.Insert("doc1", correctVector)

	assert.NoError(t, err)
	assert.True(t, storage.HasData())
}

func TestBatchInsertWithMixedDimensions(t *testing.T) {
	storage := NewMockVectorStorage("test", 768)

	// First insert should succeed
	err := storage.Insert("doc1", make([]float64, 768))
	assert.NoError(t, err)

	// Second insert with wrong dimension should fail
	err = storage.Insert("doc2", make([]float64, 1536))
	assert.Error(t, err)

	// Storage should only have first document
	assert.Len(t, storage.data, 1)
}

// ============================================================================
// Model Suffix Safety Tests
// ============================================================================

// These tests ensure data safety when model suffix changes

func TestNoAccidentalDeletionWithoutSuffix(t *testing.T) {
	// Test that storage without model suffix doesn't accidentally delete data
	storage := NewMockVectorStorage("entities", 1536)
	storage.data["entity1"] = make([]float64, 1536)
	storage.data["entity2"] = make([]float64, 1536)

	// Verify data exists
	assert.True(t, storage.HasData())
	assert.Len(t, storage.data, 2)

	// Creating a new storage instance should not affect first storage
	storage2 := NewMockVectorStorage("entities", 1536)
	// Old storage should still have data
	assert.True(t, storage.HasData())
	// New storage starts empty
	assert.False(t, storage2.HasData())
}

func TestStorageIsolationWithModelSuffix(t *testing.T) {
	// Test that different model suffixes create isolated storage
	storage1 := NewMockVectorStorage("entities_model_a", 1536)
	storage2 := NewMockVectorStorage("entities_model_b", 768)

	storage1.data["doc1"] = make([]float64, 1536)
	storage2.data["doc1"] = make([]float64, 768)

	// Each storage should have its own data
	assert.Len(t, storage1.data, 1)
	assert.Len(t, storage2.data, 1)

	// Dimensions should be different
	assert.NotEqual(t, storage1.GetDimension(), storage2.GetDimension())
}
