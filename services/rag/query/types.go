package query

// ConversationMessage represents a single message in the conversation history.
type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// QueryRequest represents a RAG query request.
type QueryRequest struct {
	Query               string                `json:"query"`
	Mode                *string               `json:"mode,omitempty"`
	ContextOnly         bool                  `json:"context_only,omitempty"`
	PromptOnly          bool                  `json:"prompt_only,omitempty"`
	IncludeReferences   bool                  `json:"include_references,omitempty"`
	MaxResults          *int                  `json:"max_results,omitempty"`
	ConversationHistory []ConversationMessage `json:"conversation_history,omitempty"`
	EnableRerank        bool                  `json:"enable_rerank,omitempty"`
	RerankModel         *string               `json:"rerank_model,omitempty"`
	RerankTopK          *int                  `json:"rerank_top_k,omitempty"`
	LLMProvider         *string               `json:"llm_provider,omitempty"`
	LLMModel            *string               `json:"llm_model,omitempty"`
}

// StreamQueryRequest represents a streaming RAG query request.
type StreamQueryRequest struct {
	Query string  `json:"query"`
	Mode  *string `json:"mode,omitempty"`
}

// SourceReference represents a retrieved context source.
type SourceReference struct {
	SourceType  string   `json:"source_type"`
	ID          string   `json:"id"`
	Score       float32  `json:"score"`
	RerankScore *float32 `json:"rerank_score,omitempty"`
	Snippet     *string  `json:"snippet,omitempty"`
	ReferenceID *int     `json:"reference_id,omitempty"`
	DocumentID  *string  `json:"document_id,omitempty"`
	FilePath    *string  `json:"file_path,omitempty"`
	StartLine   *int     `json:"start_line,omitempty"`
	EndLine     *int     `json:"end_line,omitempty"`
	ChunkIndex  *int     `json:"chunk_index,omitempty"`
}

// QueryStats represents retrieval and generation statistics.
type QueryStats struct {
	EmbeddingTimeMs  uint64   `json:"embedding_time_ms"`
	RetrievalTimeMs  uint64   `json:"retrieval_time_ms"`
	GenerationTimeMs uint64   `json:"generation_time_ms"`
	TotalTimeMs      uint64   `json:"total_time_ms"`
	SourcesRetrieved int      `json:"sources_retrieved"`
	RerankTimeMs     *uint64  `json:"rerank_time_ms,omitempty"`
	TokensUsed       *int     `json:"tokens_used,omitempty"`
	TokensPerSecond  *float32 `json:"tokens_per_second,omitempty"`
	LLMProvider      *string  `json:"llm_provider,omitempty"`
	LLMModel         *string  `json:"llm_model,omitempty"`
}

// QueryResponse represents a RAG query response.
type QueryResponse struct {
	Answer         string            `json:"answer"`
	Mode           string            `json:"mode"`
	Sources        []SourceReference `json:"sources"`
	Stats          QueryStats        `json:"stats"`
	ConversationID *string           `json:"conversation_id,omitempty"`
	Reranked       bool              `json:"reranked,omitempty"`
}
