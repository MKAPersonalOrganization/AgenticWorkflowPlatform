package chat

// ChatCompletionRequest is the request body for the chat completions endpoint.
type ChatCompletionRequest struct {
ConversationID *string  `json:"conversation_id,omitempty"`
Message        string   `json:"message"`
Mode           *string  `json:"mode,omitempty"`
Stream         bool     `json:"stream"`
MaxTokens      *int     `json:"max_tokens,omitempty"`
Temperature    *float32 `json:"temperature,omitempty"`
TopK           *int     `json:"top_k,omitempty"`
ParentID       *string  `json:"parent_id,omitempty"`
Provider       *string  `json:"provider,omitempty"`
Model          *string  `json:"model,omitempty"`
}

// ChatCompletionResponse is the response body from the chat completions endpoint.
type ChatCompletionResponse struct {
ConversationID     string            `json:"conversation_id"`
UserMessageID      string            `json:"user_message_id"`
AssistantMessageID string            `json:"assistant_message_id"`
Content            string            `json:"content"`
Mode               string            `json:"mode"`
Sources            []SourceReference `json:"sources"`
Stats              QueryStats        `json:"stats"`
TokensUsed         int               `json:"tokens_used"`
DurationMs         int64             `json:"duration_ms"`
LLMProvider        *string           `json:"llm_provider,omitempty"`
LLMModel           *string           `json:"llm_model,omitempty"`
}

// SourceReference represents a source used in the response.
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

// QueryStats contains timing and usage statistics for a query.
type QueryStats struct {
EmbeddingTimeMs  int64    `json:"embedding_time_ms"`
RetrievalTimeMs  int64    `json:"retrieval_time_ms"`
GenerationTimeMs int64    `json:"generation_time_ms"`
TotalTimeMs      int64    `json:"total_time_ms"`
SourcesRetrieved int      `json:"sources_retrieved"`
RerankTimeMs     *int64   `json:"rerank_time_ms,omitempty"`
TokensUsed       *int     `json:"tokens_used,omitempty"`
TokensPerSecond  *float32 `json:"tokens_per_second,omitempty"`
LLMProvider      *string  `json:"llm_provider,omitempty"`
LLMModel         *string  `json:"llm_model,omitempty"`
}

// ErrorResponse represents an error returned by the API.
type ErrorResponse struct {
Code    string `json:"code"`
Message string `json:"message"`
}
