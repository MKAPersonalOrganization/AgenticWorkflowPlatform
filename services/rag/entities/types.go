package entities

import "encoding/json"

type EntityResponse struct {
ID          string          `json:"id"`
EntityName  string          `json:"entity_name"`
EntityType  string          `json:"entity_type"`
Description string          `json:"description"`
SourceID    string          `json:"source_id"`
CreatedAt   string          `json:"created_at"`
UpdatedAt   string          `json:"updated_at"`
Degree      int             `json:"degree"`
Metadata    json.RawMessage `json:"metadata"`
}

type ListEntitiesResponse struct {
Items      []EntityResponse `json:"items"`
Total      int              `json:"total"`
Page       int              `json:"page"`
PageSize   int              `json:"page_size"`
TotalPages int              `json:"total_pages"`
}

type ListEntitiesParams struct {
Page       int
PageSize   int
EntityType string
Search     string
}

type CreateEntityRequest struct {
EntityName  string          `json:"entity_name"`
EntityType  string          `json:"entity_type"`
Description string          `json:"description"`
SourceID    string          `json:"source_id"`
Metadata    json.RawMessage `json:"metadata,omitempty"`
}

type CreateEntityResponse struct {
Status  string         `json:"status"`
Message string         `json:"message"`
Entity  EntityResponse `json:"entity"`
}

type UpdateEntityRequest struct {
EntityType  *string          `json:"entity_type,omitempty"`
Description *string          `json:"description,omitempty"`
Metadata    *json.RawMessage `json:"metadata,omitempty"`
}

type UpdateEntityResponse struct {
Status  string         `json:"status"`
Message string         `json:"message"`
Entity  EntityResponse `json:"entity"`
Changes ChangesSummary `json:"changes"`
}

type ChangesSummary struct {
FieldsUpdated       []string `json:"fields_updated"`
PreviousDescription *string  `json:"previous_description,omitempty"`
}

type DeleteEntityResponse struct {
Status               string   `json:"status"`
Message              string   `json:"message"`
DeletedEntityID      string   `json:"deleted_entity_id"`
DeletedRelationships int      `json:"deleted_relationships"`
AffectedEntities     []string `json:"affected_entities"`
}

type EntityExistsResponse struct {
Exists     bool    `json:"exists"`
EntityID   *string `json:"entity_id"`
EntityType *string `json:"entity_type"`
Degree     *int    `json:"degree"`
}

type MergeEntitiesRequest struct {
SourceEntity  string          `json:"source_entity"`
TargetEntity  string          `json:"target_entity"`
MergeStrategy string          `json:"merge_strategy,omitempty"`
Metadata      json.RawMessage `json:"metadata,omitempty"`
}

type MergeEntitiesResponse struct {
Status       string         `json:"status"`
Message      string         `json:"message"`
MergedEntity EntityResponse `json:"merged_entity"`
MergeDetails MergeDetails   `json:"merge_details"`
}

type MergeDetails struct {
SourceEntityID                string `json:"source_entity_id"`
TargetEntityID                string `json:"target_entity_id"`
RelationshipsMerged           int    `json:"relationships_merged"`
DuplicateRelationshipsRemoved int    `json:"duplicate_relationships_removed"`
DescriptionStrategy           string `json:"description_strategy"`
MetadataStrategy              string `json:"metadata_strategy"`
}

type GetEntityResponse struct {
Entity        EntityResponse   `json:"entity"`
Relationships RelationshipsInfo `json:"relationships"`
Statistics    EntityStatistics  `json:"statistics"`
}

type RelationshipsInfo struct {
Outgoing []RelationshipSummary `json:"outgoing"`
Incoming []RelationshipSummary `json:"incoming"`
}

type RelationshipSummary struct {
Target       *string `json:"target,omitempty"`
Source       *string `json:"source,omitempty"`
RelationType string  `json:"relation_type"`
Weight       float64 `json:"weight"`
}

type EntityStatistics struct {
TotalRelationships int `json:"total_relationships"`
OutgoingCount      int `json:"outgoing_count"`
IncomingCount      int `json:"incoming_count"`
DocumentReferences int `json:"document_references"`
}

type EntityNeighborhoodResponse struct {
Nodes []NeighborhoodNode `json:"nodes"`
Edges []NeighborhoodEdge `json:"edges"`
}

type NeighborhoodNode struct {
ID          string `json:"id"`
EntityType  string `json:"entity_type"`
Description string `json:"description"`
Degree      int    `json:"degree"`
}

type NeighborhoodEdge struct {
ID           string  `json:"id"`
Source       string  `json:"source"`
Target       string  `json:"target"`
RelationType string  `json:"relation_type"`
Weight       float64 `json:"weight"`
}
