package graph

import "encoding/json"

// GraphNodeResponse represents a graph node.
type GraphNodeResponse struct {
ID          string          `json:"id"`
Label       string          `json:"label"`
NodeType    string          `json:"node_type"`
Description string          `json:"description"`
Degree      int             `json:"degree"`
Properties  json.RawMessage `json:"properties"`
}

// GraphEdgeResponse represents a graph edge.
type GraphEdgeResponse struct {
Source     string          `json:"source"`
Target     string          `json:"target"`
EdgeType   string          `json:"edge_type"`
Weight     float32         `json:"weight"`
Properties json.RawMessage `json:"properties"`
}

// KnowledgeGraphResponse represents the full graph response.
type KnowledgeGraphResponse struct {
Nodes       []GraphNodeResponse `json:"nodes"`
Edges       []GraphEdgeResponse `json:"edges"`
IsTruncated bool                `json:"is_truncated"`
TotalNodes  int                 `json:"total_nodes"`
TotalEdges  int                 `json:"total_edges"`
}

// GraphQueryParams for GET /api/v1/graph.
type GraphQueryParams struct {
StartNode *string `json:"start_node,omitempty"`
Depth     int     `json:"depth,omitempty"`
MaxNodes  int     `json:"max_nodes,omitempty"`
}

// SearchLabelsResponse from GET /api/v1/graph/labels/search.
type SearchLabelsResponse struct {
Labels []string `json:"labels"`
}

// SearchNodesResponse from GET /api/v1/graph/nodes/search.
type SearchNodesResponse struct {
Nodes        []GraphNodeResponse `json:"nodes"`
Edges        []GraphEdgeResponse `json:"edges"`
TotalMatches int                 `json:"total_matches"`
IsTruncated  bool                `json:"is_truncated"`
}

// PopularLabel represents a popular label entry.
type PopularLabel struct {
Label       string `json:"label"`
EntityType  string `json:"entity_type"`
Degree      int    `json:"degree"`
Description string `json:"description"`
}

// PopularLabelsResponse from GET /api/v1/graph/labels/popular.
type PopularLabelsResponse struct {
Labels        []PopularLabel `json:"labels"`
TotalEntities int            `json:"total_entities"`
}

// BatchDegreeRequest for POST /api/v1/graph/degrees/batch.
type BatchDegreeRequest struct {
NodeIDs []string `json:"node_ids"`
}

// NodeDegree represents a single node's degree.
type NodeDegree struct {
NodeID string `json:"node_id"`
Degree int    `json:"degree"`
}

// BatchDegreeResponse from POST /api/v1/graph/degrees/batch.
type BatchDegreeResponse struct {
Degrees []NodeDegree `json:"degrees"`
Count   int          `json:"count"`
}
