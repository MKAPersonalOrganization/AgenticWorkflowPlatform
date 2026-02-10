// Package namespace provides namespace constants for LightRAG storage.
package namespace

import "strings"

// NameSpace contains all namespace constants for storage.
// All namespace values should not be changed after data is inserted.
type NameSpace struct{}

// KV Store namespaces
const (
	KVStoreFullDocs         = "full_docs"
	KVStoreTextChunks       = "text_chunks"
	KVStoreLLMResponseCache = "llm_response_cache"
	KVStoreFullEntities     = "full_entities"
	KVStoreFullRelations    = "full_relations"
	KVStoreEntityChunks     = "entity_chunks"
	KVStoreRelationChunks   = "relation_chunks"
)

// Vector Store namespaces
const (
	VectorStoreEntities      = "entities"
	VectorStoreRelationships = "relationships"
	VectorStoreChunks        = "chunks"
)

// Graph Store namespaces
const (
	GraphStoreChunkEntityRelation = "chunk_entity_relation"
)

// Document Status namespace
const (
	DocStatus = "doc_status"
)

// AllKVNamespaces returns all KV store namespaces.
func AllKVNamespaces() []string {
	return []string{
		KVStoreFullDocs,
		KVStoreTextChunks,
		KVStoreLLMResponseCache,
		KVStoreFullEntities,
		KVStoreFullRelations,
		KVStoreEntityChunks,
		KVStoreRelationChunks,
	}
}

// AllVectorNamespaces returns all vector store namespaces.
func AllVectorNamespaces() []string {
	return []string{
		VectorStoreEntities,
		VectorStoreRelationships,
		VectorStoreChunks,
	}
}

// IsNamespace checks if a namespace ends with any of the base namespaces.
func IsNamespace(namespace string, baseNamespaces ...string) bool {
	for _, base := range baseNamespaces {
		if strings.HasSuffix(namespace, base) {
			return true
		}
	}
	return false
}

// IsKVNamespace checks if a namespace is a KV store namespace.
func IsKVNamespace(namespace string) bool {
	return IsNamespace(namespace, AllKVNamespaces()...)
}

// IsVectorNamespace checks if a namespace is a vector store namespace.
func IsVectorNamespace(namespace string) bool {
	return IsNamespace(namespace, AllVectorNamespaces()...)
}

// IsGraphNamespace checks if a namespace is a graph store namespace.
func IsGraphNamespace(namespace string) bool {
	return IsNamespace(namespace, GraphStoreChunkEntityRelation)
}

// IsDocStatusNamespace checks if a namespace is a doc status namespace.
func IsDocStatusNamespace(namespace string) bool {
	return IsNamespace(namespace, DocStatus)
}
