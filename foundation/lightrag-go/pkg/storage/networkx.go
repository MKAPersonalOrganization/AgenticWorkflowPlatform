// Package storage provides graph storage implementations.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/lightrag/lightrag-go/pkg/core/base"
	"github.com/sirupsen/logrus"
)

// GraphNode represents a node in the graph.
type GraphNode struct {
	ID         string            `json:"id"`
	Properties map[string]string `json:"properties"`
}

// GraphEdge represents an edge in the graph.
type GraphEdge struct {
	Source     string            `json:"source"`
	Target     string            `json:"target"`
	Properties map[string]string `json:"properties"`
}

// GraphData represents the serializable graph structure.
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// NetworkXStorage implements BaseGraphStorage using an in-memory graph structure.
type NetworkXStorage struct {
	namespace     string
	workspace     string
	globalConfig  map[string]interface{}
	embeddingFunc EmbeddingFunc

	nodes       map[string]map[string]string
	edges       map[string]map[string]map[string]string // source -> target -> properties
	mu          sync.RWMutex
	dirty       bool
	initialized bool

	logger *logrus.Logger
}

// NewNetworkXStorage creates a new NetworkXStorage instance.
func NewNetworkXStorage(config *StorageConfig, logger *logrus.Logger) *NetworkXStorage {
	if logger == nil {
		logger = logrus.New()
	}
	return &NetworkXStorage{
		namespace:     config.Namespace,
		workspace:     config.Workspace,
		globalConfig:  config.GlobalConfig,
		embeddingFunc: config.EmbeddingFunc,
		nodes:         make(map[string]map[string]string),
		edges:         make(map[string]map[string]map[string]string),
		logger:        logger,
	}
}

// Namespace returns the namespace.
func (s *NetworkXStorage) Namespace() string {
	return s.namespace
}

// GetNamespace returns the namespace.
func (s *NetworkXStorage) GetNamespace() string {
	return s.namespace
}

// Workspace returns the workspace path.
func (s *NetworkXStorage) Workspace() string {
	return s.workspace
}

// GetWorkspace returns the workspace path.
func (s *NetworkXStorage) GetWorkspace() string {
	return s.workspace
}

// getFilePath returns the path to the graph storage file.
func (s *NetworkXStorage) getFilePath() string {
	return filepath.Join(s.workspace, fmt.Sprintf("graph_%s.json", s.namespace))
}

// Initialize sets up the storage.
func (s *NetworkXStorage) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	filePath := s.getFilePath()

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		s.nodes = make(map[string]map[string]string)
		s.edges = make(map[string]map[string]map[string]string)
		s.initialized = true
		s.logger.WithField("namespace", s.namespace).Debug("NetworkXStorage initialized with empty data")
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %w", err)
	}

	var graphData GraphData
	if err := json.Unmarshal(data, &graphData); err != nil {
		return fmt.Errorf("failed to parse storage file: %w", err)
	}

	// Load nodes
	s.nodes = make(map[string]map[string]string)
	for _, node := range graphData.Nodes {
		s.nodes[node.ID] = node.Properties
	}

	// Load edges
	s.edges = make(map[string]map[string]map[string]string)
	for _, edge := range graphData.Edges {
		if _, ok := s.edges[edge.Source]; !ok {
			s.edges[edge.Source] = make(map[string]map[string]string)
		}
		s.edges[edge.Source][edge.Target] = edge.Properties
	}

	s.initialized = true
	s.logger.WithFields(logrus.Fields{
		"namespace":  s.namespace,
		"node_count": len(s.nodes),
		"edge_count": len(graphData.Edges),
	}).Debug("NetworkXStorage initialized with existing data")

	return nil
}

// Finalize cleans up storage resources.
func (s *NetworkXStorage) Finalize(ctx context.Context) error {
	return s.IndexDoneCallback(ctx)
}

// IndexDoneCallback persists the graph to disk.
func (s *NetworkXStorage) IndexDoneCallback(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.dirty {
		return nil
	}

	if err := os.MkdirAll(s.workspace, 0o755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Build graph data
	graphData := GraphData{
		Nodes: make([]GraphNode, 0, len(s.nodes)),
		Edges: make([]GraphEdge, 0),
	}

	for id, props := range s.nodes {
		graphData.Nodes = append(graphData.Nodes, GraphNode{
			ID:         id,
			Properties: props,
		})
	}

	for source, targets := range s.edges {
		for target, props := range targets {
			graphData.Edges = append(graphData.Edges, GraphEdge{
				Source:     source,
				Target:     target,
				Properties: props,
			})
		}
	}

	data, err := json.MarshalIndent(graphData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal graph data: %w", err)
	}

	filePath := s.getFilePath()
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write storage file: %w", err)
	}

	s.dirty = false
	s.logger.WithFields(logrus.Fields{
		"namespace":  s.namespace,
		"node_count": len(s.nodes),
		"edge_count": len(graphData.Edges),
	}).Debug("NetworkXStorage persisted to disk")

	return nil
}

// Drop removes all data from storage.
func (s *NetworkXStorage) Drop(ctx context.Context) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nodes = make(map[string]map[string]string)
	s.edges = make(map[string]map[string]map[string]string)
	s.dirty = false

	filePath := s.getFilePath()
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return map[string]string{
			"status":  "error",
			"message": fmt.Sprintf("failed to remove file: %v", err),
		}, nil
	}

	s.logger.WithField("namespace", s.namespace).Debug("NetworkXStorage dropped")

	return map[string]string{
		"status":  "success",
		"message": "data dropped",
	}, nil
}

// HasNode checks if a node exists.
func (s *NetworkXStorage) HasNode(ctx context.Context, nodeID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.nodes[nodeID]
	return ok, nil
}

// HasEdge checks if an edge exists between two nodes.
func (s *NetworkXStorage) HasEdge(ctx context.Context, sourceNodeID, targetNodeID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check both directions (undirected graph)
	if targets, ok := s.edges[sourceNodeID]; ok {
		if _, ok := targets[targetNodeID]; ok {
			return true, nil
		}
	}
	if targets, ok := s.edges[targetNodeID]; ok {
		if _, ok := targets[sourceNodeID]; ok {
			return true, nil
		}
	}
	return false, nil
}

// NodeDegree returns the degree of a node.
func (s *NetworkXStorage) NodeDegree(ctx context.Context, nodeID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.nodes[nodeID]; !ok {
		return 0, nil
	}

	degree := 0

	// Count outgoing edges
	if targets, ok := s.edges[nodeID]; ok {
		degree += len(targets)
	}

	// Count incoming edges (for undirected graph, avoid double counting)
	for source, targets := range s.edges {
		if source == nodeID {
			continue
		}
		if _, ok := targets[nodeID]; ok {
			degree++
		}
	}

	return degree, nil
}

// EdgeDegree returns the combined degree of an edge's endpoints.
func (s *NetworkXStorage) EdgeDegree(ctx context.Context, srcID, tgtID string) (int, error) {
	srcDegree, err := s.NodeDegree(ctx, srcID)
	if err != nil {
		return 0, err
	}
	tgtDegree, err := s.NodeDegree(ctx, tgtID)
	if err != nil {
		return 0, err
	}
	return srcDegree + tgtDegree, nil
}

// GetNode retrieves a node by its ID.
func (s *NetworkXStorage) GetNode(ctx context.Context, nodeID string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if props, ok := s.nodes[nodeID]; ok {
		result := make(map[string]string)
		for k, v := range props {
			result[k] = v
		}
		return result, nil
	}

	return nil, nil
}

// GetEdge retrieves edge properties between two nodes.
func (s *NetworkXStorage) GetEdge(ctx context.Context, sourceNodeID, targetNodeID string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check forward direction
	if targets, ok := s.edges[sourceNodeID]; ok {
		if props, ok := targets[targetNodeID]; ok {
			result := make(map[string]string)
			for k, v := range props {
				result[k] = v
			}
			return result, nil
		}
	}

	// Check reverse direction (undirected graph)
	if targets, ok := s.edges[targetNodeID]; ok {
		if props, ok := targets[sourceNodeID]; ok {
			result := make(map[string]string)
			for k, v := range props {
				result[k] = v
			}
			return result, nil
		}
	}

	return nil, nil
}

// GetNodeEdges retrieves all edges connected to a node.
func (s *NetworkXStorage) GetNodeEdges(ctx context.Context, sourceNodeID string) ([][2]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.nodes[sourceNodeID]; !ok {
		return nil, nil
	}

	edges := make([][2]string, 0)

	// Outgoing edges
	if targets, ok := s.edges[sourceNodeID]; ok {
		for target := range targets {
			edges = append(edges, [2]string{sourceNodeID, target})
		}
	}

	// Incoming edges
	for source, targets := range s.edges {
		if source == sourceNodeID {
			continue
		}
		if _, ok := targets[sourceNodeID]; ok {
			edges = append(edges, [2]string{source, sourceNodeID})
		}
	}

	return edges, nil
}

// GetNodesBatch retrieves multiple nodes.
func (s *NetworkXStorage) GetNodesBatch(ctx context.Context, nodeIDs []string) (map[string]map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[string]string)

	for _, nodeID := range nodeIDs {
		if props, ok := s.nodes[nodeID]; ok {
			nodeCopy := make(map[string]string)
			for k, v := range props {
				nodeCopy[k] = v
			}
			result[nodeID] = nodeCopy
		}
	}

	return result, nil
}

// NodeDegreesBatch retrieves degrees for multiple nodes.
func (s *NetworkXStorage) NodeDegreesBatch(ctx context.Context, nodeIDs []string) (map[string]int, error) {
	result := make(map[string]int)

	for _, nodeID := range nodeIDs {
		degree, err := s.NodeDegree(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		result[nodeID] = degree
	}

	return result, nil
}

// EdgeDegreesBatch retrieves degrees for multiple edges.
func (s *NetworkXStorage) EdgeDegreesBatch(ctx context.Context, edgePairs [][2]string) (map[[2]string]int, error) {
	result := make(map[[2]string]int)

	for _, pair := range edgePairs {
		degree, err := s.EdgeDegree(ctx, pair[0], pair[1])
		if err != nil {
			return nil, err
		}
		result[pair] = degree
	}

	return result, nil
}

// GetEdgesBatch retrieves multiple edges.
func (s *NetworkXStorage) GetEdgesBatch(ctx context.Context, pairs []map[string]string) (map[[2]string]map[string]string, error) {
	result := make(map[[2]string]map[string]string)

	for _, pair := range pairs {
		src := pair["src"]
		tgt := pair["tgt"]
		edge, err := s.GetEdge(ctx, src, tgt)
		if err != nil {
			return nil, err
		}
		if edge != nil {
			result[[2]string{src, tgt}] = edge
		}
	}

	return result, nil
}

// GetNodesEdgesBatch retrieves edges for multiple nodes.
func (s *NetworkXStorage) GetNodesEdgesBatch(ctx context.Context, nodeIDs []string) (map[string][][2]string, error) {
	result := make(map[string][][2]string)

	for _, nodeID := range nodeIDs {
		edges, err := s.GetNodeEdges(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		if edges == nil {
			result[nodeID] = [][2]string{}
		} else {
			result[nodeID] = edges
		}
	}

	return result, nil
}

// UpsertNode inserts or updates a node.
func (s *NetworkXStorage) UpsertNode(ctx context.Context, nodeID string, nodeData map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.nodes[nodeID]; ok {
		// Merge data into existing node
		for k, v := range nodeData {
			existing[k] = v
		}
	} else {
		// Create new node
		newNode := make(map[string]string)
		for k, v := range nodeData {
			newNode[k] = v
		}
		s.nodes[nodeID] = newNode
	}

	s.dirty = true
	return nil
}

// UpsertEdge inserts or updates an edge.
func (s *NetworkXStorage) UpsertEdge(ctx context.Context, sourceNodeID, targetNodeID string, edgeData map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure source node exists
	if _, ok := s.nodes[sourceNodeID]; !ok {
		s.nodes[sourceNodeID] = make(map[string]string)
	}

	// Ensure target node exists
	if _, ok := s.nodes[targetNodeID]; !ok {
		s.nodes[targetNodeID] = make(map[string]string)
	}

	// Create edge map for source if needed
	if _, ok := s.edges[sourceNodeID]; !ok {
		s.edges[sourceNodeID] = make(map[string]map[string]string)
	}

	// Upsert edge
	if existing, ok := s.edges[sourceNodeID][targetNodeID]; ok {
		// Merge data
		for k, v := range edgeData {
			existing[k] = v
		}
	} else {
		newEdge := make(map[string]string)
		for k, v := range edgeData {
			newEdge[k] = v
		}
		s.edges[sourceNodeID][targetNodeID] = newEdge
	}

	s.dirty = true
	return nil
}

// DeleteNode removes a node and all its edges.
func (s *NetworkXStorage) DeleteNode(ctx context.Context, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove the node
	delete(s.nodes, nodeID)

	// Remove all edges from this node
	delete(s.edges, nodeID)

	// Remove all edges to this node
	for source := range s.edges {
		delete(s.edges[source], nodeID)
	}

	s.dirty = true
	return nil
}

// RemoveNodes deletes multiple nodes.
func (s *NetworkXStorage) RemoveNodes(ctx context.Context, nodes []string) error {
	for _, nodeID := range nodes {
		if err := s.DeleteNode(ctx, nodeID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteEdge removes an edge between two nodes.
func (s *NetworkXStorage) DeleteEdge(ctx context.Context, sourceNodeID, targetNodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete in forward direction
	if targets, ok := s.edges[sourceNodeID]; ok {
		delete(targets, targetNodeID)
	}

	// Delete in reverse direction (for undirected graph)
	if targets, ok := s.edges[targetNodeID]; ok {
		delete(targets, sourceNodeID)
	}

	s.dirty = true
	return nil
}

// RemoveEdges deletes multiple edges.
func (s *NetworkXStorage) RemoveEdges(ctx context.Context, edges [][2]string) error {
	for _, edge := range edges {
		if err := s.DeleteEdge(ctx, edge[0], edge[1]); err != nil {
			return err
		}
	}
	return nil
}

// GetKnowledgeGraph retrieves a connected subgraph starting from the given node.
func (s *NetworkXStorage) GetKnowledgeGraph(ctx context.Context, nodeLabel string, maxDepth, maxNodes int) (*base.KnowledgeGraph, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	kg := &base.KnowledgeGraph{
		Nodes: []base.KnowledgeGraphNode{},
		Edges: []base.KnowledgeGraphEdge{},
	}

	// Find the starting node
	if _, exists := s.nodes[nodeLabel]; !exists {
		return kg, nil
	}

	visited := make(map[string]bool)
	edgeSet := make(map[string]bool)
	queue := []struct {
		nodeID string
		depth  int
	}{{nodeLabel, 0}}

	for len(queue) > 0 && len(kg.Nodes) < maxNodes {
		current := queue[0]
		queue = queue[1:]

		if visited[current.nodeID] {
			continue
		}
		visited[current.nodeID] = true

		// Add node
		if nodeData, ok := s.nodes[current.nodeID]; ok {
			kg.Nodes = append(kg.Nodes, base.KnowledgeGraphNode{
				ID:         current.nodeID,
				Properties: nodeData,
			})
		}

		// Stop expanding if at max depth
		if current.depth >= maxDepth {
			continue
		}

		// Add edges and queue neighbors
		if targets, ok := s.edges[current.nodeID]; ok {
			for target, edgeData := range targets {
				edgeKey := current.nodeID + "->" + target
				if !edgeSet[edgeKey] {
					edgeSet[edgeKey] = true
					kg.Edges = append(kg.Edges, base.KnowledgeGraphEdge{
						Source:     current.nodeID,
						Target:     target,
						Properties: edgeData,
					})
				}
				if !visited[target] && len(kg.Nodes) < maxNodes {
					queue = append(queue, struct {
						nodeID string
						depth  int
					}{target, current.depth + 1})
				}
			}
		}
	}

	kg.IsTruncated = len(queue) > 0 || len(kg.Nodes) >= maxNodes
	return kg, nil
}

// GetAllLabels returns all node labels (entity names) in the graph.
func (s *NetworkXStorage) GetAllLabels(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	labels := make([]string, 0, len(s.nodes))
	for id := range s.nodes {
		labels = append(labels, id)
	}
	return labels, nil
}

// GetPopularLabels returns the most popular labels by node degree.
func (s *NetworkXStorage) GetPopularLabels(ctx context.Context, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type nodeDegree struct {
		id     string
		degree int
	}

	degrees := make([]nodeDegree, 0, len(s.nodes))
	for id := range s.nodes {
		degree := 0
		if targets, ok := s.edges[id]; ok {
			degree = len(targets)
		}
		// Also count incoming edges
		for _, targets := range s.edges {
			if _, ok := targets[id]; ok {
				degree++
			}
		}
		degrees = append(degrees, nodeDegree{id: id, degree: degree})
	}

	// Sort by degree descending
	sort.Slice(degrees, func(i, j int) bool {
		return degrees[i].degree > degrees[j].degree
	})

	result := make([]string, 0, limit)
	for i := 0; i < len(degrees) && i < limit; i++ {
		result = append(result, degrees[i].id)
	}
	return result, nil
}

// SearchLabels searches for labels matching the query with fuzzy matching.
func (s *NetworkXStorage) SearchLabels(ctx context.Context, query string, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.ToLower(query)
	var matches []string

	for id := range s.nodes {
		if strings.Contains(strings.ToLower(id), query) {
			matches = append(matches, id)
			if len(matches) >= limit {
				break
			}
		}
	}
	return matches, nil
}

// edgeCount returns the total number of edges.
func (s *NetworkXStorage) edgeCount() int {
	count := 0
	for _, targets := range s.edges {
		count += len(targets)
	}
	return count
}

// NodeCount returns the number of nodes.
func (s *NetworkXStorage) NodeCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.nodes)
}

// EdgeCount returns the number of edges.
func (s *NetworkXStorage) EdgeCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.edgeCount()
}

// AllNodes returns all node IDs.
func (s *NetworkXStorage) AllNodes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]string, 0, len(s.nodes))
	for id := range s.nodes {
		nodes = append(nodes, id)
	}
	return nodes
}

// AllEdges returns all edges as source-target pairs.
func (s *NetworkXStorage) AllEdges() [][2]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	edges := make([][2]string, 0)
	for source, targets := range s.edges {
		for target := range targets {
			edges = append(edges, [2]string{source, target})
		}
	}
	return edges
}

// GetAllNodes returns all nodes with their properties.
func (s *NetworkXStorage) GetAllNodes(ctx context.Context) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]map[string]interface{}, 0, len(s.nodes))
	for id, props := range s.nodes {
		node := map[string]interface{}{
			"id": id,
		}
		for k, v := range props {
			node[k] = v
		}
		result = append(result, node)
	}
	return result, nil
}

// GetAllEdges returns all edges with their properties.
func (s *NetworkXStorage) GetAllEdges(ctx context.Context) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]map[string]interface{}, 0)
	for source, targets := range s.edges {
		for target, props := range targets {
			edge := map[string]interface{}{
				"source": source,
				"target": target,
			}
			for k, v := range props {
				edge[k] = v
			}
			result = append(result, edge)
		}
	}
	return result, nil
}
