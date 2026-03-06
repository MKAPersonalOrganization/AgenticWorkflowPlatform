package provenance

type EntityProvenanceResponse struct {
	EntityID             string              `json:"entity_id"`
	EntityName           string              `json:"entity_name"`
	EntityType           string              `json:"entity_type"`
	Description          *string             `json:"description"`
	Sources              []EntitySourceInfo  `json:"sources"`
	TotalExtractionCount int                 `json:"total_extraction_count"`
	RelatedEntities      []RelatedEntityInfo `json:"related_entities"`
}

type EntitySourceInfo struct {
	DocumentID       string            `json:"document_id"`
	DocumentName     *string           `json:"document_name"`
	Chunks           []ChunkSourceInfo `json:"chunks"`
	FirstExtractedAt *string           `json:"first_extracted_at"`
}

type ChunkSourceInfo struct {
	ChunkID    string  `json:"chunk_id"`
	StartLine  *int    `json:"start_line"`
	EndLine    *int    `json:"end_line"`
	SourceText *string `json:"source_text"`
}

type RelatedEntityInfo struct {
	EntityID         string `json:"entity_id"`
	EntityName       string `json:"entity_name"`
	RelationshipType string `json:"relationship_type"`
	SharedDocuments  int    `json:"shared_documents"`
}
