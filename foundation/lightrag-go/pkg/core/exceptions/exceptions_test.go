package exceptions

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPIStatusError(t *testing.T) {
	err := NewAPIStatusError("Test error", 500, "req-123", nil)

	assert.Equal(t, 500, err.StatusCode)
	assert.Equal(t, "req-123", err.RequestID)
	assert.Contains(t, err.Error(), "Test error")
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "req-123")
}

func TestAPIStatusErrorWithoutRequestID(t *testing.T) {
	err := NewAPIStatusError("Test error", 400, "", nil)

	assert.Equal(t, 400, err.StatusCode)
	assert.Empty(t, err.RequestID)
	assert.Contains(t, err.Error(), "Test error")
	assert.Contains(t, err.Error(), "400")
	assert.NotContains(t, err.Error(), "request_id")
}

func TestAPIConnectionError(t *testing.T) {
	err := NewAPIConnectionError("Connection failed", nil)

	assert.Equal(t, "Connection failed", err.Error())

	// Test default message
	err2 := NewAPIConnectionError("", nil)
	assert.Equal(t, "Connection error.", err2.Error())
}

func TestBadRequestError(t *testing.T) {
	err := NewBadRequestError("Invalid request", "req-456", nil)

	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	assert.Contains(t, err.Error(), "Invalid request")
}

func TestAuthenticationError(t *testing.T) {
	err := NewAuthenticationError("Invalid credentials", "", nil)

	assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
	assert.Contains(t, err.Error(), "Invalid credentials")
}

func TestPermissionDeniedError(t *testing.T) {
	err := NewPermissionDeniedError("Access denied", "", nil)

	assert.Equal(t, http.StatusForbidden, err.StatusCode)
	assert.Contains(t, err.Error(), "Access denied")
}

func TestNotFoundError(t *testing.T) {
	err := NewNotFoundError("Resource not found", "", nil)

	assert.Equal(t, http.StatusNotFound, err.StatusCode)
	assert.Contains(t, err.Error(), "Resource not found")
}

func TestConflictError(t *testing.T) {
	err := NewConflictError("Resource conflict", "", nil)

	assert.Equal(t, http.StatusConflict, err.StatusCode)
	assert.Contains(t, err.Error(), "Resource conflict")
}

func TestUnprocessableEntityError(t *testing.T) {
	err := NewUnprocessableEntityError("Validation failed", "", nil)

	assert.Equal(t, http.StatusUnprocessableEntity, err.StatusCode)
	assert.Contains(t, err.Error(), "Validation failed")
}

func TestRateLimitError(t *testing.T) {
	err := NewRateLimitError("Too many requests", "", nil)

	assert.Equal(t, http.StatusTooManyRequests, err.StatusCode)
	assert.Contains(t, err.Error(), "Too many requests")
}

func TestAPITimeoutError(t *testing.T) {
	err := NewAPITimeoutError(nil)

	assert.Contains(t, err.Error(), "timed out")
}

func TestStorageNotInitializedError(t *testing.T) {
	err := NewStorageNotInitializedError("JsonKVStorage")

	assert.Contains(t, err.Error(), "JsonKVStorage")
	assert.Contains(t, err.Error(), "not initialized")
	assert.Contains(t, err.Error(), "InitializeStorages")

	// Test default storage type
	err2 := NewStorageNotInitializedError("")
	assert.Contains(t, err2.Error(), "Storage")
}

func TestPipelineNotInitializedError(t *testing.T) {
	err := NewPipelineNotInitializedError("test_workspace")

	assert.Contains(t, err.Error(), "test_workspace")
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "InitializeStorages")
}

func TestPipelineCancelledException(t *testing.T) {
	err := NewPipelineCancelledException("User cancelled operation")

	assert.Equal(t, "User cancelled operation", err.Error())

	// Test default message
	err2 := NewPipelineCancelledException("")
	assert.Equal(t, "User cancelled", err2.Error())
}

func TestChunkTokenLimitExceededError(t *testing.T) {
	// Test without preview
	err := NewChunkTokenLimitExceededError(1500, 1000, "")

	assert.Contains(t, err.Error(), "1500")
	assert.Contains(t, err.Error(), "1000")
	assert.NotContains(t, err.Error(), "Preview")

	// Test with short preview
	err2 := NewChunkTokenLimitExceededError(1500, 1000, "Short preview")
	assert.Contains(t, err2.Error(), "Preview")
	assert.Contains(t, err2.Error(), "Short preview")

	// Test with long preview (should be truncated)
	longPreview := strings.Repeat("a", 100)
	err3 := NewChunkTokenLimitExceededError(1500, 1000, longPreview)
	assert.Contains(t, err3.Error(), "Preview")
	assert.LessOrEqual(t, len(err3.ChunkPreview), 100)
}

func TestDataMigrationError(t *testing.T) {
	err := NewDataMigrationError("Migration failed: incompatible schema")

	assert.Equal(t, "Migration failed: incompatible schema", err.Error())
}

func TestInvalidResponseError(t *testing.T) {
	err := NewInvalidResponseError("Invalid JSON response")

	assert.Equal(t, "Invalid JSON response", err.Error())
}

func TestEmbeddingDimensionMismatchError(t *testing.T) {
	err := NewEmbeddingDimensionMismatchError(1536, 768)

	assert.Contains(t, err.Error(), "1536")
	assert.Contains(t, err.Error(), "768")
	assert.Contains(t, err.Error(), "mismatch")
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "RateLimitError is retryable",
			err:      NewRateLimitError("rate limit", "", nil),
			expected: true,
		},
		{
			name:     "APIConnectionError is retryable",
			err:      NewAPIConnectionError("connection error", nil),
			expected: true,
		},
		{
			name:     "APITimeoutError is retryable",
			err:      NewAPITimeoutError(nil),
			expected: true,
		},
		{
			name:     "InvalidResponseError is retryable",
			err:      NewInvalidResponseError("invalid response"),
			expected: true,
		},
		{
			name:     "BadRequestError is not retryable",
			err:      NewBadRequestError("bad request", "", nil),
			expected: false,
		},
		{
			name:     "NotFoundError is not retryable",
			err:      NewNotFoundError("not found", "", nil),
			expected: false,
		},
		{
			name:     "StorageNotInitializedError is not retryable",
			err:      NewStorageNotInitializedError("test"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
