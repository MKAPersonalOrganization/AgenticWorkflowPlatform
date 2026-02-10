package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGPTKeywordExtractionFormat(t *testing.T) {
	format := GPTKeywordExtractionFormat{
		HighLevelKeywords: []string{"machine learning", "artificial intelligence"},
		LowLevelKeywords:  []string{"neural networks", "deep learning", "backpropagation"},
	}

	assert.Len(t, format.HighLevelKeywords, 2)
	assert.Len(t, format.LowLevelKeywords, 3)
	assert.Equal(t, "machine learning", format.HighLevelKeywords[0])
}

func TestGPTKeywordExtractionFormatJSON(t *testing.T) {
	format := GPTKeywordExtractionFormat{
		HighLevelKeywords: []string{"AI", "ML"},
		LowLevelKeywords:  []string{"CNN", "RNN"},
	}

	data, err := json.Marshal(format)
	assert.NoError(t, err)

	var decoded GPTKeywordExtractionFormat
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, format, decoded)
}

func TestKnowledgeGraphNode(t *testing.T) {
	node := KnowledgeGraphNode{
		ID:          "node-1",
		Name:        "TestNode",
		Type:        "Person",
		Description: "A test person node",
		Properties: map[string]interface{}{
			"age":  30,
			"city": "NYC",
		},
	}

	assert.Equal(t, "node-1", node.ID)
	assert.Equal(t, "TestNode", node.Name)
	assert.Equal(t, "Person", node.Type)
	assert.Equal(t, 30, node.Properties["age"])
}

func TestKnowledgeGraphEdge(t *testing.T) {
	edge := KnowledgeGraphEdge{
		Source:      "node-1",
		Target:      "node-2",
		Type:        "KNOWS",
		Description: "Person 1 knows Person 2",
		Weight:      0.8,
		Properties: map[string]interface{}{
			"since": "2020",
		},
	}

	assert.Equal(t, "node-1", edge.Source)
	assert.Equal(t, "node-2", edge.Target)
	assert.Equal(t, "KNOWS", edge.Type)
	assert.Equal(t, 0.8, edge.Weight)
}

func TestNewKnowledgeGraph(t *testing.T) {
	kg := NewKnowledgeGraph()

	assert.NotNil(t, kg)
	assert.Empty(t, kg.Nodes)
	assert.Empty(t, kg.Edges)
	assert.Equal(t, 0, kg.NodeCount())
	assert.Equal(t, 0, kg.EdgeCount())
}

func TestKnowledgeGraphAddNode(t *testing.T) {
	kg := NewKnowledgeGraph()

	node := KnowledgeGraphNode{
		ID:   "n1",
		Name: "Node 1",
		Type: "Entity",
	}

	kg.AddNode(node)

	assert.Equal(t, 1, kg.NodeCount())
	assert.Equal(t, "n1", kg.Nodes[0].ID)
}

func TestKnowledgeGraphAddEdge(t *testing.T) {
	kg := NewKnowledgeGraph()

	edge := KnowledgeGraphEdge{
		Source: "n1",
		Target: "n2",
		Type:   "RELATES_TO",
		Weight: 1.0,
	}

	kg.AddEdge(edge)

	assert.Equal(t, 1, kg.EdgeCount())
	assert.Equal(t, "n1", kg.Edges[0].Source)
}

func TestKnowledgeGraphGetNode(t *testing.T) {
	kg := NewKnowledgeGraph()

	kg.AddNode(KnowledgeGraphNode{ID: "n1", Name: "Node 1"})
	kg.AddNode(KnowledgeGraphNode{ID: "n2", Name: "Node 2"})

	// Find existing node
	node := kg.GetNode("n1")
	assert.NotNil(t, node)
	assert.Equal(t, "Node 1", node.Name)

	// Non-existing node
	node = kg.GetNode("n3")
	assert.Nil(t, node)
}

func TestKnowledgeGraphGetNodeByName(t *testing.T) {
	kg := NewKnowledgeGraph()

	kg.AddNode(KnowledgeGraphNode{ID: "n1", Name: "Alpha"})
	kg.AddNode(KnowledgeGraphNode{ID: "n2", Name: "Beta"})

	node := kg.GetNodeByName("Alpha")
	assert.NotNil(t, node)
	assert.Equal(t, "n1", node.ID)

	node = kg.GetNodeByName("Gamma")
	assert.Nil(t, node)
}

func TestKnowledgeGraphGetEdge(t *testing.T) {
	kg := NewKnowledgeGraph()

	kg.AddEdge(KnowledgeGraphEdge{Source: "n1", Target: "n2", Type: "KNOWS"})
	kg.AddEdge(KnowledgeGraphEdge{Source: "n2", Target: "n3", Type: "OWNS"})

	edge := kg.GetEdge("n1", "n2")
	assert.NotNil(t, edge)
	assert.Equal(t, "KNOWS", edge.Type)

	edge = kg.GetEdge("n1", "n3")
	assert.Nil(t, edge)
}

func TestKnowledgeGraphGetNodeNeighbors(t *testing.T) {
	kg := NewKnowledgeGraph()

	kg.AddNode(KnowledgeGraphNode{ID: "n1", Name: "Center"})
	kg.AddNode(KnowledgeGraphNode{ID: "n2", Name: "Neighbor1"})
	kg.AddNode(KnowledgeGraphNode{ID: "n3", Name: "Neighbor2"})
	kg.AddNode(KnowledgeGraphNode{ID: "n4", Name: "Isolated"})

	kg.AddEdge(KnowledgeGraphEdge{Source: "n1", Target: "n2"})
	kg.AddEdge(KnowledgeGraphEdge{Source: "n3", Target: "n1"})

	neighbors := kg.GetNodeNeighbors("n1")
	assert.Len(t, neighbors, 2)

	// n4 should have no neighbors
	neighbors = kg.GetNodeNeighbors("n4")
	assert.Len(t, neighbors, 0)
}

func TestKnowledgeGraphToJSON(t *testing.T) {
	kg := NewKnowledgeGraph()

	kg.AddNode(KnowledgeGraphNode{ID: "n1", Name: "Node1", Type: "Entity"})
	kg.AddEdge(KnowledgeGraphEdge{Source: "n1", Target: "n2", Type: "RELATES"})

	data, err := kg.ToJSON()
	assert.NoError(t, err)
	assert.Contains(t, string(data), "Node1")
	assert.Contains(t, string(data), "RELATES")
}

func TestKnowledgeGraphFromJSON(t *testing.T) {
	jsonData := `{
		"nodes": [
			{"id": "n1", "name": "TestNode", "type": "Entity", "description": ""}
		],
		"edges": [
			{"source": "n1", "target": "n2", "type": "KNOWS", "description": "", "weight": 0.5}
		]
	}`

	kg := NewKnowledgeGraph()
	err := kg.FromJSON([]byte(jsonData))

	assert.NoError(t, err)
	assert.Equal(t, 1, kg.NodeCount())
	assert.Equal(t, 1, kg.EdgeCount())
	assert.Equal(t, "TestNode", kg.Nodes[0].Name)
	assert.Equal(t, 0.5, kg.Edges[0].Weight)
}

func TestKnowledgeGraphMerge(t *testing.T) {
	kg1 := NewKnowledgeGraph()
	kg1.AddNode(KnowledgeGraphNode{ID: "n1", Name: "Node1"})
	kg1.AddNode(KnowledgeGraphNode{ID: "n2", Name: "Node2"})
	kg1.AddEdge(KnowledgeGraphEdge{Source: "n1", Target: "n2"})

	kg2 := NewKnowledgeGraph()
	kg2.AddNode(KnowledgeGraphNode{ID: "n2", Name: "Node2Dup"}) // Duplicate ID
	kg2.AddNode(KnowledgeGraphNode{ID: "n3", Name: "Node3"})
	kg2.AddEdge(KnowledgeGraphEdge{Source: "n1", Target: "n2"}) // Duplicate edge
	kg2.AddEdge(KnowledgeGraphEdge{Source: "n2", Target: "n3"})

	kg1.Merge(kg2)

	// Should have 3 nodes (n2 not duplicated)
	assert.Equal(t, 3, kg1.NodeCount())
	// Should have 2 edges (n1->n2 not duplicated)
	assert.Equal(t, 2, kg1.EdgeCount())
}

func TestKnowledgeGraphClear(t *testing.T) {
	kg := NewKnowledgeGraph()

	kg.AddNode(KnowledgeGraphNode{ID: "n1"})
	kg.AddEdge(KnowledgeGraphEdge{Source: "n1", Target: "n2"})

	assert.Equal(t, 1, kg.NodeCount())
	assert.Equal(t, 1, kg.EdgeCount())

	kg.Clear()

	assert.Equal(t, 0, kg.NodeCount())
	assert.Equal(t, 0, kg.EdgeCount())
}

func TestQueryResult(t *testing.T) {
	result := QueryResult{
		Answer:     "This is the answer",
		QueryMode:  "hybrid",
		Successful: true,
		Keywords:   []string{"keyword1", "keyword2"},
		Sources: []SourceReference{
			{ChunkID: "c1", DocID: "d1", Content: "Source content", Score: 0.95},
		},
	}

	assert.Equal(t, "This is the answer", result.Answer)
	assert.True(t, result.Successful)
	assert.Len(t, result.Sources, 1)
	assert.Equal(t, 0.95, result.Sources[0].Score)
}

func TestInsertResult(t *testing.T) {
	result := InsertResult{
		DocID:       "doc-123",
		ChunkCount:  10,
		EntityCount: 25,
		EdgeCount:   15,
		Successful:  true,
	}

	assert.Equal(t, "doc-123", result.DocID)
	assert.Equal(t, 10, result.ChunkCount)
	assert.True(t, result.Successful)
}

func TestEntityInfo(t *testing.T) {
	entity := EntityInfo{
		Name:        "OpenAI",
		Type:        "Organization",
		Description: "An AI research company",
		SourceIDs:   []string{"src1", "src2"},
		ChunkIDs:    []string{"c1", "c2", "c3"},
		Occurrences: 5,
	}

	assert.Equal(t, "OpenAI", entity.Name)
	assert.Len(t, entity.SourceIDs, 2)
	assert.Equal(t, 5, entity.Occurrences)
}

func TestRelationInfo(t *testing.T) {
	relation := RelationInfo{
		SourceEntity: "OpenAI",
		TargetEntity: "ChatGPT",
		RelationType: "DEVELOPED",
		Description:  "OpenAI developed ChatGPT",
		Weight:       1.0,
		Keywords:     []string{"AI", "development"},
	}

	assert.Equal(t, "OpenAI", relation.SourceEntity)
	assert.Equal(t, "ChatGPT", relation.TargetEntity)
	assert.Equal(t, "DEVELOPED", relation.RelationType)
}

func TestChunkInfo(t *testing.T) {
	chunk := ChunkInfo{
		ChunkID:       "chunk-001",
		DocID:         "doc-001",
		Content:       "This is the chunk content...",
		TokenCount:    50,
		StartPos:      0,
		EndPos:        100,
		Entities:      []string{"Entity1", "Entity2"},
		ChunkOrderIdx: 0,
	}

	assert.Equal(t, "chunk-001", chunk.ChunkID)
	assert.Equal(t, 50, chunk.TokenCount)
	assert.Len(t, chunk.Entities, 2)
}

func TestGraphStatistics(t *testing.T) {
	stats := GraphStatistics{
		TotalNodes:     100,
		TotalEdges:     250,
		TotalChunks:    50,
		TotalDocuments: 5,
		NodesByType: map[string]int{
			"Person":       40,
			"Organization": 30,
			"Location":     30,
		},
		EdgesByType: map[string]int{
			"KNOWS":      100,
			"LOCATED_IN": 150,
		},
		AverageDegree:  5.0,
		ConnectedComps: 3,
	}

	assert.Equal(t, 100, stats.TotalNodes)
	assert.Equal(t, 250, stats.TotalEdges)
	assert.Equal(t, 40, stats.NodesByType["Person"])
	assert.Equal(t, 5.0, stats.AverageDegree)
}
