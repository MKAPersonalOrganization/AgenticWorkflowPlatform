// Package constants provides centralized configuration constants for LightRAG.
// This module defines default values for configuration constants used across
// different parts of the LightRAG system.
package constants

import "time"

// Server settings
const (
	DefaultWorkers       = 2
	DefaultMaxGraphNodes = 1000
	DefaultPort          = 9621
	DefaultHost          = "0.0.0.0"
)

// Extraction settings
const (
	DefaultSummaryLanguage     = "English"
	DefaultMaxGleaning         = 1
	DefaultEntityNameMaxLength = 256
	DefaultForeceLLMSummary    = 8
	DefaultSummaryMaxTokens    = 1200
	DefaultSummaryLengthRec    = 600
	DefaultSummaryContextSize  = 12000
	DefaultMaxExtractInputTkns = 20480
)

// Default entity types for extraction
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

// Graph field separator
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
	DefaultHistoryTurns       = 0
)

// Rerank configuration
const (
	DefaultMinRerankScore = 0.0
	DefaultRerankBinding  = "null"
)

// Source ID limits
const (
	DefaultMaxSourceIDsPerEntity   = 300
	DefaultMaxSourceIDsPerRelation = 300
	SourceIDsLimitMethodKeep       = "KEEP"
	SourceIDsLimitMethodFIFO       = "FIFO"
	DefaultSourceIDsLimitMethod    = SourceIDsLimitMethodFIFO
	DefaultMaxFilePaths            = 100
	DefaultMaxFilePathLength       = 32768
	DefaultFilePathMorePlaceholder = "truncated"
)

// LLM configuration
const (
	DefaultTemperature    = 1.0
	DefaultMaxAsync       = 4
	DefaultMaxParallelIns = 2
)

// Embedding configuration
const (
	DefaultEmbeddingFuncMaxAsync = 8
	DefaultEmbeddingBatchNum     = 10
)

// Timeouts
const (
	DefaultTimeout          = 300 * time.Second
	DefaultLLMTimeout       = 180 * time.Second
	DefaultEmbeddingTimeout = 60 * time.Second
)

// Logging configuration
const (
	DefaultLogMaxBytes    = 10485760 // 10MB
	DefaultLogBackupCount = 5
	DefaultLogFilename    = "lightrag.log"
)

// Ollama model defaults
const (
	DefaultOllamaModelName = "lightrag"
	DefaultOllamaModelTag  = "latest"
	DefaultOllamaModelSize = 7365960935
	DefaultOllamaCreatedAt = "2024-01-01T00:00:00.000000000Z"
	DefaultOllamaDigest    = "lightrag-digest-placeholder"
)

// Storage defaults
const (
	DefaultKVStorage        = "JsonKVStorage"
	DefaultVectorStorage    = "NanoVectorDBStorage"
	DefaultGraphStorage     = "NetworkXStorage"
	DefaultDocStatusStorage = "JsonDocStatusStorage"
)

// JWT and auth defaults
const (
	DefaultTokenExpireHours      = 48.0
	DefaultGuestTokenExpireHours = 24.0
	DefaultJWTAlgorithm          = "HS256"
	DefaultTokenSecret           = "lightrag-jwt-default-secret"
	DefaultTokenAutoRenew        = true
	DefaultTokenRenewThreshold   = 0.5
	TokenRenewalMinInterval      = 60 * time.Second
)

// File upload limits
const (
	DefaultMaxUploadSize = 104857600 // 100MB
)

// Chunk configuration
const (
	DefaultChunkSize        = 1200
	DefaultChunkOverlapSize = 100
)

// CORS configuration
const DefaultCORSOrigins = "*"

// Whitelist paths
const DefaultWhitelistPaths = "/health,/api/*"
