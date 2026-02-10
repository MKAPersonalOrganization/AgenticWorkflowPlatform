package graph

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockKVStorage implements BaseKVStorage for testing
type MockKVStorage struct {
	data map[string]map[string]interface{}
}

func NewMockKVStorage() *MockKVStorage {
	return &MockKVStorage{
		data: make(map[string]map[string]interface{}),
	}
}

func (m *MockKVStorage) GetByID(id string) (map[string]interface{}, error) {
	if data, exists := m.data[id]; exists {
		return data, nil
	}
	return nil, nil
}

func (m *MockKVStorage) GetByIDs(ids []string) (map[string]map[string]interface{}, error) {
	result := make(map[string]map[string]interface{})
	for _, id := range ids {
		if data, exists := m.data[id]; exists {
			result[id] = data
		}
	}
	return result, nil
}

func (m *MockKVStorage) Upsert(data map[string]interface{}) error {
	for k, v := range data {
		if mapV, ok := v.(map[string]interface{}); ok {
			m.data[k] = mapV
		}
	}
	return nil
}

func (m *MockKVStorage) Delete(ids []string) error {
	for _, id := range ids {
		delete(m.data, id)
	}
	return nil
}

func (m *MockKVStorage) IndexDoneCallback() error {
	return nil
}

// MockVectorStorage implements BaseVectorStorage for testing
type MockVectorStorage struct {
	data         map[string]map[string]interface{}
	globalConfig map[string]interface{}
}

func NewMockVectorStorage() *MockVectorStorage {
	return &MockVectorStorage{
		data:         make(map[string]map[string]interface{}),
		globalConfig: make(map[string]interface{}),
	}
}

func (m *MockVectorStorage) GetByID(id string) (map[string]interface{}, error) {
	if data, exists := m.data[id]; exists {
		return data, nil
	}
	return nil, nil
}

func (m *MockVectorStorage) Upsert(data map[string]interface{}) error {
	for k, v := range data {
		if mapV, ok := v.(map[string]interface{}); ok {
			m.data[k] = mapV
		}
	}
	return nil
}

func (m *MockVectorStorage) Delete(ids []string) error {
	for _, id := range ids {
		delete(m.data, id)
	}
	return nil
}

func (m *MockVectorStorage) DeleteEntity(entityName string) error {
	// Delete all entries with matching entity_name
	toDelete := []string{}
	for id, data := range m.data {
		if name, ok := data["entity_name"].(string); ok && name == entityName {
			toDelete = append(toDelete, id)
		}
	}
	for _, id := range toDelete {
		delete(m.data, id)
	}
	return nil
}

func (m *MockVectorStorage) DeleteEntityRelation(entityName string) error {
	// Delete all relations involving the entity
	toDelete := []string{}
	for id, data := range m.data {
		if src, ok := data["src_id"].(string); ok && src == entityName {
			toDelete = append(toDelete, id)
		} else if tgt, ok := data["tgt_id"].(string); ok && tgt == entityName {
			toDelete = append(toDelete, id)
		}
	}
	for _, id := range toDelete {
		delete(m.data, id)
	}
	return nil
}

func (m *MockVectorStorage) IndexDoneCallback() error {
	return nil
}

func (m *MockVectorStorage) GetGlobalConfig() map[string]interface{} {
	return m.globalConfig
}

// MockGraphStorage implements BaseGraphStorage for testing
type MockGraphStorage struct {
	nodes map[string]map[string]interface{}
	edges map[string]map[string]interface{} // key: "src|tgt"
}

func NewMockGraphStorage() *MockGraphStorage {
	return &MockGraphStorage{
		nodes: make(map[string]map[string]interface{}),
		edges: make(map[string]map[string]interface{}),
	}
}

func (m *MockGraphStorage) edgeKey(src, tgt string) string {
	if src > tgt {
		src, tgt = tgt, src
	}
	return fmt.Sprintf("%s|%s", src, tgt)
}

func (m *MockGraphStorage) HasNode(nodeName string) (bool, error) {
	_, exists := m.nodes[nodeName]
	return exists, nil
}

func (m *MockGraphStorage) HasEdge(srcName, tgtName string) (bool, error) {
	key := m.edgeKey(srcName, tgtName)
	_, exists := m.edges[key]
	return exists, nil
}

func (m *MockGraphStorage) GetNode(nodeName string) (map[string]interface{}, error) {
	if data, exists := m.nodes[nodeName]; exists {
		return data, nil
	}
	return nil, nil
}

func (m *MockGraphStorage) GetEdge(srcName, tgtName string) (map[string]interface{}, error) {
	key := m.edgeKey(srcName, tgtName)
	if data, exists := m.edges[key]; exists {
		return data, nil
	}
	return nil, nil
}

func (m *MockGraphStorage) GetNodeEdges(nodeName string) ([][2]string, error) {
	edges := make([][2]string, 0)
	for key := range m.edges {
		var src, tgt string
		fmt.Sscanf(key, "%s|%s", &src, &tgt)
		// Parse the key manually
		for i := 0; i < len(key); i++ {
			if key[i] == '|' {
				src = key[:i]
				tgt = key[i+1:]
				break
			}
		}
		if src == nodeName || tgt == nodeName {
			edges = append(edges, [2]string{src, tgt})
		}
	}
	return edges, nil
}

func (m *MockGraphStorage) UpsertNode(nodeName string, nodeData map[string]interface{}) error {
	m.nodes[nodeName] = nodeData
	return nil
}

func (m *MockGraphStorage) UpsertEdge(srcName, tgtName string, edgeData map[string]interface{}) error {
	key := m.edgeKey(srcName, tgtName)
	m.edges[key] = edgeData
	return nil
}

func (m *MockGraphStorage) DeleteNode(nodeName string) error {
	delete(m.nodes, nodeName)
	// Also delete all edges involving this node
	toDelete := []string{}
	for key := range m.edges {
		var src, tgt string
		for i := 0; i < len(key); i++ {
			if key[i] == '|' {
				src = key[:i]
				tgt = key[i+1:]
				break
			}
		}
		if src == nodeName || tgt == nodeName {
			toDelete = append(toDelete, key)
		}
	}
	for _, key := range toDelete {
		delete(m.edges, key)
	}
	return nil
}

func (m *MockGraphStorage) RemoveEdges(edges [][2]string) error {
	for _, edge := range edges {
		key := m.edgeKey(edge[0], edge[1])
		delete(m.edges, key)
	}
	return nil
}

func (m *MockGraphStorage) IndexDoneCallback() error {
	return nil
}

// Test helper to create a GraphOperator with mocks
func newTestGraphOperator() *GraphOperator {
	return NewGraphOperator(
		NewMockGraphStorage(),
		NewMockVectorStorage(),
		NewMockVectorStorage(),
		NewMockKVStorage(),
		NewMockKVStorage(),
		"test-workspace",
	)
}

func TestComputeMDHashID(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		prefix   string
		expected string
	}{
		{
			name:    "with prefix",
			content: "test content",
			prefix:  "ent-",
		},
		{
			name:    "without prefix",
			content: "test content",
			prefix:  "",
		},
		{
			name:    "empty content",
			content: "",
			prefix:  "rel-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeMDHashID(tt.content, tt.prefix)
			assert.NotEmpty(t, result)
			if tt.prefix != "" {
				assert.True(t, len(result) > len(tt.prefix))
				assert.Equal(t, tt.prefix, result[:len(tt.prefix)])
			}
			// Same input should produce same output
			result2 := ComputeMDHashID(tt.content, tt.prefix)
			assert.Equal(t, result, result2)
		})
	}
}

func TestMakeRelationChunkKey(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		tgt      string
		expected string
	}{
		{
			name:     "alphabetical order",
			src:      "Alice",
			tgt:      "Bob",
			expected: "Alice|Bob",
		},
		{
			name:     "reverse alphabetical order",
			src:      "Zebra",
			tgt:      "Apple",
			expected: "Apple|Zebra",
		},
		{
			name:     "same entity",
			src:      "Entity",
			tgt:      "Entity",
			expected: "Entity|Entity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MakeRelationChunkKey(tt.src, tt.tgt)
			assert.Equal(t, tt.expected, result)

			// Test symmetry
			result2 := MakeRelationChunkKey(tt.tgt, tt.src)
			assert.Equal(t, result, result2)
		})
	}
}

func TestDeletionResult(t *testing.T) {
	t.Run("success result", func(t *testing.T) {
		result := DeletionResult{
			Status:     "success",
			DocID:      "entity1",
			Message:    "Deleted successfully",
			StatusCode: 200,
		}
		assert.Equal(t, "success", result.Status)
		assert.Equal(t, 200, result.StatusCode)
	})

	t.Run("not found result", func(t *testing.T) {
		result := DeletionResult{
			Status:     "not_found",
			DocID:      "entity2",
			Message:    "Entity not found",
			StatusCode: 404,
		}
		assert.Equal(t, "not_found", result.Status)
		assert.Equal(t, 404, result.StatusCode)
	})
}

func TestGraphOperator_DeleteByEntity(t *testing.T) {
	t.Run("delete existing entity", func(t *testing.T) {
		op := newTestGraphOperator()

		// Setup: Create an entity
		graph := op.Graph.(*MockGraphStorage)
		graph.nodes["TestEntity"] = map[string]interface{}{
			"entity_id":   "TestEntity",
			"description": "Test description",
		}

		result := op.DeleteByEntity("TestEntity")
		assert.Equal(t, "success", result.Status)
		assert.Equal(t, 200, result.StatusCode)

		// Verify entity is deleted
		exists, _ := op.Graph.HasNode("TestEntity")
		assert.False(t, exists)
	})

	t.Run("delete non-existing entity", func(t *testing.T) {
		op := newTestGraphOperator()

		result := op.DeleteByEntity("NonExistent")
		assert.Equal(t, "not_found", result.Status)
		assert.Equal(t, 404, result.StatusCode)
	})

	t.Run("delete entity with relations", func(t *testing.T) {
		op := newTestGraphOperator()

		// Setup: Create entities and a relation
		graph := op.Graph.(*MockGraphStorage)
		graph.nodes["Entity1"] = map[string]interface{}{"entity_id": "Entity1"}
		graph.nodes["Entity2"] = map[string]interface{}{"entity_id": "Entity2"}
		graph.edges["Entity1|Entity2"] = map[string]interface{}{"description": "test relation"}

		result := op.DeleteByEntity("Entity1")
		assert.Equal(t, "success", result.Status)

		// Verify entity and its edges are deleted
		exists, _ := op.Graph.HasNode("Entity1")
		assert.False(t, exists)

		edgeExists, _ := op.Graph.HasEdge("Entity1", "Entity2")
		assert.False(t, edgeExists)
	})
}

func TestGraphOperator_DeleteByRelation(t *testing.T) {
	t.Run("delete existing relation", func(t *testing.T) {
		op := newTestGraphOperator()

		// Setup: Create a relation
		graph := op.Graph.(*MockGraphStorage)
		graph.nodes["Entity1"] = map[string]interface{}{"entity_id": "Entity1"}
		graph.nodes["Entity2"] = map[string]interface{}{"entity_id": "Entity2"}
		graph.edges["Entity1|Entity2"] = map[string]interface{}{"description": "test relation"}

		result := op.DeleteByRelation("Entity1", "Entity2")
		assert.Equal(t, "success", result.Status)
		assert.Equal(t, 200, result.StatusCode)

		// Verify relation is deleted but nodes remain
		edgeExists, _ := op.Graph.HasEdge("Entity1", "Entity2")
		assert.False(t, edgeExists)

		exists1, _ := op.Graph.HasNode("Entity1")
		assert.True(t, exists1)

		exists2, _ := op.Graph.HasNode("Entity2")
		assert.True(t, exists2)
	})

	t.Run("delete non-existing relation", func(t *testing.T) {
		op := newTestGraphOperator()

		result := op.DeleteByRelation("Entity1", "Entity2")
		assert.Equal(t, "not_found", result.Status)
		assert.Equal(t, 404, result.StatusCode)
	})
}

func TestGraphOperator_CreateEntity(t *testing.T) {
	t.Run("create new entity", func(t *testing.T) {
		op := newTestGraphOperator()

		entityData := map[string]interface{}{
			"description": "A test entity",
			"entity_type": "PERSON",
		}

		result, err := op.CreateEntity("TestPerson", entityData)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "TestPerson", result.EntityName)

		// Verify entity was created
		exists, _ := op.Graph.HasNode("TestPerson")
		assert.True(t, exists)
	})

	t.Run("create duplicate entity", func(t *testing.T) {
		op := newTestGraphOperator()

		// Create first entity
		_, err := op.CreateEntity("TestEntity", map[string]interface{}{})
		require.NoError(t, err)

		// Try to create duplicate
		_, err = op.CreateEntity("TestEntity", map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestGraphOperator_CreateRelation(t *testing.T) {
	t.Run("create new relation", func(t *testing.T) {
		op := newTestGraphOperator()

		// Setup: Create entities first
		_, err := op.CreateEntity("Entity1", map[string]interface{}{"entity_type": "PERSON"})
		require.NoError(t, err)
		_, err = op.CreateEntity("Entity2", map[string]interface{}{"entity_type": "ORGANIZATION"})
		require.NoError(t, err)

		relationData := map[string]interface{}{
			"description": "works at",
			"keywords":    "employment",
		}

		result, err := op.CreateRelation("Entity1", "Entity2", relationData)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify relation was created
		exists, _ := op.Graph.HasEdge("Entity1", "Entity2")
		assert.True(t, exists)
	})

	t.Run("create relation with missing source entity", func(t *testing.T) {
		op := newTestGraphOperator()

		_, err := op.CreateEntity("Entity2", map[string]interface{}{})
		require.NoError(t, err)

		_, err = op.CreateRelation("NonExistent", "Entity2", map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("create duplicate relation", func(t *testing.T) {
		op := newTestGraphOperator()

		// Setup
		_, _ = op.CreateEntity("E1", map[string]interface{}{})
		_, _ = op.CreateEntity("E2", map[string]interface{}{})
		_, err := op.CreateRelation("E1", "E2", map[string]interface{}{})
		require.NoError(t, err)

		// Try to create duplicate
		_, err = op.CreateRelation("E1", "E2", map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestGraphOperator_EditEntity(t *testing.T) {
	t.Run("edit entity description", func(t *testing.T) {
		op := newTestGraphOperator()

		// Create entity
		_, err := op.CreateEntity("TestEntity", map[string]interface{}{
			"description": "Original description",
			"entity_type": "PERSON",
		})
		require.NoError(t, err)

		// Edit entity
		updatedData := map[string]string{
			"description": "Updated description",
		}
		result, err := op.EditEntity("TestEntity", updatedData, false, false)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify update
		nodeData, _ := op.Graph.GetNode("TestEntity")
		assert.Equal(t, "Updated description", nodeData["description"])
	})

	t.Run("edit non-existing entity", func(t *testing.T) {
		op := newTestGraphOperator()

		_, err := op.EditEntity("NonExistent", map[string]string{"description": "test"}, false, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("rename entity without permission", func(t *testing.T) {
		op := newTestGraphOperator()

		_, _ = op.CreateEntity("OldName", map[string]interface{}{})

		_, err := op.EditEntity("OldName", map[string]string{"entity_name": "NewName"}, false, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "renaming is not allowed")
	})

	t.Run("rename entity with permission", func(t *testing.T) {
		op := newTestGraphOperator()

		_, _ = op.CreateEntity("OldName", map[string]interface{}{
			"description": "Test entity",
		})

		result, err := op.EditEntity("OldName", map[string]string{"entity_name": "NewName"}, true, false)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify old name no longer exists
		exists, _ := op.Graph.HasNode("OldName")
		assert.False(t, exists)

		// Verify new name exists
		exists, _ = op.Graph.HasNode("NewName")
		assert.True(t, exists)
	})
}

func TestGraphOperator_EditRelation(t *testing.T) {
	t.Run("edit relation description", func(t *testing.T) {
		op := newTestGraphOperator()

		// Setup
		_, _ = op.CreateEntity("E1", map[string]interface{}{})
		_, _ = op.CreateEntity("E2", map[string]interface{}{})
		_, _ = op.CreateRelation("E1", "E2", map[string]interface{}{
			"description": "Original",
		})

		// Edit relation
		result, err := op.EditRelation("E1", "E2", map[string]interface{}{
			"description": "Updated",
		})
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify update
		edgeData, _ := op.Graph.GetEdge("E1", "E2")
		assert.Equal(t, "Updated", edgeData["description"])
	})

	t.Run("edit non-existing relation", func(t *testing.T) {
		op := newTestGraphOperator()

		_, err := op.EditRelation("E1", "E2", map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})
}

func TestGraphOperator_MergeEntities(t *testing.T) {
	t.Run("merge two entities", func(t *testing.T) {
		op := newTestGraphOperator()

		// Create source entities
		_, _ = op.CreateEntity("Entity1", map[string]interface{}{
			"description": "First entity",
			"entity_type": "PERSON",
		})
		_, _ = op.CreateEntity("Entity2", map[string]interface{}{
			"description": "Second entity",
			"entity_type": "PERSON",
		})

		// Merge entities
		result, err := op.MergeEntities([]string{"Entity1", "Entity2"}, "MergedEntity", nil, nil)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "MergedEntity", result.EntityName)

		// Verify source entities are deleted
		exists1, _ := op.Graph.HasNode("Entity1")
		assert.False(t, exists1)

		exists2, _ := op.Graph.HasNode("Entity2")
		assert.False(t, exists2)

		// Verify merged entity exists
		exists, _ := op.Graph.HasNode("MergedEntity")
		assert.True(t, exists)
	})

	t.Run("merge into existing entity", func(t *testing.T) {
		op := newTestGraphOperator()

		// Create entities
		_, _ = op.CreateEntity("Source", map[string]interface{}{
			"description": "Source entity",
		})
		_, _ = op.CreateEntity("Target", map[string]interface{}{
			"description": "Target entity",
		})

		// Merge Source into Target
		result, err := op.MergeEntities([]string{"Source"}, "Target", nil, nil)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify Source is deleted
		existsSource, _ := op.Graph.HasNode("Source")
		assert.False(t, existsSource)

		// Verify Target still exists
		existsTarget, _ := op.Graph.HasNode("Target")
		assert.True(t, existsTarget)
	})

	t.Run("merge non-existing entity", func(t *testing.T) {
		op := newTestGraphOperator()

		_, err := op.MergeEntities([]string{"NonExistent"}, "Target", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})
}

func TestGraphOperator_GetEntityInfo(t *testing.T) {
	t.Run("get existing entity info", func(t *testing.T) {
		op := newTestGraphOperator()

		_, _ = op.CreateEntity("TestEntity", map[string]interface{}{
			"description": "Test description",
			"entity_type": "PERSON",
			"source_id":   "chunk1",
		})

		info, err := op.GetEntityInfo("TestEntity", true)
		require.NoError(t, err)
		require.NotNil(t, info)

		assert.Equal(t, "TestEntity", info.EntityName)
		assert.NotNil(t, info.GraphData)
	})

	t.Run("get non-existing entity info", func(t *testing.T) {
		op := newTestGraphOperator()

		info, err := op.GetEntityInfo("NonExistent", false)
		require.NoError(t, err)

		assert.Equal(t, "NonExistent", info.EntityName)
		assert.Nil(t, info.GraphData)
	})
}

func TestGraphOperator_GetRelationInfo(t *testing.T) {
	t.Run("get existing relation info", func(t *testing.T) {
		op := newTestGraphOperator()

		_, _ = op.CreateEntity("E1", map[string]interface{}{})
		_, _ = op.CreateEntity("E2", map[string]interface{}{})
		_, _ = op.CreateRelation("E1", "E2", map[string]interface{}{
			"description": "Test relation",
			"keywords":    "test",
		})

		info, err := op.GetRelationInfo("E1", "E2", true)
		require.NoError(t, err)
		require.NotNil(t, info)

		assert.Equal(t, "E1", info.SrcEntity)
		assert.Equal(t, "E2", info.TgtEntity)
		assert.NotNil(t, info.GraphData)
	})
}

func TestMergeAttributes(t *testing.T) {
	t.Run("concatenate strategy", func(t *testing.T) {
		dataList := []map[string]interface{}{
			{"description": "First"},
			{"description": "Second"},
		}
		strategy := map[string]MergeStrategy{
			"description": MergeStrategyConcatenate,
		}

		result := mergeAttributes(dataList, strategy, false)
		assert.Contains(t, result["description"], "First")
		assert.Contains(t, result["description"], "Second")
	})

	t.Run("keep first strategy", func(t *testing.T) {
		dataList := []map[string]interface{}{
			{"type": "First"},
			{"type": "Second"},
		}
		strategy := map[string]MergeStrategy{
			"type": MergeStrategyKeepFirst,
		}

		result := mergeAttributes(dataList, strategy, false)
		assert.Equal(t, "First", result["type"])
	})

	t.Run("keep last strategy", func(t *testing.T) {
		dataList := []map[string]interface{}{
			{"type": "First"},
			{"type": "Second"},
		}
		strategy := map[string]MergeStrategy{
			"type": MergeStrategyKeepLast,
		}

		result := mergeAttributes(dataList, strategy, false)
		assert.Equal(t, "Second", result["type"])
	})

	t.Run("join unique strategy", func(t *testing.T) {
		dataList := []map[string]interface{}{
			{"source_id": "chunk1<SEP>chunk2"},
			{"source_id": "chunk2<SEP>chunk3"},
		}
		strategy := map[string]MergeStrategy{
			"source_id": MergeStrategyJoinUnique,
		}

		result := mergeAttributes(dataList, strategy, false)
		sourceID := result["source_id"].(string)
		assert.Contains(t, sourceID, "chunk1")
		assert.Contains(t, sourceID, "chunk2")
		assert.Contains(t, sourceID, "chunk3")
	})

	t.Run("max strategy", func(t *testing.T) {
		dataList := []map[string]interface{}{
			{"weight": 0.5},
			{"weight": 0.8},
			{"weight": 0.3},
		}
		strategy := map[string]MergeStrategy{
			"weight": MergeStrategyMax,
		}

		result := mergeAttributes(dataList, strategy, false)
		assert.Equal(t, 0.8, result["weight"])
	})

	t.Run("filter none only", func(t *testing.T) {
		dataList := []map[string]interface{}{
			{"value": ""},
			{"value": "non-empty"},
		}
		strategy := map[string]MergeStrategy{
			"value": MergeStrategyKeepFirst,
		}

		// With filterNoneOnly=false, empty string is filtered
		result := mergeAttributes(dataList, strategy, false)
		assert.Equal(t, "non-empty", result["value"])

		// With filterNoneOnly=true, empty string is kept
		result2 := mergeAttributes(dataList, strategy, true)
		assert.Equal(t, "", result2["value"])
	})
}

func TestSplitSourceID(t *testing.T) {
	t.Run("split source ID", func(t *testing.T) {
		result := splitSourceID("chunk1<SEP>chunk2<SEP>chunk3")
		assert.Len(t, result, 3)
		assert.Contains(t, result, "chunk1")
		assert.Contains(t, result, "chunk2")
		assert.Contains(t, result, "chunk3")
	})

	t.Run("empty source ID", func(t *testing.T) {
		result := splitSourceID("")
		assert.Nil(t, result)
	})

	t.Run("single chunk", func(t *testing.T) {
		result := splitSourceID("chunk1")
		assert.Len(t, result, 1)
		assert.Equal(t, "chunk1", result[0])
	})
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{"nil", nil, false},
		{"empty string", "", false},
		{"non-empty string", "hello", true},
		{"zero int", 0, false},
		{"non-zero int", 42, true},
		{"zero float", 0.0, false},
		{"non-zero float", 3.14, true},
		{"false bool", false, false},
		{"true bool", true, true},
		{"empty slice", []interface{}{}, false},
		{"non-empty slice", []interface{}{1}, true},
		{"empty map", map[string]interface{}{}, false},
		{"non-empty map", map[string]interface{}{"key": "value"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTruthy(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected float64
		ok       bool
	}{
		{"float64", 3.14, 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", 42, 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"int32", int32(50), 50.0, true},
		{"string", "not a number", 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toFloat64(tt.value)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestComputeIncrementalChunkIDs(t *testing.T) {
	t.Run("basic incremental update", func(t *testing.T) {
		existing := []string{"chunk1", "chunk2", "chunk3"}
		old := []string{"chunk2"}
		newIDs := []string{"chunk4"}

		result := ComputeIncrementalChunkIDs(existing, old, newIDs)

		// chunk2 should be removed (it was in old)
		// chunk1, chunk3 should remain
		// chunk4 should be added
		assert.Contains(t, result, "chunk1")
		assert.NotContains(t, result, "chunk2")
		assert.Contains(t, result, "chunk3")
		assert.Contains(t, result, "chunk4")
	})

	t.Run("empty inputs", func(t *testing.T) {
		result := ComputeIncrementalChunkIDs(nil, nil, nil)
		assert.Empty(t, result)
	})

	t.Run("deduplication", func(t *testing.T) {
		existing := []string{"chunk1", "chunk1", "chunk2"}
		old := []string{}
		newIDs := []string{"chunk1", "chunk3"}

		result := ComputeIncrementalChunkIDs(existing, old, newIDs)

		// Count occurrences of chunk1
		count := 0
		for _, id := range result {
			if id == "chunk1" {
				count++
			}
		}
		assert.Equal(t, 1, count) // Should only appear once
	})
}

func TestOperationSummary(t *testing.T) {
	t.Run("success operation summary", func(t *testing.T) {
		summary := &OperationSummary{
			Merged:          false,
			MergeStatus:     "not_attempted",
			MergeError:      nil,
			OperationStatus: "success",
			TargetEntity:    nil,
			FinalEntity:     "TestEntity",
			Renamed:         false,
		}

		assert.Equal(t, "success", summary.OperationStatus)
		assert.False(t, summary.Merged)
		assert.Nil(t, summary.MergeError)
	})

	t.Run("merge success operation summary", func(t *testing.T) {
		targetEntity := "MergedEntity"
		summary := &OperationSummary{
			Merged:          true,
			MergeStatus:     "success",
			MergeError:      nil,
			OperationStatus: "success",
			TargetEntity:    &targetEntity,
			FinalEntity:     "MergedEntity",
			Renamed:         true,
		}

		assert.True(t, summary.Merged)
		assert.Equal(t, "success", summary.MergeStatus)
		assert.NotNil(t, summary.TargetEntity)
	})

	t.Run("partial success operation summary", func(t *testing.T) {
		errMsg := "merge failed"
		summary := &OperationSummary{
			Merged:          false,
			MergeStatus:     "failed",
			MergeError:      &errMsg,
			OperationStatus: "partial_success",
			FinalEntity:     "OriginalEntity",
			Renamed:         false,
		}

		assert.Equal(t, "partial_success", summary.OperationStatus)
		assert.NotNil(t, summary.MergeError)
		assert.Equal(t, "merge failed", *summary.MergeError)
	})
}

func TestEntityInfo(t *testing.T) {
	t.Run("entity info with vector data", func(t *testing.T) {
		info := &EntityInfo{
			EntityName: "TestEntity",
			SourceID:   "chunk1<SEP>chunk2",
			GraphData: map[string]interface{}{
				"description": "Test description",
				"entity_type": "PERSON",
			},
			VectorData: map[string]interface{}{
				"content": "TestEntity\nTest description",
			},
		}

		assert.Equal(t, "TestEntity", info.EntityName)
		assert.NotNil(t, info.GraphData)
		assert.NotNil(t, info.VectorData)
	})
}

func TestRelationInfo(t *testing.T) {
	t.Run("relation info", func(t *testing.T) {
		info := &RelationInfo{
			SrcEntity: "Entity1",
			TgtEntity: "Entity2",
			SourceID:  "chunk1",
			GraphData: map[string]interface{}{
				"description": "works at",
				"keywords":    "employment",
			},
		}

		assert.Equal(t, "Entity1", info.SrcEntity)
		assert.Equal(t, "Entity2", info.TgtEntity)
		assert.NotNil(t, info.GraphData)
	})
}

func TestChunkTrackingData(t *testing.T) {
	t.Run("chunk tracking data", func(t *testing.T) {
		data := ChunkTrackingData{
			ChunkIDs: []string{"chunk1", "chunk2", "chunk3"},
			Count:    3,
		}

		assert.Len(t, data.ChunkIDs, 3)
		assert.Equal(t, 3, data.Count)
	})
}
