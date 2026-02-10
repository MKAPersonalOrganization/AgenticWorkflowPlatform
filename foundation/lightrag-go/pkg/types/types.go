// Package types provides type definitions for LightRAG.
package types

import (
	"os"
	"strconv"

	"github.com/lightrag/lightrag-go/pkg/constants"
)

// GPTKeywordExtractionFormat represents the format for GPT keyword extraction.
type GPTKeywordExtractionFormat struct {
	HighLevelKeywords []string `json:"high_level_keywords"`
	LowLevelKeywords  []string `json:"low_level_keywords"`
}

// KnowledgeGraphNode represents a node in the knowledge graph.
type KnowledgeGraphNode struct {
	ID         string                 `json:"id"`
	Labels     []string               `json:"labels"`
	Properties map[string]interface{} `json:"properties"`
}

// KnowledgeGraphEdge represents an edge in the knowledge graph.
type KnowledgeGraphEdge struct {
	ID         string                 `json:"id"`
	Type       *string                `json:"type,omitempty"`
	Source     string                 `json:"source"`
	Target     string                 `json:"target"`
	Properties map[string]interface{} `json:"properties"`
}

// KnowledgeGraph represents a knowledge graph structure.
type KnowledgeGraph struct {
	Nodes       []KnowledgeGraphNode `json:"nodes"`
	Edges       []KnowledgeGraphEdge `json:"edges"`
	IsTruncated bool                 `json:"is_truncated"`
}

// TextChunkSchema represents a text chunk.
type TextChunkSchema struct {
	Tokens          int    `json:"tokens"`
	Content         string `json:"content"`
	FullDocID       string `json:"full_doc_id"`
	ChunkOrderIndex int    `json:"chunk_order_index"`
}

// QueryMode represents the retrieval mode for queries.
type QueryMode string

const (
	QueryModeLocal  QueryMode = "local"
	QueryModeGlobal QueryMode = "global"
	QueryModeHybrid QueryMode = "hybrid"
	QueryModeNaive  QueryMode = "naive"
	QueryModeMix    QueryMode = "mix"
	QueryModeBypass QueryMode = "bypass"
)

// IsValid checks if the query mode is valid.
func (m QueryMode) IsValid() bool {
	switch m {
	case QueryModeLocal, QueryModeGlobal, QueryModeHybrid, QueryModeNaive, QueryModeMix, QueryModeBypass:
		return true
	}
	return false
}

// QueryParam holds configuration parameters for query execution in LightRAG.
type QueryParam struct {
	// Mode specifies the retrieval mode:
	// - "local": Focuses on context-dependent information.
	// - "global": Utilizes global knowledge.
	// - "hybrid": Combines local and global retrieval methods.
	// - "naive": Performs a basic search without advanced techniques.
	// - "mix": Integrates knowledge graph and vector retrieval.
	// - "bypass": Bypasses RAG and directly queries the LLM.
	Mode QueryMode `json:"mode"`

	// OnlyNeedContext if True, only returns the retrieved context without generating a response.
	OnlyNeedContext bool `json:"only_need_context"`

	// OnlyNeedPrompt if True, only returns the generated prompt without producing a response.
	OnlyNeedPrompt bool `json:"only_need_prompt"`

	// ResponseType defines the response format.
	ResponseType string `json:"response_type"`

	// Stream if True, enables streaming output for real-time responses.
	Stream bool `json:"stream"`

	// TopK is the number of top items to retrieve.
	TopK int `json:"top_k"`

	// ChunkTopK is the number of text chunks to retrieve.
	ChunkTopK int `json:"chunk_top_k"`

	// MaxEntityTokens is the maximum number of tokens allocated for entity context.
	MaxEntityTokens int `json:"max_entity_tokens"`

	// MaxRelationTokens is the maximum number of tokens allocated for relationship context.
	MaxRelationTokens int `json:"max_relation_tokens"`

	// MaxTotalTokens is the maximum total tokens budget for the entire query context.
	MaxTotalTokens int `json:"max_total_tokens"`

	// HLKeywords is a list of high-level keywords to prioritize in retrieval.
	HLKeywords []string `json:"hl_keywords"`

	// LLKeywords is a list of low-level keywords to refine retrieval focus.
	LLKeywords []string `json:"ll_keywords"`

	// ConversationHistory stores past conversation history to maintain context.
	ConversationHistory []ConversationMessage `json:"conversation_history"`

	// HistoryTurns is the number of complete conversation turns to consider.
	HistoryTurns int `json:"history_turns"`

	// UserPrompt is user-provided prompt for the query.
	UserPrompt *string `json:"user_prompt,omitempty"`

	// EnableRerank enables reranking for retrieved text chunks.
	EnableRerank bool `json:"enable_rerank"`

	// IncludeReferences if True, includes reference list in the response.
	IncludeReferences bool `json:"include_references"`
}

// ConversationMessage represents a message in conversation history.
type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewQueryParam creates a new QueryParam with default values.
func NewQueryParam() *QueryParam {
	return &QueryParam{
		Mode:                QueryModeMix,
		OnlyNeedContext:     false,
		OnlyNeedPrompt:      false,
		ResponseType:        "Multiple Paragraphs",
		Stream:              false,
		TopK:                getEnvInt("TOP_K", constants.DefaultTopK),
		ChunkTopK:           getEnvInt("CHUNK_TOP_K", constants.DefaultChunkTopK),
		MaxEntityTokens:     getEnvInt("MAX_ENTITY_TOKENS", constants.DefaultMaxEntityTokens),
		MaxRelationTokens:   getEnvInt("MAX_RELATION_TOKENS", constants.DefaultMaxRelationTokens),
		MaxTotalTokens:      getEnvInt("MAX_TOTAL_TOKENS", constants.DefaultMaxTotalTokens),
		HLKeywords:          []string{},
		LLKeywords:          []string{},
		ConversationHistory: []ConversationMessage{},
		HistoryTurns:        getEnvInt("HISTORY_TURNS", constants.DefaultHistoryTurns),
		EnableRerank:        getEnvBool("RERANK_BY_DEFAULT", true),
		IncludeReferences:   false,
	}
}

// DocStatus represents the status of a document in the processing pipeline.
type DocStatus string

const (
	DocStatusPending    DocStatus = "PENDING"
	DocStatusProcessing DocStatus = "PROCESSING"
	DocStatusCompleted  DocStatus = "COMPLETED"
	DocStatusFailed     DocStatus = "FAILED"
)

// DocProcessingStatus represents the processing status of a document.
type DocProcessingStatus struct {
	ID              string    `json:"id"`
	FileName        string    `json:"file_name"`
	FilePath        string    `json:"file_path"`
	Status          DocStatus `json:"status"`
	CreatedAt       string    `json:"created_at,omitempty"`
	UpdatedAt       string    `json:"updated_at,omitempty"`
	ChunksTotal     int       `json:"chunks_total"`
	ChunksProcessed int       `json:"chunks_processed"`
	Error           string    `json:"error,omitempty"`
	TrackID         string    `json:"track_id,omitempty"`
}

// DeletionResult represents the result of a deletion operation.
type DeletionResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Deleted int    `json:"deleted"`
}

// Helper functions for environment variable parsing
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "True" || value == "1"
	}
	return defaultValue
}

// SearchMode represents query mode for Ollama API.
type SearchMode string

const (
	SearchModeNaive   SearchMode = "naive"
	SearchModeLocal   SearchMode = "local"
	SearchModeGlobal  SearchMode = "global"
	SearchModeHybrid  SearchMode = "hybrid"
	SearchModeMix     SearchMode = "mix"
	SearchModeBypass  SearchMode = "bypass"
	SearchModeContext SearchMode = "context"
)
