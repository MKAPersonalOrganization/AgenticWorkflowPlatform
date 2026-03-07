package lineage

type EntityLineageResponse struct {
	EntityName          string                       `json:"entity_name"`
	EntityType          *string                      `json:"entity_type"`
	SourceDocuments     []SourceDocumentInfo         `json:"source_documents"`
	SourceCount         int                          `json:"source_count"`
	DescriptionVersions []DescriptionVersionResponse `json:"description_versions"`
}

type SourceDocumentInfo struct {
	DocumentID string          `json:"document_id"`
	ChunkIDs   []string        `json:"chunk_ids"`
	LineRanges []LineRangeInfo `json:"line_ranges"`
}

type LineRangeInfo struct {
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
}

type DescriptionVersionResponse struct {
	Version       int     `json:"version"`
	Description   string  `json:"description"`
	SourceChunkID *string `json:"source_chunk_id"`
	CreatedAt     string  `json:"created_at"`
}

type DocumentGraphLineageResponse struct {
	DocumentID      string                        `json:"document_id"`
	ChunkCount      int                           `json:"chunk_count"`
	Entities        []EntitySummaryResponse       `json:"entities"`
	Relationships   []RelationshipSummaryResponse `json:"relationships"`
	ExtractionStats ExtractionStatsResponse       `json:"extraction_stats"`
}

type EntitySummaryResponse struct {
	Name         string   `json:"name"`
	EntityType   string   `json:"entity_type"`
	SourceChunks []string `json:"source_chunks"`
	IsShared     bool     `json:"is_shared"`
}

type RelationshipSummaryResponse struct {
	Source       string   `json:"source"`
	Target       string   `json:"target"`
	Keywords     string   `json:"keywords"`
	SourceChunks []string `json:"source_chunks"`
}

type ExtractionStatsResponse struct {
	TotalEntities       int    `json:"total_entities"`
	UniqueEntities      int    `json:"unique_entities"`
	TotalRelationships  int    `json:"total_relationships"`
	UniqueRelationships int    `json:"unique_relationships"`
	ProcessingTimeMs    *int64 `json:"processing_time_ms"`
}
