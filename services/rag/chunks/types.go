package chunks

import "encoding/json"

// ChunkDetailResponse represents detailed information about a chunk.
type ChunkDetailResponse struct {
	ChunkID            string                      `json:"chunk_id"`
	DocumentID         string                      `json:"document_id"`
	DocumentName       *string                     `json:"document_name"`
	Content            string                      `json:"content"`
	Index              int                         `json:"index"`
	CharRange          CharRange                   `json:"char_range"`
	StartLine          *int                        `json:"start_line,omitempty"`
	EndLine            *int                        `json:"end_line,omitempty"`
	TokenCount         int                         `json:"token_count"`
	Entities           []ExtractedEntityInfo       `json:"entities"`
	Relationships      []ExtractedRelationshipInfo `json:"relationships"`
	ExtractionMetadata *ExtractionMetadataInfo     `json:"extraction_metadata"`
}

// CharRange represents a character range in a document.
type CharRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// ExtractedEntityInfo represents an entity extracted from a chunk.
type ExtractedEntityInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	EntityType  string  `json:"entity_type"`
	Description *string `json:"description"`
}

// ExtractedRelationshipInfo represents a relationship extracted from a chunk.
type ExtractedRelationshipInfo struct {
	SourceName   string  `json:"source_name"`
	TargetName   string  `json:"target_name"`
	RelationType string  `json:"relation_type"`
	Description  *string `json:"description"`
}

// ExtractionMetadataInfo represents metadata about the extraction process.
type ExtractionMetadataInfo struct {
	Model              string `json:"model"`
	GleaningIterations int    `json:"gleaning_iterations"`
	DurationMs         int64  `json:"duration_ms"`
	InputTokens        int    `json:"input_tokens"`
	OutputTokens       int    `json:"output_tokens"`
	Cached             bool   `json:"cached"`
}

// ChunkLineageResponse represents the lineage of a single chunk.
type ChunkLineageResponse struct {
	ChunkID           string          `json:"chunk_id"`
	DocumentID        string          `json:"document_id"`
	DocumentName      *string         `json:"document_name,omitempty"`
	DocumentType      *string         `json:"document_type,omitempty"`
	Index             int             `json:"index"`
	StartLine         *int            `json:"start_line,omitempty"`
	EndLine           *int            `json:"end_line,omitempty"`
	StartOffset       *int            `json:"start_offset,omitempty"`
	EndOffset         *int            `json:"end_offset,omitempty"`
	TokenCount        int             `json:"token_count"`
	ContentPreview    string          `json:"content_preview"`
	EntityCount       int             `json:"entity_count"`
	RelationshipCount int             `json:"relationship_count"`
	EntityNames       []string        `json:"entity_names"`
	DocumentMetadata  json.RawMessage `json:"document_metadata,omitempty"`
}
