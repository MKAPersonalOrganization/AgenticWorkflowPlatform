// Package types provides data structures for LightRAG knowledge graph operations.
package types

import (
	"encoding/json"
)

// GPTKeywordExtractionFormat represents the output format for keyword extraction.
type GPTKeywordExtractionFormat struct {
	HighLevelKeywords []string `json:"high_level_keywords"`
	LowLevelKeywords  []string `json:"low_level_keywords"`
}

// KnowledgeGraphNode represents a node in the knowledge graph.
type KnowledgeGraphNode struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

// KnowledgeGraphEdge represents an edge/relationship in the knowledge graph.
type KnowledgeGraphEdge struct {
	Source      string                 `json:"source"`
	Target      string                 `json:"target"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Weight      float64                `json:"weight"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

// KnowledgeGraph represents a complete knowledge graph with nodes and edges.
type KnowledgeGraph struct {
	Nodes []KnowledgeGraphNode `json:"nodes"`
	Edges []KnowledgeGraphEdge `json:"edges"`
}

// NewKnowledgeGraph creates a new empty KnowledgeGraph.
func NewKnowledgeGraph() *KnowledgeGraph {
	return &KnowledgeGraph{
		Nodes: make([]KnowledgeGraphNode, 0),
		Edges: make([]KnowledgeGraphEdge, 0),
	}
}

// AddNode adds a node to the knowledge graph.
func (kg *KnowledgeGraph) AddNode(node KnowledgeGraphNode) {
	kg.Nodes = append(kg.Nodes, node)
}

// AddEdge adds an edge to the knowledge graph.
func (kg *KnowledgeGraph) AddEdge(edge KnowledgeGraphEdge) {
	kg.Edges = append(kg.Edges, edge)
}

// GetNode returns a node by its ID, or nil if not found.
func (kg *KnowledgeGraph) GetNode(id string) *KnowledgeGraphNode {
	for i := range kg.Nodes {
		if kg.Nodes[i].ID == id {
			return &kg.Nodes[i]
		}
	}
	return nil
}

// GetNodeByName returns a node by its name, or nil if not found.
func (kg *KnowledgeGraph) GetNodeByName(name string) *KnowledgeGraphNode {
	for i := range kg.Nodes {
		if kg.Nodes[i].Name == name {
			return &kg.Nodes[i]
		}
	}
	return nil
}

// GetEdge returns an edge between source and target, or nil if not found.
func (kg *KnowledgeGraph) GetEdge(source, target string) *KnowledgeGraphEdge {
	for i := range kg.Edges {
		if kg.Edges[i].Source == source && kg.Edges[i].Target == target {
			return &kg.Edges[i]
		}
	}
	return nil
}

// GetNodeNeighbors returns all nodes connected to the given node.
func (kg *KnowledgeGraph) GetNodeNeighbors(nodeID string) []KnowledgeGraphNode {
	neighbors := make([]KnowledgeGraphNode, 0)
	connectedIDs := make(map[string]bool)

	// Find all connected node IDs
	for _, edge := range kg.Edges {
		if edge.Source == nodeID {
			connectedIDs[edge.Target] = true
		}
		if edge.Target == nodeID {
			connectedIDs[edge.Source] = true
		}
	}

	// Collect neighbor nodes
	for _, node := range kg.Nodes {
		if connectedIDs[node.ID] {
			neighbors = append(neighbors, node)
		}
	}

	return neighbors
}

// ToJSON serializes the knowledge graph to JSON.
func (kg *KnowledgeGraph) ToJSON() ([]byte, error) {
	return json.Marshal(kg)
}

// FromJSON deserializes a knowledge graph from JSON.
func (kg *KnowledgeGraph) FromJSON(data []byte) error {
	return json.Unmarshal(data, kg)
}

// Merge combines another knowledge graph into this one.
func (kg *KnowledgeGraph) Merge(other *KnowledgeGraph) {
	// Create map of existing nodes by ID
	existingNodes := make(map[string]bool)
	for _, node := range kg.Nodes {
		existingNodes[node.ID] = true
	}

	// Add new nodes
	for _, node := range other.Nodes {
		if !existingNodes[node.ID] {
			kg.Nodes = append(kg.Nodes, node)
		}
	}

	// Create map of existing edges
	existingEdges := make(map[string]bool)
	for _, edge := range kg.Edges {
		key := edge.Source + "->" + edge.Target
		existingEdges[key] = true
	}

	// Add new edges
	for _, edge := range other.Edges {
		key := edge.Source + "->" + edge.Target
		if !existingEdges[key] {
			kg.Edges = append(kg.Edges, edge)
		}
	}
}

// NodeCount returns the number of nodes in the graph.
func (kg *KnowledgeGraph) NodeCount() int {
	return len(kg.Nodes)
}

// EdgeCount returns the number of edges in the graph.
func (kg *KnowledgeGraph) EdgeCount() int {
	return len(kg.Edges)
}

// Clear removes all nodes and edges from the graph.
func (kg *KnowledgeGraph) Clear() {
	kg.Nodes = make([]KnowledgeGraphNode, 0)
	kg.Edges = make([]KnowledgeGraphEdge, 0)
}

// QueryResult represents the result of a RAG query.
type QueryResult struct {
	Answer     string            `json:"answer"`
	Sources    []SourceReference `json:"sources,omitempty"`
	Keywords   []string          `json:"keywords,omitempty"`
	QueryMode  string            `json:"query_mode"`
	Successful bool              `json:"successful"`
}

// SourceReference represents a reference to a source document or chunk.
type SourceReference struct {
	ChunkID  string  `json:"chunk_id"`
	DocID    string  `json:"doc_id"`
	Content  string  `json:"content"`
	Score    float64 `json:"score"`
	FilePath string  `json:"file_path,omitempty"`
	StartPos int     `json:"start_pos,omitempty"`
	EndPos   int     `json:"end_pos,omitempty"`
}

// InsertResult represents the result of a document insertion.
type InsertResult struct {
	DocID       string   `json:"doc_id"`
	ChunkCount  int      `json:"chunk_count"`
	EntityCount int      `json:"entity_count"`
	EdgeCount   int      `json:"edge_count"`
	Successful  bool     `json:"successful"`
	Errors      []string `json:"errors,omitempty"`
}

// EntityInfo represents detailed information about an extracted entity.
type EntityInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	SourceIDs   []string `json:"source_ids"`
	ChunkIDs    []string `json:"chunk_ids"`
	Occurrences int      `json:"occurrences"`
}

// RelationInfo represents detailed information about an extracted relation.
type RelationInfo struct {
	SourceEntity string   `json:"source_entity"`
	TargetEntity string   `json:"target_entity"`
	RelationType string   `json:"relation_type"`
	Description  string   `json:"description"`
	Weight       float64  `json:"weight"`
	Keywords     []string `json:"keywords"`
	SourceIDs    []string `json:"source_ids"`
}

// ChunkInfo represents information about a text chunk.
type ChunkInfo struct {
	ChunkID       string   `json:"chunk_id"`
	DocID         string   `json:"doc_id"`
	Content       string   `json:"content"`
	TokenCount    int      `json:"token_count"`
	StartPos      int      `json:"start_pos"`
	EndPos        int      `json:"end_pos"`
	Entities      []string `json:"entities,omitempty"`
	ChunkOrderIdx int      `json:"chunk_order_idx"`
}

// GraphStatistics represents statistics about the knowledge graph.
type GraphStatistics struct {
	TotalNodes     int            `json:"total_nodes"`
	TotalEdges     int            `json:"total_edges"`
	TotalChunks    int            `json:"total_chunks"`
	TotalDocuments int            `json:"total_documents"`
	NodesByType    map[string]int `json:"nodes_by_type"`
	EdgesByType    map[string]int `json:"edges_by_type"`
	AverageDegree  float64        `json:"average_degree"`
	ConnectedComps int            `json:"connected_components"`
}
