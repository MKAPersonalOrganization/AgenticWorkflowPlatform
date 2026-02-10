package namespace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNamespaceConstants(t *testing.T) {
	// Test KV store namespaces
	assert.Equal(t, "full_docs", KVStoreFullDocs)
	assert.Equal(t, "text_chunks", KVStoreTextChunks)
	assert.Equal(t, "llm_response_cache", KVStoreLLMResponseCache)
	assert.Equal(t, "full_entities", KVStoreFullEntities)
	assert.Equal(t, "full_relations", KVStoreFullRelations)
	assert.Equal(t, "entity_chunks", KVStoreEntityChunks)
	assert.Equal(t, "relation_chunks", KVStoreRelationChunks)

	// Test vector store namespaces
	assert.Equal(t, "entities", VectorStoreEntities)
	assert.Equal(t, "relationships", VectorStoreRelationships)
	assert.Equal(t, "chunks", VectorStoreChunks)

	// Test graph store namespace
	assert.Equal(t, "chunk_entity_relation", GraphStoreChunkEntityRelation)

	// Test doc status namespace
	assert.Equal(t, "doc_status", DocStatus)
}

func TestAllKVNamespaces(t *testing.T) {
	namespaces := AllKVNamespaces()
	assert.Len(t, namespaces, 7)
	assert.Contains(t, namespaces, KVStoreFullDocs)
	assert.Contains(t, namespaces, KVStoreTextChunks)
	assert.Contains(t, namespaces, KVStoreLLMResponseCache)
}

func TestAllVectorNamespaces(t *testing.T) {
	namespaces := AllVectorNamespaces()
	assert.Len(t, namespaces, 3)
	assert.Contains(t, namespaces, VectorStoreEntities)
	assert.Contains(t, namespaces, VectorStoreRelationships)
	assert.Contains(t, namespaces, VectorStoreChunks)
}

func TestIsNamespace(t *testing.T) {
	tests := []struct {
		namespace      string
		baseNamespaces []string
		expected       bool
	}{
		{"workspace_full_docs", []string{"full_docs"}, true},
		{"full_docs", []string{"full_docs"}, true},
		{"workspace_text_chunks", []string{"text_chunks", "full_docs"}, true},
		{"workspace_entities", []string{"entities", "chunks"}, true},
		{"workspace_other", []string{"full_docs", "text_chunks"}, false},
		{"", []string{"full_docs"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			result := IsNamespace(tt.namespace, tt.baseNamespaces...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsKVNamespace(t *testing.T) {
	assert.True(t, IsKVNamespace("workspace_full_docs"))
	assert.True(t, IsKVNamespace("full_docs"))
	assert.True(t, IsKVNamespace("test_text_chunks"))
	assert.False(t, IsKVNamespace("entities"))
	assert.False(t, IsKVNamespace("random_namespace"))
}

func TestIsVectorNamespace(t *testing.T) {
	assert.True(t, IsVectorNamespace("workspace_entities"))
	assert.True(t, IsVectorNamespace("entities"))
	assert.True(t, IsVectorNamespace("workspace_relationships"))
	assert.True(t, IsVectorNamespace("chunks"))
	assert.False(t, IsVectorNamespace("full_docs"))
}

func TestIsGraphNamespace(t *testing.T) {
	assert.True(t, IsGraphNamespace("chunk_entity_relation"))
	assert.True(t, IsGraphNamespace("workspace_chunk_entity_relation"))
	assert.False(t, IsGraphNamespace("entities"))
	assert.False(t, IsGraphNamespace("full_docs"))
}

func TestIsDocStatusNamespace(t *testing.T) {
	assert.True(t, IsDocStatusNamespace("doc_status"))
	assert.True(t, IsDocStatusNamespace("workspace_doc_status"))
	assert.False(t, IsDocStatusNamespace("full_docs"))
}
