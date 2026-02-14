package chunks

type ChunkDetailResponse struct {
ChunkID            string                      `json:"chunk_id"`
DocumentID         string                      `json:"document_id"`
DocumentName       *string                     `json:"document_name"`
Content            string                      `json:"content"`
Index              int                         `json:"index"`
CharRange          CharRange                   `json:"char_range"`
TokenCount         int                         `json:"token_count"`
Entities           []ExtractedEntityInfo       `json:"entities"`
Relationships      []ExtractedRelationshipInfo `json:"relationships"`
ExtractionMetadata *ExtractionMetadataInfo     `json:"extraction_metadata"`
}

type CharRange struct {
Start int `json:"start"`
End   int `json:"end"`
}

type ExtractedEntityInfo struct {
ID          string  `json:"id"`
Name        string  `json:"name"`
EntityType  string  `json:"entity_type"`
Description *string `json:"description"`
}

type ExtractedRelationshipInfo struct {
SourceName   string  `json:"source_name"`
TargetName   string  `json:"target_name"`
RelationType string  `json:"relation_type"`
Description  *string `json:"description"`
}

type ExtractionMetadataInfo struct {
Model              string `json:"model"`
GleaningIterations int    `json:"gleaning_iterations"`
DurationMs         int64  `json:"duration_ms"`
InputTokens        int    `json:"input_tokens"`
OutputTokens       int    `json:"output_tokens"`
Cached             bool   `json:"cached"`
}
