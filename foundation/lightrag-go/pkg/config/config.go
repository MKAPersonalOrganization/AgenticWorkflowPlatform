// Package config provides configuration management for LightRAG API.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/lightrag/lightrag-go/pkg/constants"
)

// Config holds all configuration values for the LightRAG API server.
type Config struct {
	// Server configuration
	Host    string
	Port    int
	Workers int

	// Directory configuration
	WorkingDir string
	InputDir   string

	// Timeout configuration
	Timeout time.Duration

	// RAG configuration
	MaxAsync                 int
	SummaryMaxTokens         int
	SummaryContextSize       int
	SummaryLengthRecommended int
	MaxParallelInsert        int
	MaxGraphNodes            int

	// Logging configuration
	LogLevel string
	Verbose  bool

	// API Key
	APIKey string

	// SSL configuration
	SSL         bool
	SSLCertFile string
	SSLKeyFile  string

	// Ollama model configuration
	SimulatedModelName string
	SimulatedModelTag  string

	// Workspace
	Workspace string

	// LLM and embedding bindings
	LLMBinding             string
	EmbeddingBinding       string
	RerankBinding          string
	LLMBindingHost         string
	EmbeddingBindingHost   string
	LLMBindingAPIKey       string
	EmbeddingBindingAPIKey string

	// Model configuration
	LLMModel         string
	EmbeddingModel   string
	EmbeddingDim     *int
	EmbeddingSendDim bool

	// Chunk configuration
	ChunkSize        int
	ChunkOverlapSize int

	// LLM cache configuration
	EnableLLMCacheForExtract bool
	EnableLLMCache           bool

	// Document loading engine
	DocumentLoadingEngine string
	PDFDecryptPassword    string

	// Query configuration
	HistoryTurns       int
	TopK               int
	ChunkTopK          int
	MaxEntityTokens    int
	MaxRelationTokens  int
	MaxTotalTokens     int
	CosineThreshold    float64
	RelatedChunkNumber int

	// Rerank configuration
	RerankModel         string
	RerankBindingHost   string
	RerankBindingAPIKey string
	MinRerankScore      float64

	// Storage configuration
	KVStorage        string
	DocStatusStorage string
	GraphStorage     string
	VectorStorage    string

	// CORS configuration
	CORSOrigins        string
	CORSAllowedOrigins []string

	// Language configuration
	SummaryLanguage string
	EntityTypes     []string

	// Auth configuration
	WhitelistPaths        string
	AuthAccounts          string
	AuthUsername          string
	AuthPassword          string
	AuthDisabled          bool
	TokenSecret           string
	TokenExpireHours      float64
	GuestTokenExpireHours float64
	JWTAlgorithm          string
	TokenAutoRenew        bool
	TokenRenewThreshold   float64

	// Web UI configuration
	EnableWebUI bool

	// Embedding configuration
	ForceLLMSummaryOnMerge int
	EmbeddingFuncMaxAsync  int
	EmbeddingBatchNum      int
	EmbeddingTokenLimit    *int

	// File upload configuration
	MaxUploadSize *int64
}

// IsProduction returns true if the server is running in production mode.
func (c *Config) IsProduction() bool {
	env := os.Getenv("GO_ENV")
	return env == "production" || env == "prod"
}

// DefaultConfig returns a new Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Host:                     constants.DefaultHost,
		Port:                     constants.DefaultPort,
		Workers:                  constants.DefaultWorkers,
		WorkingDir:               "./rag_storage",
		InputDir:                 "./inputs",
		Timeout:                  constants.DefaultTimeout,
		MaxAsync:                 constants.DefaultMaxAsync,
		SummaryMaxTokens:         constants.DefaultSummaryMaxTokens,
		SummaryContextSize:       constants.DefaultSummaryContextSize,
		SummaryLengthRecommended: constants.DefaultSummaryLengthRec,
		MaxParallelInsert:        constants.DefaultMaxParallelIns,
		MaxGraphNodes:            constants.DefaultMaxGraphNodes,
		LogLevel:                 "INFO",
		Verbose:                  false,
		SimulatedModelName:       constants.DefaultOllamaModelName,
		SimulatedModelTag:        constants.DefaultOllamaModelTag,
		LLMBinding:               "ollama",
		EmbeddingBinding:         "ollama",
		RerankBinding:            constants.DefaultRerankBinding,
		LLMModel:                 "mistral-nemo:latest",
		ChunkSize:                constants.DefaultChunkSize,
		ChunkOverlapSize:         constants.DefaultChunkOverlapSize,
		EnableLLMCacheForExtract: true,
		EnableLLMCache:           true,
		DocumentLoadingEngine:    "DEFAULT",
		HistoryTurns:             constants.DefaultHistoryTurns,
		TopK:                     constants.DefaultTopK,
		ChunkTopK:                constants.DefaultChunkTopK,
		MaxEntityTokens:          constants.DefaultMaxEntityTokens,
		MaxRelationTokens:        constants.DefaultMaxRelationTokens,
		MaxTotalTokens:           constants.DefaultMaxTotalTokens,
		CosineThreshold:          constants.DefaultCosineThreshold,
		RelatedChunkNumber:       constants.DefaultRelatedChunkNumber,
		MinRerankScore:           constants.DefaultMinRerankScore,
		KVStorage:                constants.DefaultKVStorage,
		DocStatusStorage:         constants.DefaultDocStatusStorage,
		GraphStorage:             constants.DefaultGraphStorage,
		VectorStorage:            constants.DefaultVectorStorage,
		CORSOrigins:              constants.DefaultCORSOrigins,
		CORSAllowedOrigins:       []string{"*"},
		SummaryLanguage:          constants.DefaultSummaryLanguage,
		EntityTypes:              constants.DefaultEntityTypes,
		WhitelistPaths:           constants.DefaultWhitelistPaths,
		AuthDisabled:             false,
		TokenSecret:              constants.DefaultTokenSecret,
		TokenExpireHours:         constants.DefaultTokenExpireHours,
		GuestTokenExpireHours:    constants.DefaultGuestTokenExpireHours,
		JWTAlgorithm:             constants.DefaultJWTAlgorithm,
		TokenAutoRenew:           constants.DefaultTokenAutoRenew,
		TokenRenewThreshold:      constants.DefaultTokenRenewThreshold,
		ForceLLMSummaryOnMerge:   constants.DefaultForeceLLMSummary,
		EmbeddingFuncMaxAsync:    constants.DefaultEmbeddingFuncMaxAsync,
		EmbeddingBatchNum:        constants.DefaultEmbeddingBatchNum,
		EnableWebUI:              false,
	}
}

// LoadFromEnv loads configuration from environment variables.
func (c *Config) LoadFromEnv() {
	// Load .env file if it exists
	_ = godotenv.Load(".env")

	// Server configuration
	c.Host = getEnvString("HOST", c.Host)
	c.Port = getEnvInt("PORT", c.Port)
	c.Workers = getEnvInt("WORKERS", c.Workers)

	// Directory configuration
	c.WorkingDir = getEnvString("WORKING_DIR", c.WorkingDir)
	c.InputDir = getEnvString("INPUT_DIR", c.InputDir)

	// Timeout configuration
	if timeout := getEnvInt("TIMEOUT", int(c.Timeout.Seconds())); timeout > 0 {
		c.Timeout = time.Duration(timeout) * time.Second
	}

	// RAG configuration
	c.MaxAsync = getEnvInt("MAX_ASYNC", c.MaxAsync)
	c.SummaryMaxTokens = getEnvInt("SUMMARY_MAX_TOKENS", c.SummaryMaxTokens)
	c.SummaryContextSize = getEnvInt("SUMMARY_CONTEXT_SIZE", c.SummaryContextSize)
	c.SummaryLengthRecommended = getEnvInt("SUMMARY_LENGTH_RECOMMENDED", c.SummaryLengthRecommended)
	c.MaxParallelInsert = getEnvInt("MAX_PARALLEL_INSERT", c.MaxParallelInsert)
	c.MaxGraphNodes = getEnvInt("MAX_GRAPH_NODES", c.MaxGraphNodes)

	// Logging configuration
	c.LogLevel = getEnvString("LOG_LEVEL", c.LogLevel)
	c.Verbose = getEnvBool("VERBOSE", c.Verbose)

	// API Key
	c.APIKey = getEnvString("LIGHTRAG_API_KEY", c.APIKey)

	// SSL configuration
	c.SSL = getEnvBool("SSL", c.SSL)
	c.SSLCertFile = getEnvString("SSL_CERTFILE", c.SSLCertFile)
	c.SSLKeyFile = getEnvString("SSL_KEYFILE", c.SSLKeyFile)

	// Ollama model configuration
	c.SimulatedModelName = getEnvString("OLLAMA_EMULATING_MODEL_NAME", c.SimulatedModelName)
	c.SimulatedModelTag = getEnvString("OLLAMA_EMULATING_MODEL_TAG", c.SimulatedModelTag)

	// Workspace
	c.Workspace = getEnvString("WORKSPACE", c.Workspace)

	// LLM and embedding bindings
	c.LLMBinding = getEnvString("LLM_BINDING", c.LLMBinding)
	c.EmbeddingBinding = getEnvString("EMBEDDING_BINDING", c.EmbeddingBinding)
	c.RerankBinding = getEnvString("RERANK_BINDING", c.RerankBinding)

	// Binding hosts
	c.LLMBindingHost = getEnvString("LLM_BINDING_HOST", getDefaultHost(c.LLMBinding))
	c.EmbeddingBindingHost = getEnvString("EMBEDDING_BINDING_HOST", getDefaultHost(c.EmbeddingBinding))
	c.LLMBindingAPIKey = getEnvString("LLM_BINDING_API_KEY", c.LLMBindingAPIKey)
	c.EmbeddingBindingAPIKey = getEnvString("EMBEDDING_BINDING_API_KEY", c.EmbeddingBindingAPIKey)

	// Model configuration
	c.LLMModel = getEnvString("LLM_MODEL", c.LLMModel)
	c.EmbeddingModel = getEnvString("EMBEDDING_MODEL", c.EmbeddingModel)
	if dimStr := os.Getenv("EMBEDDING_DIM"); dimStr != "" && dimStr != "None" {
		if dim, err := strconv.Atoi(dimStr); err == nil {
			c.EmbeddingDim = &dim
		}
	}
	c.EmbeddingSendDim = getEnvBool("EMBEDDING_SEND_DIM", c.EmbeddingSendDim)

	// Chunk configuration
	c.ChunkSize = getEnvInt("CHUNK_SIZE", c.ChunkSize)
	c.ChunkOverlapSize = getEnvInt("CHUNK_OVERLAP_SIZE", c.ChunkOverlapSize)

	// LLM cache configuration
	c.EnableLLMCacheForExtract = getEnvBool("ENABLE_LLM_CACHE_FOR_EXTRACT", c.EnableLLMCacheForExtract)
	c.EnableLLMCache = getEnvBool("ENABLE_LLM_CACHE", c.EnableLLMCache)

	// Document loading engine
	c.DocumentLoadingEngine = getEnvString("DOCUMENT_LOADING_ENGINE", c.DocumentLoadingEngine)
	c.PDFDecryptPassword = getEnvString("PDF_DECRYPT_PASSWORD", c.PDFDecryptPassword)

	// Query configuration
	c.HistoryTurns = getEnvInt("HISTORY_TURNS", c.HistoryTurns)
	c.TopK = getEnvInt("TOP_K", c.TopK)
	c.ChunkTopK = getEnvInt("CHUNK_TOP_K", c.ChunkTopK)
	c.MaxEntityTokens = getEnvInt("MAX_ENTITY_TOKENS", c.MaxEntityTokens)
	c.MaxRelationTokens = getEnvInt("MAX_RELATION_TOKENS", c.MaxRelationTokens)
	c.MaxTotalTokens = getEnvInt("MAX_TOTAL_TOKENS", c.MaxTotalTokens)
	c.CosineThreshold = getEnvFloat("COSINE_THRESHOLD", c.CosineThreshold)
	c.RelatedChunkNumber = getEnvInt("RELATED_CHUNK_NUMBER", c.RelatedChunkNumber)

	// Rerank configuration
	c.RerankModel = getEnvString("RERANK_MODEL", c.RerankModel)
	c.RerankBindingHost = getEnvString("RERANK_BINDING_HOST", c.RerankBindingHost)
	c.RerankBindingAPIKey = getEnvString("RERANK_BINDING_API_KEY", c.RerankBindingAPIKey)
	c.MinRerankScore = getEnvFloat("MIN_RERANK_SCORE", c.MinRerankScore)

	// Storage configuration
	c.KVStorage = getEnvString("LIGHTRAG_KV_STORAGE", c.KVStorage)
	c.DocStatusStorage = getEnvString("LIGHTRAG_DOC_STATUS_STORAGE", c.DocStatusStorage)
	c.GraphStorage = getEnvString("LIGHTRAG_GRAPH_STORAGE", c.GraphStorage)
	c.VectorStorage = getEnvString("LIGHTRAG_VECTOR_STORAGE", c.VectorStorage)

	// CORS configuration
	c.CORSOrigins = getEnvString("CORS_ORIGINS", c.CORSOrigins)
	if origins := os.Getenv("CORS_ALLOWED_ORIGINS"); origins != "" {
		c.CORSAllowedOrigins = strings.Split(origins, ",")
	}

	// Language configuration
	c.SummaryLanguage = getEnvString("SUMMARY_LANGUAGE", c.SummaryLanguage)
	if entityTypes := os.Getenv("ENTITY_TYPES"); entityTypes != "" {
		c.EntityTypes = parseEntityTypes(entityTypes)
	}

	// Auth configuration
	c.WhitelistPaths = getEnvString("WHITELIST_PATHS", c.WhitelistPaths)
	c.AuthAccounts = getEnvString("AUTH_ACCOUNTS", c.AuthAccounts)
	c.AuthUsername = getEnvString("LIGHTRAG_API_AUTH_USERNAME", c.AuthUsername)
	c.AuthPassword = getEnvString("LIGHTRAG_API_AUTH_PASSWORD", c.AuthPassword)
	c.AuthDisabled = getEnvBool("AUTH_DISABLED", c.AuthDisabled)
	c.TokenSecret = getEnvString("TOKEN_SECRET", c.TokenSecret)
	c.TokenExpireHours = getEnvFloat("TOKEN_EXPIRE_HOURS", c.TokenExpireHours)
	c.GuestTokenExpireHours = getEnvFloat("GUEST_TOKEN_EXPIRE_HOURS", c.GuestTokenExpireHours)
	c.JWTAlgorithm = getEnvString("JWT_ALGORITHM", c.JWTAlgorithm)
	c.TokenAutoRenew = getEnvBool("TOKEN_AUTO_RENEW", c.TokenAutoRenew)
	c.TokenRenewThreshold = getEnvFloat("TOKEN_RENEW_THRESHOLD", c.TokenRenewThreshold)

	// Web UI configuration
	c.EnableWebUI = getEnvBool("ENABLE_WEBUI", c.EnableWebUI)

	// Embedding configuration
	c.ForceLLMSummaryOnMerge = getEnvInt("FORCE_LLM_SUMMARY_ON_MERGE", c.ForceLLMSummaryOnMerge)
	c.EmbeddingFuncMaxAsync = getEnvInt("EMBEDDING_FUNC_MAX_ASYNC", c.EmbeddingFuncMaxAsync)
	c.EmbeddingBatchNum = getEnvInt("EMBEDDING_BATCH_NUM", c.EmbeddingBatchNum)
	if tokenLimit := os.Getenv("EMBEDDING_TOKEN_LIMIT"); tokenLimit != "" && tokenLimit != "None" {
		if limit, err := strconv.Atoi(tokenLimit); err == nil {
			c.EmbeddingTokenLimit = &limit
		}
	}

	// File upload configuration
	if maxUpload := os.Getenv("MAX_UPLOAD_SIZE"); maxUpload != "" && maxUpload != "None" {
		if size, err := strconv.ParseInt(maxUpload, 10, 64); err == nil {
			c.MaxUploadSize = &size
		}
	}
}

// getDefaultHost returns the default host for a binding type.
func getDefaultHost(bindingType string) string {
	defaultHosts := map[string]string{
		"ollama":       "http://localhost:11434",
		"lollms":       "http://localhost:9600",
		"azure_openai": "https://api.openai.com/v1",
		"openai":       "https://api.openai.com/v1",
		"gemini":       "https://generativelanguage.googleapis.com",
	}
	if host, ok := defaultHosts[bindingType]; ok {
		return host
	}
	return "http://localhost:11434"
}

// Helper functions for environment variable parsing
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.ToLower(value) == "true" || value == "1"
	}
	return defaultValue
}

func parseEntityTypes(s string) []string {
	// Try to parse as JSON array first
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = s[1 : len(s)-1]
	}

	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, `"'`)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// OllamaServerInfos holds Ollama server configuration.
type OllamaServerInfos struct {
	LightRAGName      string
	LightRAGTag       string
	LightRAGSize      int64
	LightRAGCreatedAt string
	LightRAGDigest    string
}

// NewOllamaServerInfos creates a new OllamaServerInfos with defaults.
func NewOllamaServerInfos(name, tag string) *OllamaServerInfos {
	if name == "" {
		name = getEnvString("OLLAMA_EMULATING_MODEL_NAME", constants.DefaultOllamaModelName)
	}
	if tag == "" {
		tag = getEnvString("OLLAMA_EMULATING_MODEL_TAG", constants.DefaultOllamaModelTag)
	}
	return &OllamaServerInfos{
		LightRAGName:      name,
		LightRAGTag:       tag,
		LightRAGSize:      constants.DefaultOllamaModelSize,
		LightRAGCreatedAt: constants.DefaultOllamaCreatedAt,
		LightRAGDigest:    constants.DefaultOllamaDigest,
	}
}

// LightRAGModel returns the full model name with tag.
func (o *OllamaServerInfos) LightRAGModel() string {
	return o.LightRAGName + ":" + o.LightRAGTag
}

// GlobalConfig is the global configuration instance.
var GlobalConfig *Config

// LoadFromEnv creates a new Config and loads values from environment.
// This is a convenience function that returns a new config instead of modifying an existing one.
func LoadFromEnv() (*Config, error) {
	cfg := DefaultConfig()
	cfg.LoadFromEnv()
	return cfg, nil
}

// Init initializes the global configuration.
func Init() {
	GlobalConfig = DefaultConfig()
	GlobalConfig.LoadFromEnv()
}
