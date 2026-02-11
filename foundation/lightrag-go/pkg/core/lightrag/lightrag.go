// Package lightrag provides the main LightRAG orchestrator for RAG operations.
// This is a direct translation of Python lightrag/lightrag.py to Go.
package lightrag

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lightrag/lightrag-go/pkg/core/base"
	"github.com/lightrag/lightrag-go/pkg/core/coreconstants"
	"github.com/lightrag/lightrag-go/pkg/core/namespace"
	"github.com/lightrag/lightrag-go/pkg/core/operate"
	"github.com/lightrag/lightrag-go/pkg/core/prompt"
	"github.com/lightrag/lightrag-go/pkg/core/utils"
)

// StoragesStatus represents the current state of storage backends
type StoragesStatus int

const (
	// StoragesNotCreated indicates storages have not been created yet
	StoragesNotCreated StoragesStatus = iota
	// StoragesCreated indicates storages have been created but not initialized
	StoragesCreated
	// StoragesInitialized indicates storages are fully initialized and ready
	StoragesInitialized
	// StoragesFinalized indicates storages have been properly closed
	StoragesFinalized
)

// String returns a string representation of StoragesStatus
func (s StoragesStatus) String() string {
	switch s {
	case StoragesNotCreated:
		return "NOT_CREATED"
	case StoragesCreated:
		return "CREATED"
	case StoragesInitialized:
		return "INITIALIZED"
	case StoragesFinalized:
		return "FINALIZED"
	default:
		return "UNKNOWN"
	}
}

// Config holds all configuration for LightRAG
type Config struct {
	// Directory Configuration
	WorkingDir string `json:"working_dir"`

	// Storage Configuration
	KVStorageType        string `json:"kv_storage"`
	VectorStorageType    string `json:"vector_storage"`
	GraphStorageType     string `json:"graph_storage"`
	DocStatusStorageType string `json:"doc_status_storage"`

	// Workspace
	Workspace string `json:"workspace"`

	// Query Parameters
	TopK               int     `json:"top_k"`
	ChunkTopK          int     `json:"chunk_top_k"`
	MaxEntityTokens    int     `json:"max_entity_tokens"`
	MaxRelationTokens  int     `json:"max_relation_tokens"`
	MaxTotalTokens     int     `json:"max_total_tokens"`
	CosineThreshold    float64 `json:"cosine_threshold"`
	RelatedChunkNumber int     `json:"related_chunk_number"`
	KGChunkPickMethod  string  `json:"kg_chunk_pick_method"`

	// Entity Extraction
	EntityExtractMaxGleaning int `json:"entity_extract_max_gleaning"`
	MaxExtractInputTokens    int `json:"max_extract_input_tokens"`
	ForceLLMSummaryOnMerge   int `json:"force_llm_summary_on_merge"`

	// Text Chunking
	ChunkTokenSize        int                      `json:"chunk_token_size"`
	ChunkOverlapTokenSize int                      `json:"chunk_overlap_token_size"`
	Tokenizer             utils.TokenizerInterface `json:"-"`
	TiktokenModelName     string                   `json:"tiktoken_model_name"`

	// Embedding Configuration
	EmbeddingFunc           *base.EmbeddingFunc `json:"-"`
	EmbeddingBatchNum       int                 `json:"embedding_batch_num"`
	EmbeddingFuncMaxAsync   int                 `json:"embedding_func_max_async"`
	DefaultEmbeddingTimeout int                 `json:"default_embedding_timeout"`

	// LLM Configuration
	LLMModelFunc             operate.LLMFunc        `json:"-"`
	LLMModelName             string                 `json:"llm_model_name"`
	SummaryMaxTokens         int                    `json:"summary_max_tokens"`
	SummaryContextSize       int                    `json:"summary_context_size"`
	SummaryLengthRecommended int                    `json:"summary_length_recommended"`
	LLMModelMaxAsync         int                    `json:"llm_model_max_async"`
	LLMModelKwargs           map[string]interface{} `json:"llm_model_kwargs"`
	DefaultLLMTimeout        int                    `json:"default_llm_timeout"`

	// Rerank Configuration
	RerankModelFunc *func(ctx context.Context, query string, docs []string) ([]float64, error) `json:"-"`
	MinRerankScore  float64                                                                    `json:"min_rerank_score"`

	// Storage Configuration
	VectorDBStorageClsKwargs map[string]interface{} `json:"vector_db_storage_cls_kwargs"`
	EnableLLMCache           bool                   `json:"enable_llm_cache"`
	EnableLLMCacheForExtract bool                   `json:"enable_llm_cache_for_entity_extract"`

	// Extensions
	MaxParallelInsert       int    `json:"max_parallel_insert"`
	MaxGraphNodes           int    `json:"max_graph_nodes"`
	MaxSourceIDsPerEntity   int    `json:"max_source_ids_per_entity"`
	MaxSourceIDsPerRelation int    `json:"max_source_ids_per_relation"`
	SourceIDsLimitMethod    string `json:"source_ids_limit_method"`
	MaxFilePaths            int    `json:"max_file_paths"`
	FilePathMorePlaceholder string `json:"file_path_more_placeholder"`

	// Additional Parameters
	AddonParams map[string]interface{} `json:"addon_params"`

	// Cosine threshold for vector search
	CosineBetterThanThreshold float64 `json:"cosine_better_than_threshold"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		WorkingDir:               "./rag_storage",
		KVStorageType:            "JsonKVStorage",
		VectorStorageType:        "NanoVectorDBStorage",
		GraphStorageType:         "NetworkXStorage",
		DocStatusStorageType:     "JsonDocStatusStorage",
		Workspace:                utils.GetEnvString("WORKSPACE", ""),
		TopK:                     utils.GetEnvInt("TOP_K", coreconstants.DefaultTopK),
		ChunkTopK:                utils.GetEnvInt("CHUNK_TOP_K", coreconstants.DefaultChunkTopK),
		MaxEntityTokens:          utils.GetEnvInt("MAX_ENTITY_TOKENS", coreconstants.DefaultMaxEntityTokens),
		MaxRelationTokens:        utils.GetEnvInt("MAX_RELATION_TOKENS", coreconstants.DefaultMaxRelationTokens),
		MaxTotalTokens:           utils.GetEnvInt("MAX_TOTAL_TOKENS", coreconstants.DefaultMaxTotalTokens),
		CosineThreshold:          utils.GetEnvFloat("COSINE_THRESHOLD", coreconstants.DefaultCosineThreshold),
		RelatedChunkNumber:       utils.GetEnvInt("RELATED_CHUNK_NUMBER", coreconstants.DefaultRelatedChunkNumber),
		KGChunkPickMethod:        utils.GetEnvString("KG_CHUNK_PICK_METHOD", coreconstants.DefaultKGChunkPickMethod),
		EntityExtractMaxGleaning: utils.GetEnvInt("MAX_GLEANING", coreconstants.DefaultMaxGleaning),
		MaxExtractInputTokens:    utils.GetEnvInt("MAX_EXTRACT_INPUT_TOKENS", coreconstants.DefaultMaxExtractInputTokens),
		ForceLLMSummaryOnMerge:   utils.GetEnvInt("FORCE_LLM_SUMMARY_ON_MERGE", coreconstants.DefaultForceLLMSummaryOnMerge),
		ChunkTokenSize:           utils.GetEnvInt("CHUNK_SIZE", coreconstants.DefaultChunkSize),
		ChunkOverlapTokenSize:    utils.GetEnvInt("CHUNK_OVERLAP_SIZE", coreconstants.DefaultChunkOverlapSize),
		TiktokenModelName:        "gpt-4o-mini",
		EmbeddingBatchNum:        utils.GetEnvInt("EMBEDDING_BATCH_NUM", 10),
		EmbeddingFuncMaxAsync:    utils.GetEnvInt("EMBEDDING_FUNC_MAX_ASYNC", 8),
		DefaultEmbeddingTimeout:  30, // seconds
		LLMModelName:             "gpt-4o-mini",
		SummaryMaxTokens:         utils.GetEnvInt("SUMMARY_MAX_TOKENS", coreconstants.DefaultSummaryMaxTokens),
		SummaryContextSize:       utils.GetEnvInt("SUMMARY_CONTEXT_SIZE", coreconstants.DefaultSummaryContextSize),
		SummaryLengthRecommended: utils.GetEnvInt("SUMMARY_LENGTH_RECOMMENDED", coreconstants.DefaultSummaryLengthRecommended),
		LLMModelMaxAsync:         utils.GetEnvInt("MAX_ASYNC", coreconstants.DefaultMaxAsync),
		LLMModelKwargs:           make(map[string]interface{}),
		DefaultLLMTimeout:        180, // seconds
		MinRerankScore:           utils.GetEnvFloat("MIN_RERANK_SCORE", coreconstants.DefaultMinRerankScore),
		VectorDBStorageClsKwargs: make(map[string]interface{}),
		EnableLLMCache:           true,
		EnableLLMCacheForExtract: true,
		MaxParallelInsert:        utils.GetEnvInt("MAX_PARALLEL_INSERT", coreconstants.DefaultMaxParallelInsert),
		MaxGraphNodes:            utils.GetEnvInt("MAX_GRAPH_NODES", coreconstants.DefaultMaxGraphNodes),
		MaxSourceIDsPerEntity:    utils.GetEnvInt("MAX_SOURCE_IDS_PER_ENTITY", coreconstants.DefaultMaxSourceIDsPerEntity),
		MaxSourceIDsPerRelation:  utils.GetEnvInt("MAX_SOURCE_IDS_PER_RELATION", coreconstants.DefaultMaxSourceIDsPerRelation),
		SourceIDsLimitMethod:     coreconstants.NormalizeSourceIDsLimitMethod(utils.GetEnvString("SOURCE_IDS_LIMIT_METHOD", coreconstants.DefaultSourceIDsLimitMethod)),
		MaxFilePaths:             utils.GetEnvInt("MAX_FILE_PATHS", coreconstants.DefaultMaxFilePaths),
		FilePathMorePlaceholder:  coreconstants.DefaultFilePathMorePlaceholder,
		AddonParams: map[string]interface{}{
			"language":     utils.GetEnvString("SUMMARY_LANGUAGE", coreconstants.DefaultSummaryLanguage),
			"entity_types": coreconstants.DefaultEntityTypes,
		},
		CosineBetterThanThreshold: utils.GetEnvFloat("COSINE_THRESHOLD", 0.2),
	}
}

// LightRAG is the main orchestrator for RAG operations
type LightRAG struct {
	config *Config
	mu     sync.RWMutex

	// Storage backends
	fullDocs            base.BaseKVStorage
	textChunks          base.BaseKVStorage
	fullEntities        base.BaseKVStorage
	fullRelations       base.BaseKVStorage
	entityChunks        base.BaseKVStorage
	relationChunks      base.BaseKVStorage
	entitiesVDB         base.BaseVectorStorage
	relationshipsVDB    base.BaseVectorStorage
	chunksVDB           base.BaseVectorStorage
	chunkEntityRelGraph base.BaseGraphStorage
	llmResponseCache    base.BaseKVStorage
	docStatus           base.BaseKVStorage

	// Status
	storagesStatus StoragesStatus

	// Global configuration for operations
	globalConfig *operate.GlobalConfig
}

// New creates a new LightRAG instance with the given configuration
func New(config *Config) (*LightRAG, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Validate required functions
	if config.LLMModelFunc == nil {
		return nil, errors.New("LLMModelFunc is required")
	}
	if config.EmbeddingFunc == nil {
		return nil, errors.New("EmbeddingFunc is required")
	}

	// Create working directory if it doesn't exist
	if err := os.MkdirAll(config.WorkingDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}

	// Initialize tokenizer if not provided
	if config.Tokenizer == nil {
		config.Tokenizer = utils.NewSimpleTokenizer()
	}

	// Validate configuration
	if config.ForceLLMSummaryOnMerge < 3 {
		utils.Logger.Warnf("force_llm_summary_on_merge should be at least 3, got %d", config.ForceLLMSummaryOnMerge)
	}
	if config.SummaryContextSize > config.MaxTotalTokens {
		utils.Logger.Warnf("summary_context_size(%d) should not be greater than max_total_tokens(%d)",
			config.SummaryContextSize, config.MaxTotalTokens)
	}
	if config.SummaryLengthRecommended > config.SummaryMaxTokens {
		utils.Logger.Warnf("summary_length_recommended(%d) should not be greater than summary_max_tokens(%d)",
			config.SummaryLengthRecommended, config.SummaryMaxTokens)
	}

	// Ensure vector_db_storage_cls_kwargs has required fields
	if config.VectorDBStorageClsKwargs == nil {
		config.VectorDBStorageClsKwargs = make(map[string]interface{})
	}
	config.VectorDBStorageClsKwargs["cosine_better_than_threshold"] = config.CosineBetterThanThreshold

	rag := &LightRAG{
		config:         config,
		storagesStatus: StoragesNotCreated,
	}

	// Create global config for operations
	rag.globalConfig = &operate.GlobalConfig{
		Workspace:                      config.Workspace,
		Tokenizer:                      config.Tokenizer,
		LLMModelFunc:                   config.LLMModelFunc,
		SummaryMaxTokens:               config.SummaryMaxTokens,
		SummaryContextSize:             config.SummaryContextSize,
		SummaryLengthRecommended:       config.SummaryLengthRecommended,
		ForceLLMSummaryOnMerge:         config.ForceLLMSummaryOnMerge,
		MaxSourceIDsPerEntity:          config.MaxSourceIDsPerEntity,
		MaxSourceIDsPerRelation:        config.MaxSourceIDsPerRelation,
		SourceIDsLimitMethod:           config.SourceIDsLimitMethod,
		MaxFilePaths:                   config.MaxFilePaths,
		FilePathMorePlaceholder:        config.FilePathMorePlaceholder,
		EnableLLMCache:                 config.EnableLLMCache,
		EnableLLMCacheForEntityExtract: config.EnableLLMCacheForExtract,
		AddonParams:                    config.AddonParams,
	}

	rag.storagesStatus = StoragesCreated
	utils.Logger.Info("LightRAG instance created")

	return rag, nil
}

// SetStorages sets the storage backends for the LightRAG instance
func (r *LightRAG) SetStorages(
	fullDocs, textChunks, fullEntities, fullRelations base.BaseKVStorage,
	entityChunks, relationChunks base.BaseKVStorage,
	entitiesVDB, relationshipsVDB, chunksVDB base.BaseVectorStorage,
	chunkEntityRelGraph base.BaseGraphStorage,
	llmResponseCache, docStatus base.BaseKVStorage,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.fullDocs = fullDocs
	r.textChunks = textChunks
	r.fullEntities = fullEntities
	r.fullRelations = fullRelations
	r.entityChunks = entityChunks
	r.relationChunks = relationChunks
	r.entitiesVDB = entitiesVDB
	r.relationshipsVDB = relationshipsVDB
	r.chunksVDB = chunksVDB
	r.chunkEntityRelGraph = chunkEntityRelGraph
	r.llmResponseCache = llmResponseCache
	r.docStatus = docStatus
}

// InitializeStorages initializes all storage backends
func (r *LightRAG) InitializeStorages(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.storagesStatus != StoragesCreated {
		return fmt.Errorf("storages already initialized or not created, status: %s", r.storagesStatus)
	}

	storages := []struct {
		name    string
		storage interface {
			Initialize(ctx context.Context) error
		}
	}{
		{"fullDocs", r.fullDocs},
		{"textChunks", r.textChunks},
		{"fullEntities", r.fullEntities},
		{"fullRelations", r.fullRelations},
		{"entityChunks", r.entityChunks},
		{"relationChunks", r.relationChunks},
		{"entitiesVDB", r.entitiesVDB},
		{"relationshipsVDB", r.relationshipsVDB},
		{"chunksVDB", r.chunksVDB},
		{"chunkEntityRelGraph", r.chunkEntityRelGraph},
		{"llmResponseCache", r.llmResponseCache},
		{"docStatus", r.docStatus},
	}

	for _, s := range storages {
		if s.storage != nil {
			if err := s.storage.Initialize(ctx); err != nil {
				return fmt.Errorf("failed to initialize %s: %w", s.name, err)
			}
			utils.Logger.Debugf("Initialized storage: %s", s.name)
		}
	}

	r.storagesStatus = StoragesInitialized
	utils.Logger.Info("All storages initialized")

	return nil
}

// FinalizeStorages properly closes all storage backends
func (r *LightRAG) FinalizeStorages(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.storagesStatus != StoragesInitialized {
		return fmt.Errorf("storages not initialized, status: %s", r.storagesStatus)
	}

	storages := []struct {
		name    string
		storage interface {
			Finalize(ctx context.Context) error
		}
	}{
		{"fullDocs", r.fullDocs},
		{"textChunks", r.textChunks},
		{"fullEntities", r.fullEntities},
		{"fullRelations", r.fullRelations},
		{"entityChunks", r.entityChunks},
		{"relationChunks", r.relationChunks},
		{"entitiesVDB", r.entitiesVDB},
		{"relationshipsVDB", r.relationshipsVDB},
		{"chunksVDB", r.chunksVDB},
		{"chunkEntityRelGraph", r.chunkEntityRelGraph},
		{"llmResponseCache", r.llmResponseCache},
		{"docStatus", r.docStatus},
	}

	var errs []error
	for _, s := range storages {
		if s.storage != nil {
			if err := s.storage.Finalize(ctx); err != nil {
				utils.Logger.Errorf("Failed to finalize %s: %v", s.name, err)
				errs = append(errs, fmt.Errorf("%s: %w", s.name, err))
			} else {
				utils.Logger.Debugf("Finalized storage: %s", s.name)
			}
		}
	}

	r.storagesStatus = StoragesFinalized

	if len(errs) > 0 {
		return fmt.Errorf("failed to finalize some storages: %v", errs)
	}

	utils.Logger.Info("All storages finalized")
	return nil
}

// Insert inserts a document into the knowledge base
func (r *LightRAG) Insert(ctx context.Context, content string, opts ...InsertOption) error {
	r.mu.RLock()
	if r.storagesStatus != StoragesInitialized {
		r.mu.RUnlock()
		return fmt.Errorf("storages not initialized, status: %s", r.storagesStatus)
	}
	r.mu.RUnlock()

	options := DefaultInsertOptions()
	for _, opt := range opts {
		opt(options)
	}

	// Generate document ID
	docID := utils.ComputeMDHashID("doc", content)

	// Check if document already exists
	if options.CheckDuplicate {
		existing, err := r.fullDocs.GetByID(ctx, docID)
		if err == nil && existing != nil {
			utils.Logger.Infof("Document %s already exists, skipping", docID)
			return nil
		}
	}

	// Prepare split by character pointer
	var splitByChar *string
	if options.SplitByCharacter != "" {
		splitByChar = &options.SplitByCharacter
	}

	// Chunk the content
	chunks, err := operate.ChunkingByTokenSize(
		r.config.Tokenizer,
		content,
		splitByChar,
		options.SplitByCharacterOnly,
		r.config.ChunkOverlapTokenSize,
		r.config.ChunkTokenSize,
	)
	if err != nil {
		return fmt.Errorf("failed to chunk content: %w", err)
	}

	if len(chunks) == 0 {
		return errors.New("no chunks generated from content")
	}

	timestamp := time.Now().Unix()

	// Store the full document
	docData := map[string]interface{}{
		"content":   content,
		"timestamp": timestamp,
		"chunk_ids": make([]string, len(chunks)),
		"file_path": options.FilePath,
	}
	if err := r.fullDocs.Upsert(ctx, map[string]map[string]interface{}{docID: docData}); err != nil {
		return fmt.Errorf("failed to store document: %w", err)
	}

	// Process each chunk
	for i, chunk := range chunks {
		chunkID := utils.ComputeMDHashID("chunk", chunk.Content)
		docData["chunk_ids"].([]string)[i] = chunkID

		// Store chunk
		chunkData := map[string]interface{}{
			"content":     chunk.Content,
			"tokens":      chunk.Tokens,
			"chunk_order": chunk.ChunkOrderIndex,
			"doc_id":      docID,
			"timestamp":   timestamp,
			"file_path":   options.FilePath,
		}
		if err := r.textChunks.Upsert(ctx, map[string]map[string]interface{}{chunkID: chunkData}); err != nil {
			return fmt.Errorf("failed to store chunk %s: %w", chunkID, err)
		}

		// Generate chunk embedding and store in vector DB
		if r.config.EmbeddingFunc != nil {
			embeddings, err := r.config.EmbeddingFunc.Embed(ctx, []string{chunk.Content})
			if err != nil {
				return fmt.Errorf("failed to generate embedding for chunk %s: %w", chunkID, err)
			}
			if len(embeddings) > 0 {
				vectorData := map[string]interface{}{
					"id":        chunkID,
					"content":   chunk.Content,
					"embedding": embeddings[0],
					"doc_id":    docID,
				}
				if err := r.chunksVDB.Upsert(ctx, map[string]map[string]interface{}{chunkID: vectorData}); err != nil {
					return fmt.Errorf("failed to store chunk embedding %s: %w", chunkID, err)
				}
			}
		}

		// Extract entities and relationships
		if options.ExtractEntities {
			if err := r.extractAndStoreEntities(ctx, chunk.Content, chunkID, timestamp, options.FilePath); err != nil {
				utils.Logger.Warnf("Failed to extract entities from chunk %s: %v", chunkID, err)
			}
		}
	}

	// Update document status
	if err := r.docStatus.Upsert(ctx, map[string]map[string]interface{}{docID: {
		"status":      "processed",
		"timestamp":   timestamp,
		"chunk_count": len(chunks),
	}}); err != nil {
		utils.Logger.Warnf("Failed to update document status: %v", err)
	}

	utils.Logger.Infof("Inserted document %s with %d chunks", docID, len(chunks))
	return nil
}

// extractAndStoreEntities extracts entities and relationships from content and stores them
func (r *LightRAG) extractAndStoreEntities(ctx context.Context, content, chunkID string, timestamp int64, filePath string) error {
	// Build extraction prompt
	entityTypes := coreconstants.DefaultEntityTypes
	if types, ok := r.config.AddonParams["entity_types"].([]string); ok {
		entityTypes = types
	}

	tupleDelimiter := prompt.DefaultTupleDelimiter
	completionDelimiter := prompt.DefaultCompletionDelimiter

	// Call LLM for entity extraction
	// Note: This is a simplified version - full implementation would use the extraction prompts
	extractionResult, err := r.config.LLMModelFunc(ctx, content, operate.WithPriority(5))
	if err != nil {
		return fmt.Errorf("LLM extraction failed: %w", err)
	}

	_ = entityTypes // Used in full extraction prompt

	// Process extraction result
	result, err := operate.ProcessExtractionResult(
		extractionResult,
		chunkID,
		timestamp,
		filePath,
		tupleDelimiter,
		completionDelimiter,
	)
	if err != nil {
		return fmt.Errorf("failed to process extraction result: %w", err)
	}

	// Store entities
	for entityName, entities := range result.Entities {
		for _, entity := range entities {
			entityID := utils.ComputeMDHashID("ent", entityName)
			entityData := map[string]interface{}{
				"entity_name": entity.EntityName,
				"entity_type": entity.EntityType,
				"description": entity.Description,
				"source_id":   entity.SourceID,
				"file_path":   entity.FilePath,
				"timestamp":   entity.Timestamp,
			}
			if err := r.fullEntities.Upsert(ctx, map[string]map[string]interface{}{entityID: entityData}); err != nil {
				utils.Logger.Warnf("Failed to store entity %s: %v", entityName, err)
			}

			// Store in vector DB
			if r.config.EmbeddingFunc != nil {
				embeddings, err := r.config.EmbeddingFunc.Embed(ctx, []string{entity.Description})
				if err == nil && len(embeddings) > 0 {
					vectorData := map[string]interface{}{
						"id":          entityID,
						"entity_name": entity.EntityName,
						"description": entity.Description,
						"embedding":   embeddings[0],
					}
					if err := r.entitiesVDB.Upsert(ctx, map[string]map[string]interface{}{entityID: vectorData}); err != nil {
						utils.Logger.Warnf("Failed to store entity embedding %s: %v", entityName, err)
					}
				}
			}

			// Store entity-chunk relationship
			entityChunkID := fmt.Sprintf("%s||%s", entityID, chunkID)
			if err := r.entityChunks.Upsert(ctx, map[string]map[string]interface{}{entityChunkID: {
				"entity_id": entityID,
				"chunk_id":  chunkID,
			}}); err != nil {
				utils.Logger.Warnf("Failed to store entity-chunk relation: %v", err)
			}
		}
	}

	// Store relationships
	for relKey, relationships := range result.Relationships {
		for _, rel := range relationships {
			relID := utils.ComputeMDHashID("rel", relKey)
			relData := map[string]interface{}{
				"src_id":      rel.SrcID,
				"tgt_id":      rel.TgtID,
				"description": rel.Description,
				"keywords":    rel.Keywords,
				"weight":      rel.Weight,
				"source_id":   rel.SourceID,
				"file_path":   rel.FilePath,
				"timestamp":   rel.Timestamp,
			}
			if err := r.fullRelations.Upsert(ctx, map[string]map[string]interface{}{relID: relData}); err != nil {
				utils.Logger.Warnf("Failed to store relationship %s: %v", relKey, err)
			}

			// Store in vector DB
			if r.config.EmbeddingFunc != nil {
				embeddings, err := r.config.EmbeddingFunc.Embed(ctx, []string{rel.Description})
				if err == nil && len(embeddings) > 0 {
					vectorData := map[string]interface{}{
						"id":          relID,
						"src_id":      rel.SrcID,
						"tgt_id":      rel.TgtID,
						"description": rel.Description,
						"embedding":   embeddings[0],
					}
					if err := r.relationshipsVDB.Upsert(ctx, map[string]map[string]interface{}{relID: vectorData}); err != nil {
						utils.Logger.Warnf("Failed to store relationship embedding %s: %v", relKey, err)
					}
				}
			}

			// Store relationship-chunk mapping
			relChunkID := fmt.Sprintf("%s||%s", relID, chunkID)
			if err := r.relationChunks.Upsert(ctx, map[string]map[string]interface{}{relChunkID: {
				"relation_id": relID,
				"chunk_id":    chunkID,
			}}); err != nil {
				utils.Logger.Warnf("Failed to store relation-chunk mapping: %v", err)
			}
		}
	}

	return nil
}

// Query performs a query against the knowledge base
func (r *LightRAG) Query(ctx context.Context, query string, param *base.QueryParam) (*base.QueryResult, error) {
	r.mu.RLock()
	if r.storagesStatus != StoragesInitialized {
		r.mu.RUnlock()
		return nil, fmt.Errorf("storages not initialized, status: %s", r.storagesStatus)
	}
	r.mu.RUnlock()

	if param == nil {
		param = base.NewQueryParam()
	}

	// Create query context
	queryContext := &operate.QueryContext{
		GlobalConfig:          r.globalConfig,
		KnowledgeGraph:        r.chunkEntityRelGraph,
		EntitiesVDB:           r.entitiesVDB,
		RelationshipsVDB:      r.relationshipsVDB,
		ChunksVDB:             r.chunksVDB,
		TextChunksStorage:     r.textChunks,
		LLMResponseCache:      r.llmResponseCache,
		EntityChunksStorage:   r.entityChunks,
		RelationChunksStorage: r.relationChunks,
		EmbeddingFunc:         r.config.EmbeddingFunc,
	}

	// Perform query based on mode
	switch param.Mode {
	case "naive", "vector":
		return operate.NaiveQuery(ctx, query, param, queryContext)
	case "local", "global", "hybrid", "":
		return operate.KGQuery(ctx, query, param, queryContext)
	default:
		return nil, fmt.Errorf("unknown query mode: %s", param.Mode)
	}
}

// Delete removes a document and its associated data from the knowledge base
func (r *LightRAG) Delete(ctx context.Context, docID string) error {
	r.mu.RLock()
	if r.storagesStatus != StoragesInitialized {
		r.mu.RUnlock()
		return fmt.Errorf("storages not initialized, status: %s", r.storagesStatus)
	}
	r.mu.RUnlock()

	// Get document data to find associated chunks
	if r.fullDocs == nil {
		return fmt.Errorf("fullDocs storage not initialized")
	}
	docData, err := r.fullDocs.GetByID(ctx, docID)
	if err != nil {
		return fmt.Errorf("failed to get document %s: %w", docID, err)
	}
	if docData == nil {
		return fmt.Errorf("document %s not found", docID)
	}

	// Get chunk IDs
	var chunkIDs []string
	if ids, ok := docData["chunk_ids"].([]string); ok {
		chunkIDs = ids
	} else if ids, ok := docData["chunk_ids"].([]interface{}); ok {
		for _, id := range ids {
			if s, ok := id.(string); ok {
				chunkIDs = append(chunkIDs, s)
			}
		}
	}

	// Delete chunks and their associated data
	for _, chunkID := range chunkIDs {
		// Delete from text chunks
		if err := r.textChunks.Delete(ctx, []string{chunkID}); err != nil {
			utils.Logger.Warnf("Failed to delete chunk %s: %v", chunkID, err)
		}

		// Delete from chunks VDB
		if err := r.chunksVDB.Delete(ctx, []string{chunkID}); err != nil {
			utils.Logger.Warnf("Failed to delete chunk from VDB %s: %v", chunkID, err)
		}

		// Note: Entity and relationship cleanup would require more complex logic
		// to check if they're used by other documents
	}

	// Delete the document itself
	if err := r.fullDocs.Delete(ctx, []string{docID}); err != nil {
		return fmt.Errorf("failed to delete document %s: %w", docID, err)
	}

	// Update document status
	if err := r.docStatus.Delete(ctx, []string{docID}); err != nil {
		utils.Logger.Warnf("Failed to delete document status: %v", err)
	}

	utils.Logger.Infof("Deleted document %s and %d chunks", docID, len(chunkIDs))
	return nil
}

// GetStatus returns the current status of the LightRAG instance
func (r *LightRAG) GetStatus() StoragesStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.storagesStatus
}

// GetConfig returns the current configuration
func (r *LightRAG) GetConfig() *Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// GetWorkingDir returns the working directory path
func (r *LightRAG) GetWorkingDir() string {
	return r.config.WorkingDir
}

// GetWorkspace returns the workspace name
func (r *LightRAG) GetWorkspace() string {
	return r.config.Workspace
}

// GetFullStoragePath returns the full path for a storage file
func (r *LightRAG) GetFullStoragePath(filename string) string {
	if r.config.Workspace != "" {
		return filepath.Join(r.config.WorkingDir, r.config.Workspace, filename)
	}
	return filepath.Join(r.config.WorkingDir, filename)
}

// InsertOptions holds options for document insertion
type InsertOptions struct {
	FilePath             string
	SplitByCharacter     string
	SplitByCharacterOnly bool
	CheckDuplicate       bool
	ExtractEntities      bool
}

// InsertOption is a function that modifies InsertOptions
type InsertOption func(*InsertOptions)

// DefaultInsertOptions returns the default insert options
func DefaultInsertOptions() *InsertOptions {
	return &InsertOptions{
		CheckDuplicate:  true,
		ExtractEntities: true,
	}
}

// WithFilePath sets the file path for the document
func WithFilePath(path string) InsertOption {
	return func(o *InsertOptions) {
		o.FilePath = path
	}
}

// WithSplitByCharacter sets the character to split by
func WithSplitByCharacter(char string) InsertOption {
	return func(o *InsertOptions) {
		o.SplitByCharacter = char
	}
}

// WithSplitByCharacterOnly sets whether to split only by character
func WithSplitByCharacterOnly(only bool) InsertOption {
	return func(o *InsertOptions) {
		o.SplitByCharacterOnly = only
	}
}

// WithCheckDuplicate sets whether to check for duplicate documents
func WithCheckDuplicate(check bool) InsertOption {
	return func(o *InsertOptions) {
		o.CheckDuplicate = check
	}
}

// WithExtractEntities sets whether to extract entities
func WithExtractEntities(extract bool) InsertOption {
	return func(o *InsertOptions) {
		o.ExtractEntities = extract
	}
}

// RegisterFile saves a file to the working directory and records it in the doc status store.
// It does NOT read or parse the file content — suitable for any file type (PDF, images, etc.).
func (r *LightRAG) RegisterFile(ctx context.Context, filename string, data []byte) error {
	r.mu.RLock()
	if r.storagesStatus != StoragesInitialized {
		r.mu.RUnlock()
		return fmt.Errorf("storages not initialized, status: %s", r.storagesStatus)
	}
	r.mu.RUnlock()

	// Save file to the input directory inside working dir
	inputDir := filepath.Join(r.config.WorkingDir, "input")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		return fmt.Errorf("create input dir: %w", err)
	}

	dest := filepath.Join(inputDir, filepath.Base(filename))
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	// Register in doc status
	docID := utils.ComputeMDHashID("doc", filename)
	ts := time.Now().Format(time.RFC3339)

	if r.docStatus != nil {
		if err := r.docStatus.Upsert(ctx, map[string]map[string]interface{}{docID: {
			"status":          "pending",
			"file_path":       filename,
			"created_at":      ts,
			"updated_at":      ts,
			"content_length":  len(data),
			"content_summary": "",
		}}); err != nil {
			return fmt.Errorf("record doc status: %w", err)
		}
	}

	// Also register in fullDocs
	if r.fullDocs != nil {
		if err := r.fullDocs.Upsert(ctx, map[string]map[string]interface{}{docID: {
			"file_path": filename,
			"timestamp": time.Now().Unix(),
		}}); err != nil {
			return fmt.Errorf("record full doc: %w", err)
		}
	}

	utils.Logger.Infof("Registered file %s (id=%s, %d bytes)", filename, docID, len(data))
	return nil
}

// IndexResult reports per-document indexing outcomes.
type IndexResult struct {
	Processed int      `json:"processed"`
	Failed    int      `json:"failed"`
	Skipped   int      `json:"skipped"`
	Errors    []string `json:"errors,omitempty"`
}

// supportedTextExt returns true for file extensions we can read as text.
func supportedTextExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".txt", ".md", ".csv", ".json", ".html", ".htm", ".xml", ".yaml", ".yml", ".log", ".go", ".py", ".js", ".ts", ".rb", ".rs", ".java", ".c", ".cpp", ".h":
		return true
	}
	return false
}

// IndexPendingDocuments reads every document with status "pending", reads
// its file from disk, inserts the text content into the knowledge base,
// and updates the status to "processed" (or "failed").
func (r *LightRAG) IndexPendingDocuments(ctx context.Context) (*IndexResult, error) {
	r.mu.RLock()
	if r.storagesStatus != StoragesInitialized {
		r.mu.RUnlock()
		return nil, fmt.Errorf("storages not initialized, status: %s", r.storagesStatus)
	}
	r.mu.RUnlock()

	if r.docStatus == nil {
		return &IndexResult{}, nil
	}

	statuses, err := r.docStatus.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("read doc statuses: %w", err)
	}

	result := &IndexResult{}
	inputDir := filepath.Join(r.config.WorkingDir, "input")

	for docID, meta := range statuses {
		s, _ := meta["status"].(string)
		if s != "pending" {
			continue
		}

		fp, _ := meta["file_path"].(string)
		if fp == "" {
			result.Skipped++
			continue
		}

		// Update status to processing
		now := time.Now().Format(time.RFC3339)
		_ = r.docStatus.Upsert(ctx, map[string]map[string]interface{}{docID: {
			"status":     "processing",
			"updated_at": now,
		}})

		if !supportedTextExt(fp) {
			_ = r.docStatus.Upsert(ctx, map[string]map[string]interface{}{docID: {
				"status":     "failed",
				"updated_at": now,
				"error_msg":  fmt.Sprintf("unsupported file type: %s", filepath.Ext(fp)),
			}})
			result.Failed++
			result.Errors = append(result.Errors, fp+": unsupported file type")
			continue
		}

		content, err := os.ReadFile(filepath.Join(inputDir, filepath.Base(fp)))
		if err != nil {
			_ = r.docStatus.Upsert(ctx, map[string]map[string]interface{}{docID: {
				"status":     "failed",
				"updated_at": now,
				"error_msg":  err.Error(),
			}})
			result.Failed++
			result.Errors = append(result.Errors, fp+": "+err.Error())
			continue
		}

		if err := r.Insert(ctx, string(content), WithFilePath(fp), WithCheckDuplicate(false)); err != nil {
			_ = r.docStatus.Upsert(ctx, map[string]map[string]interface{}{docID: {
				"status":     "failed",
				"updated_at": now,
				"error_msg":  err.Error(),
			}})
			result.Failed++
			result.Errors = append(result.Errors, fp+": "+err.Error())
			continue
		}

		_ = r.docStatus.Upsert(ctx, map[string]map[string]interface{}{docID: {
			"status":     "processed",
			"updated_at": time.Now().Format(time.RFC3339),
		}})
		result.Processed++
	}

	utils.Logger.Infof("Indexing complete: %d processed, %d failed, %d skipped", result.Processed, result.Failed, result.Skipped)
	return result, nil
}

// ListDocuments returns all documents from the full docs store.
func (r *LightRAG) ListDocuments(ctx context.Context) (map[string]map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.fullDocs == nil {
		return map[string]map[string]interface{}{}, nil
	}
	return r.fullDocs.GetAll(ctx)
}

// GetDocStatuses returns all document status entries.
func (r *LightRAG) GetDocStatuses(ctx context.Context) (map[string]map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.docStatus == nil {
		return map[string]map[string]interface{}{}, nil
	}
	return r.docStatus.GetAll(ctx)
}

// ClearAllDocuments removes all documents and their associated data.
func (r *LightRAG) ClearAllDocuments(ctx context.Context) error {
	r.mu.RLock()
	if r.storagesStatus != StoragesInitialized {
		r.mu.RUnlock()
		return fmt.Errorf("storages not initialized, status: %s", r.storagesStatus)
	}
	r.mu.RUnlock()

	docs, err := r.fullDocs.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}

	ids := make([]string, 0, len(docs))
	for id := range docs {
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil
	}

	if err := r.fullDocs.Delete(ctx, ids); err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}
	if r.docStatus != nil {
		if err := r.docStatus.Delete(ctx, ids); err != nil {
			utils.Logger.Warnf("Failed to delete doc statuses: %v", err)
		}
	}

	return nil
}

// ClearLLMCache clears all cached LLM responses.
func (r *LightRAG) ClearLLMCache(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.llmResponseCache == nil {
		return nil
	}

	all, err := r.llmResponseCache.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to list cache entries: %w", err)
	}

	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil
	}

	return r.llmResponseCache.Delete(ctx, ids)
}

// Helper function to get namespace constants
func getKVNamespaces() []string {
	return namespace.AllKVNamespaces()
}

func getVectorNamespaces() []string {
	return namespace.AllVectorNamespaces()
}
