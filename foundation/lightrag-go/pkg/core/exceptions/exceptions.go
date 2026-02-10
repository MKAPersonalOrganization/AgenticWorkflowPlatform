// Package exceptions provides custom error types for LightRAG.
// This is a direct translation of Python lightrag/exceptions.py to Go.
package exceptions

import (
	"fmt"
	"net/http"
)

// APIStatusError is raised when an API response has a status code of 4xx or 5xx.
type APIStatusError struct {
	Message    string
	StatusCode int
	RequestID  string
	Body       interface{}
}

func (e *APIStatusError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("%s (status: %d, request_id: %s)", e.Message, e.StatusCode, e.RequestID)
	}
	return fmt.Sprintf("%s (status: %d)", e.Message, e.StatusCode)
}

// NewAPIStatusError creates a new APIStatusError.
func NewAPIStatusError(message string, statusCode int, requestID string, body interface{}) *APIStatusError {
	return &APIStatusError{
		Message:    message,
		StatusCode: statusCode,
		RequestID:  requestID,
		Body:       body,
	}
}

// APIConnectionError is raised when there is a connection error.
type APIConnectionError struct {
	Message string
	Request *http.Request
}

func (e *APIConnectionError) Error() string {
	return e.Message
}

// NewAPIConnectionError creates a new APIConnectionError.
func NewAPIConnectionError(message string, request *http.Request) *APIConnectionError {
	if message == "" {
		message = "Connection error."
	}
	return &APIConnectionError{
		Message: message,
		Request: request,
	}
}

// BadRequestError is raised when a request returns a 400 status code.
type BadRequestError struct {
	*APIStatusError
}

// NewBadRequestError creates a new BadRequestError.
func NewBadRequestError(message string, requestID string, body interface{}) *BadRequestError {
	return &BadRequestError{
		APIStatusError: NewAPIStatusError(message, http.StatusBadRequest, requestID, body),
	}
}

// AuthenticationError is raised when a request returns a 401 status code.
type AuthenticationError struct {
	*APIStatusError
}

// NewAuthenticationError creates a new AuthenticationError.
func NewAuthenticationError(message string, requestID string, body interface{}) *AuthenticationError {
	return &AuthenticationError{
		APIStatusError: NewAPIStatusError(message, http.StatusUnauthorized, requestID, body),
	}
}

// PermissionDeniedError is raised when a request returns a 403 status code.
type PermissionDeniedError struct {
	*APIStatusError
}

// NewPermissionDeniedError creates a new PermissionDeniedError.
func NewPermissionDeniedError(message string, requestID string, body interface{}) *PermissionDeniedError {
	return &PermissionDeniedError{
		APIStatusError: NewAPIStatusError(message, http.StatusForbidden, requestID, body),
	}
}

// NotFoundError is raised when a request returns a 404 status code.
type NotFoundError struct {
	*APIStatusError
}

// NewNotFoundError creates a new NotFoundError.
func NewNotFoundError(message string, requestID string, body interface{}) *NotFoundError {
	return &NotFoundError{
		APIStatusError: NewAPIStatusError(message, http.StatusNotFound, requestID, body),
	}
}

// ConflictError is raised when a request returns a 409 status code.
type ConflictError struct {
	*APIStatusError
}

// NewConflictError creates a new ConflictError.
func NewConflictError(message string, requestID string, body interface{}) *ConflictError {
	return &ConflictError{
		APIStatusError: NewAPIStatusError(message, http.StatusConflict, requestID, body),
	}
}

// UnprocessableEntityError is raised when a request returns a 422 status code.
type UnprocessableEntityError struct {
	*APIStatusError
}

// NewUnprocessableEntityError creates a new UnprocessableEntityError.
func NewUnprocessableEntityError(message string, requestID string, body interface{}) *UnprocessableEntityError {
	return &UnprocessableEntityError{
		APIStatusError: NewAPIStatusError(message, http.StatusUnprocessableEntity, requestID, body),
	}
}

// RateLimitError is raised when a request returns a 429 status code.
type RateLimitError struct {
	*APIStatusError
}

// NewRateLimitError creates a new RateLimitError.
func NewRateLimitError(message string, requestID string, body interface{}) *RateLimitError {
	return &RateLimitError{
		APIStatusError: NewAPIStatusError(message, http.StatusTooManyRequests, requestID, body),
	}
}

// APITimeoutError is raised when a request times out.
type APITimeoutError struct {
	*APIConnectionError
}

// NewAPITimeoutError creates a new APITimeoutError.
func NewAPITimeoutError(request *http.Request) *APITimeoutError {
	return &APITimeoutError{
		APIConnectionError: NewAPIConnectionError("Request timed out.", request),
	}
}

// StorageNotInitializedError is raised when storage operations are attempted before initialization.
type StorageNotInitializedError struct {
	StorageType string
}

func (e *StorageNotInitializedError) Error() string {
	return fmt.Sprintf(
		"%s not initialized. Please ensure proper initialization:\n\n"+
			"  rag := lightrag.New(...)\n"+
			"  rag.InitializeStorages()  // Required - auto-initializes pipeline_status\n\n"+
			"See: https://github.com/HKUDS/LightRAG#important-initialization-requirements",
		e.StorageType,
	)
}

// NewStorageNotInitializedError creates a new StorageNotInitializedError.
func NewStorageNotInitializedError(storageType string) *StorageNotInitializedError {
	if storageType == "" {
		storageType = "Storage"
	}
	return &StorageNotInitializedError{
		StorageType: storageType,
	}
}

// PipelineNotInitializedError is raised when pipeline status is accessed before initialization.
type PipelineNotInitializedError struct {
	Namespace string
}

func (e *PipelineNotInitializedError) Error() string {
	return fmt.Sprintf(
		"Pipeline namespace '%s' not found.\n\n"+
			"Pipeline status should be auto-initialized by InitializeStorages().\n"+
			"If you see this error, please ensure:\n\n"+
			"  1. You called rag.InitializeStorages()\n"+
			"  2. For multi-workspace setups, each LightRAG instance was properly initialized\n\n"+
			"Standard initialization:\n"+
			"  rag := lightrag.New(lightrag.WithWorkspace(\"your_workspace\"))\n"+
			"  rag.InitializeStorages()  // Auto-initializes pipeline_status",
		e.Namespace,
	)
}

// NewPipelineNotInitializedError creates a new PipelineNotInitializedError.
func NewPipelineNotInitializedError(namespace string) *PipelineNotInitializedError {
	return &PipelineNotInitializedError{
		Namespace: namespace,
	}
}

// PipelineCancelledException is raised when pipeline processing is cancelled by user request.
type PipelineCancelledException struct {
	Message string
}

func (e *PipelineCancelledException) Error() string {
	return e.Message
}

// NewPipelineCancelledException creates a new PipelineCancelledException.
func NewPipelineCancelledException(message string) *PipelineCancelledException {
	if message == "" {
		message = "User cancelled"
	}
	return &PipelineCancelledException{
		Message: message,
	}
}

// ChunkTokenLimitExceededError is raised when a chunk exceeds the configured token limit.
type ChunkTokenLimitExceededError struct {
	ChunkTokens     int
	ChunkTokenLimit int
	ChunkPreview    string
}

func (e *ChunkTokenLimitExceededError) Error() string {
	preview := ""
	if e.ChunkPreview != "" {
		truncated := e.ChunkPreview
		if len(truncated) > 80 {
			truncated = truncated[:80]
		}
		preview = fmt.Sprintf(" Preview: '%s'", truncated)
	}
	return fmt.Sprintf(
		"Chunk token length %d exceeds chunk_token_size %d.%s",
		e.ChunkTokens,
		e.ChunkTokenLimit,
		preview,
	)
}

// NewChunkTokenLimitExceededError creates a new ChunkTokenLimitExceededError.
func NewChunkTokenLimitExceededError(chunkTokens, chunkTokenLimit int, chunkPreview string) *ChunkTokenLimitExceededError {
	return &ChunkTokenLimitExceededError{
		ChunkTokens:     chunkTokens,
		ChunkTokenLimit: chunkTokenLimit,
		ChunkPreview:    chunkPreview,
	}
}

// DataMigrationError is raised when data migration from legacy collection/table fails.
type DataMigrationError struct {
	Message string
}

func (e *DataMigrationError) Error() string {
	return e.Message
}

// NewDataMigrationError creates a new DataMigrationError.
func NewDataMigrationError(message string) *DataMigrationError {
	return &DataMigrationError{
		Message: message,
	}
}

// InvalidResponseError is a custom exception for triggering retry mechanism.
type InvalidResponseError struct {
	Message string
}

func (e *InvalidResponseError) Error() string {
	return e.Message
}

// NewInvalidResponseError creates a new InvalidResponseError.
func NewInvalidResponseError(message string) *InvalidResponseError {
	return &InvalidResponseError{
		Message: message,
	}
}

// EmbeddingDimensionMismatchError is raised when embedding dimensions don't match.
type EmbeddingDimensionMismatchError struct {
	Expected int
	Actual   int
}

func (e *EmbeddingDimensionMismatchError) Error() string {
	return fmt.Sprintf(
		"Embedding dimension mismatch: expected %d, got %d",
		e.Expected,
		e.Actual,
	)
}

// NewEmbeddingDimensionMismatchError creates a new EmbeddingDimensionMismatchError.
func NewEmbeddingDimensionMismatchError(expected, actual int) *EmbeddingDimensionMismatchError {
	return &EmbeddingDimensionMismatchError{
		Expected: expected,
		Actual:   actual,
	}
}

// IsRetryableError checks if an error is retryable.
func IsRetryableError(err error) bool {
	switch err.(type) {
	case *RateLimitError, *APIConnectionError, *APITimeoutError, *InvalidResponseError:
		return true
	default:
		return false
	}
}
