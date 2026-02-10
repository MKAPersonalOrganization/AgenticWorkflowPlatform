// Package coreconstants provides centralized configuration constants for LightRAG core.
// This module defines default values for configuration constants used across
// different parts of the LightRAG system.
package coreconstants

import "time"

// Server settings
const (
	DefaultWorkers       = 2
	DefaultMaxGraphNodes = 1000
)

// Extraction settings
const (
	DefaultSummaryLanguage     = "English"
	DefaultMaxGleaning         = 1
	DefaultEntityNameMaxLength = 256
)

// Entity types to extract if ENTITY_TYPES is not specified
var DefaultEntityTypes = []string{
	"Person",
	"Creature",
	"Organization",
	"Location",
	"Event",
	"Concept",
	"Method",
	"Content",
	"Data",
	"Artifact",
	"NaturalObject",
}

// Summary configuration
const (
	// Number of description fragments to trigger LLM summary
	DefaultForceLLMSummaryOnMerge = 8
	// Max description token size to trigger LLM summary
	DefaultSummaryMaxTokens = 1200
	// Recommended LLM summary output length in tokens
	DefaultSummaryLengthRecommended = 600
	// Maximum token size sent to LLM for summary
	DefaultSummaryContextSize = 12000
	// Maximum token size allowed for entity extraction input context
	DefaultMaxExtractInputTokens = 20480
)

// Separator for description, source_id and relation-key fields
// (Cannot be changed after data is inserted)
const GraphFieldSep = "<SEP>"

// Query and retrieval configuration
const (
	DefaultTopK               = 40
	DefaultChunkTopK          = 20
	DefaultMaxEntityTokens    = 6000
	DefaultMaxRelationTokens  = 8000
	DefaultMaxTotalTokens     = 30000
	DefaultCosineThreshold    = 0.2
	DefaultRelatedChunkNumber = 5
	DefaultKGChunkPickMethod  = "VECTOR"
)

// History configuration (deprecated - all conversation_history messages sent to LLM)
const DefaultHistoryTurns = 0

// Rerank configuration
const (
	DefaultMinRerankScore = 0.0
	DefaultRerankBinding  = "null"
)

// Source IDs limit configuration
const (
	DefaultMaxSourceIDsPerEntity   = 300
	DefaultMaxSourceIDsPerRelation = 300
)

// Source IDs limit methods
const (
	SourceIDsLimitMethodKeep = "KEEP" // Keep oldest (less merge action and faster)
	SourceIDsLimitMethodFIFO = "FIFO" // First in first out
)

// Default source IDs limit method
const DefaultSourceIDsLimitMethod = SourceIDsLimitMethodFIFO

// ValidSourceIDsLimitMethods contains all valid limit methods
var ValidSourceIDsLimitMethods = map[string]struct{}{
	SourceIDsLimitMethodKeep: {},
	SourceIDsLimitMethodFIFO: {},
}

// File path configuration
const (
	// Maximum number of file paths stored in entity/relation file_path field
	DefaultMaxFilePaths = 100
	// Field length of file_path in Milvus Schema for entity and relation
	DefaultMaxFilePathLength = 32768
	// Placeholder for more file paths in metadata
	DefaultFilePathMorePlaceholder = "truncated"
)

// LLM configuration
const DefaultTemperature = 1.0

// Async configuration
const (
	DefaultMaxAsync          = 4 // Default maximum async operations
	DefaultMaxParallelInsert = 2 // Default maximum parallel insert operations
)

// Embedding configuration
const (
	DefaultEmbeddingFuncMaxAsync = 8  // Default max async for embedding functions
	DefaultEmbeddingBatchNum     = 10 // Default batch size for embedding computations
)

// Timeout configuration
const (
	DefaultTimeout          = 300 * time.Second // Gunicorn worker timeout
	DefaultLLMTimeout       = 180 * time.Second // LLM timeout
	DefaultEmbeddingTimeout = 30 * time.Second  // Embedding timeout
)

// Logging configuration
const (
	DefaultLogMaxBytes    = 10485760       // Default 10MB
	DefaultLogBackupCount = 5              // Default 5 backups
	DefaultLogFilename    = "lightrag.log" // Default log filename
)

// Ollama server configuration
const (
	DefaultOllamaModelName = "lightrag"
	DefaultOllamaModelTag  = "latest"
	DefaultOllamaModelSize = 7365960935
	DefaultOllamaCreatedAt = "2024-01-15T00:00:00Z"
	DefaultOllamaDigest    = "sha256:lightrag"
)

// Chunk configuration
const (
	DefaultChunkSize        = 1200
	DefaultChunkOverlapSize = 100
)

// IsValidSourceIDsLimitMethod checks if a method is valid.
func IsValidSourceIDsLimitMethod(method string) bool {
	_, ok := ValidSourceIDsLimitMethods[method]
	return ok
}

// NormalizeSourceIDsLimitMethod normalizes and validates the limit method.
func NormalizeSourceIDsLimitMethod(method string) string {
	if method == "" {
		return DefaultSourceIDsLimitMethod
	}
	if IsValidSourceIDsLimitMethod(method) {
		return method
	}
	return DefaultSourceIDsLimitMethod
}
