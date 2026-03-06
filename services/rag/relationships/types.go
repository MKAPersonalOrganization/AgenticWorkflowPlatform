package relationships

import "encoding/json"

type RelationshipResponse struct {
	ID           string          `json:"id"`
	SrcID        string          `json:"src_id"`
	TgtID        string          `json:"tgt_id"`
	RelationType string          `json:"relation_type"`
	Keywords     string          `json:"keywords"`
	Weight       float64         `json:"weight"`
	Description  string          `json:"description"`
	SourceID     string          `json:"source_id"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
	Metadata     json.RawMessage `json:"metadata"`
}

type ListRelationshipsResponse struct {
	Items      []RelationshipResponse `json:"items"`
	Total      int                    `json:"total"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
	TotalPages int                    `json:"total_pages"`
}

type ListRelationshipsParams struct {
	Page             int
	PageSize         int
	RelationshipType string
}

type CreateRelationshipRequest struct {
	SrcID       string          `json:"src_id"`
	TgtID       string          `json:"tgt_id"`
	Keywords    string          `json:"keywords"`
	Weight      float64         `json:"weight,omitempty"`
	Description string          `json:"description"`
	SourceID    string          `json:"source_id"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

type CreateRelationshipResponse struct {
	Status       string               `json:"status"`
	Message      string               `json:"message"`
	Relationship RelationshipResponse `json:"relationship"`
}

type UpdateRelationshipRequest struct {
	Keywords    *string          `json:"keywords,omitempty"`
	Weight      *float64         `json:"weight,omitempty"`
	Description *string          `json:"description,omitempty"`
	Metadata    *json.RawMessage `json:"metadata,omitempty"`
}

type UpdateRelationshipResponse struct {
	Status       string                     `json:"status"`
	Message      string                     `json:"message"`
	Relationship RelationshipResponse       `json:"relationship"`
	Changes      RelationshipChangesSummary `json:"changes"`
}

type RelationshipChangesSummary struct {
	FieldsUpdated  []string `json:"fields_updated"`
	PreviousWeight *float64 `json:"previous_weight,omitempty"`
}

type GetRelationshipResponse struct {
	Relationship RelationshipResponse `json:"relationship"`
	Entities     RelationshipEntities `json:"entities"`
}

type RelationshipEntities struct {
	Source EntitySummary `json:"source"`
	Target EntitySummary `json:"target"`
}

type EntitySummary struct {
	ID         string `json:"id"`
	EntityType string `json:"entity_type"`
}

type DeleteRelationshipResponse struct {
	Status                string `json:"status"`
	Message               string `json:"message"`
	DeletedRelationshipID string `json:"deleted_relationship_id"`
	SrcID                 string `json:"src_id"`
	TgtID                 string `json:"tgt_id"`
}
