// Package main demonstrates inserting custom knowledge graph data into LightRAG.
//
// This example shows how to:
// 1. Create custom entities, relationships, and source chunks
// 2. Insert them directly into the knowledge graph
// 3. Query the custom knowledge
//
// This is useful when you have pre-existing structured knowledge that you
// want to import into LightRAG without going through document extraction.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lightrag/lightrag-go/pkg/core/base"
	"github.com/lightrag/lightrag-go/pkg/core/lightrag"
	"github.com/lightrag/lightrag-go/pkg/core/operate"
	"github.com/lightrag/lightrag-go/pkg/storage"
	"github.com/sirupsen/logrus"
)

// Entity represents a knowledge graph entity.
type Entity struct {
	Name        string
	Type        string
	Description string
	SourceID    string
}

// Relationship represents a relationship between entities.
type Relationship struct {
	Source      string
	Target      string
	Description string
	Keywords    string
	Weight      float64
	SourceID    string
}

// Chunk represents a source text chunk.
type Chunk struct {
	ID      string
	Content string
	Tokens  int
}

func main() {
	// Configure logging
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	workingDir := "./custom_kg_data"
	embeddingDim := 128 // Mock dimension for demo

	logger.Info("Starting Custom Knowledge Graph Demo")
	logger.Infof("Working Directory: %s", workingDir)

	// Create working directory
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		logger.Fatalf("Failed to create working directory: %v", err)
	}

	// Create mock LLM function (for demo purposes)
	llmFunc := func(ctx context.Context, prompt string, opts ...operate.LLMOption) (string, error) {
		// In a real scenario, this would call an actual LLM
		return "This is a mock response demonstrating custom KG insertion.", nil
	}

	// Create mock embedding function (for demo purposes)
	embeddingFunc := func(ctx context.Context, texts []string) ([][]float64, error) {
		// Generate deterministic mock embeddings based on text content
		results := make([][]float64, len(texts))
		for i, text := range texts {
			embedding := make([]float64, embeddingDim)
			for j := 0; j < embeddingDim; j++ {
				// Simple hash-based embedding for reproducibility
				embedding[j] = float64((len(text)+j)%100) / 100.0
			}
			results[i] = embedding
		}
		return results, nil
	}

	// Create LightRAG configuration
	config := lightrag.DefaultConfig()
	config.WorkingDir = workingDir
	config.LLMModelFunc = llmFunc
	config.LLMModelName = "mock-llm"
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func:         embeddingFunc,
		EmbeddingDim: embeddingDim,
		MaxTokenSize: 8192,
		ModelName:    "mock-embedding",
	}

	// Create LightRAG instance
	rag, err := lightrag.New(config)
	if err != nil {
		logger.Fatalf("Failed to create LightRAG instance: %v", err)
	}

	// Create storage instances
	logger.Info("Initializing storage backends...")

	kvConfig := func(namespace string) *storage.StorageConfig {
		return storage.NewStorageConfig(namespace, workingDir)
	}

	vdbConfig := func(namespace string) *storage.StorageConfig {
		cfg := storage.NewStorageConfig(namespace, workingDir)
		cfg.EmbeddingDim = embeddingDim
		cfg.CosineThreshold = 0.2
		cfg.EmbeddingFunc = embeddingFunc
		return cfg
	}

	graphConfig := func(namespace string) *storage.StorageConfig {
		return storage.NewStorageConfig(namespace, workingDir)
	}

	// Create all storage backends
	fullDocs := storage.NewJsonKVStorage(kvConfig("full_docs"), logger)
	textChunks := storage.NewJsonKVStorage(kvConfig("text_chunks"), logger)
	fullEntities := storage.NewJsonKVStorage(kvConfig("full_entities"), logger)
	fullRelations := storage.NewJsonKVStorage(kvConfig("full_relations"), logger)
	entityChunks := storage.NewJsonKVStorage(kvConfig("entity_chunks"), logger)
	relationChunks := storage.NewJsonKVStorage(kvConfig("relation_chunks"), logger)
	entitiesVDB := storage.NewNanoVectorDBStorage(vdbConfig("entities"), logger)
	relationshipsVDB := storage.NewNanoVectorDBStorage(vdbConfig("relationships"), logger)
	chunksVDB := storage.NewNanoVectorDBStorage(vdbConfig("chunks"), logger)
	graphStorage := storage.NewNetworkXStorage(graphConfig("graph"), logger)
	llmCache := storage.NewJsonKVStorage(kvConfig("llm_cache"), logger)
	docStatus := storage.NewJsonKVStorage(kvConfig("doc_status"), logger)

	// Set all storages
	rag.SetStorages(
		fullDocs, textChunks, fullEntities, fullRelations,
		entityChunks, relationChunks,
		entitiesVDB, relationshipsVDB, chunksVDB,
		graphStorage, llmCache, docStatus,
	)

	// Initialize storages
	ctx := context.Background()
	if err := rag.InitializeStorages(ctx); err != nil {
		logger.Fatalf("Failed to initialize storages: %v", err)
	}
	defer rag.FinalizeStorages(ctx)

	// Define custom entities
	entities := []Entity{
		{
			Name:        "Albert Einstein",
			Type:        "PERSON",
			Description: "Theoretical physicist who developed the theory of relativity",
			SourceID:    "chunk_physics_1",
		},
		{
			Name:        "Theory of Relativity",
			Type:        "CONCEPT",
			Description: "A scientific theory describing space, time, and gravitation",
			SourceID:    "chunk_physics_1",
		},
		{
			Name:        "Marie Curie",
			Type:        "PERSON",
			Description: "Physicist and chemist who conducted pioneering research on radioactivity",
			SourceID:    "chunk_physics_2",
		},
		{
			Name:        "Radioactivity",
			Type:        "CONCEPT",
			Description: "The emission of ionizing radiation by atomic nuclei",
			SourceID:    "chunk_physics_2",
		},
		{
			Name:        "Nobel Prize",
			Type:        "AWARD",
			Description: "International awards given annually for achievements in physics, chemistry, medicine, literature, and peace",
			SourceID:    "chunk_awards_1",
		},
	}

	// Define custom relationships
	relationships := []Relationship{
		{
			Source:      "Albert Einstein",
			Target:      "Theory of Relativity",
			Description: "developed and published",
			Keywords:    "discovery, physics, publication",
			Weight:      1.0,
			SourceID:    "chunk_physics_1",
		},
		{
			Source:      "Marie Curie",
			Target:      "Radioactivity",
			Description: "discovered and researched",
			Keywords:    "discovery, chemistry, research",
			Weight:      1.0,
			SourceID:    "chunk_physics_2",
		},
		{
			Source:      "Albert Einstein",
			Target:      "Nobel Prize",
			Description: "received in 1921 for physics",
			Keywords:    "award, recognition, physics",
			Weight:      0.8,
			SourceID:    "chunk_awards_1",
		},
		{
			Source:      "Marie Curie",
			Target:      "Nobel Prize",
			Description: "received twice in physics and chemistry",
			Keywords:    "award, recognition, historic",
			Weight:      1.0,
			SourceID:    "chunk_awards_1",
		},
	}

	// Define source chunks
	chunks := []Chunk{
		{
			ID: "chunk_physics_1",
			Content: `Albert Einstein (1879-1955) was a theoretical physicist who developed the theory of relativity,
one of the two pillars of modern physics. His work is known for its influence on the philosophy of science.
Einstein's mass-energy equivalence formula E=mc² has been dubbed "the world's most famous equation."`,
			Tokens: 50,
		},
		{
			ID: "chunk_physics_2",
			Content: `Marie Curie (1867-1934) was a Polish and naturalized-French physicist and chemist who conducted
pioneering research on radioactivity. She was the first woman to win a Nobel Prize and the only person
to win the Nobel Prize in two different sciences.`,
			Tokens: 45,
		},
		{
			ID: "chunk_awards_1",
			Content: `The Nobel Prize is a set of international awards given annually for achievements in physics,
chemistry, medicine, literature, and peace. Albert Einstein received the Nobel Prize in Physics in 1921.
Marie Curie received the Nobel Prize in Physics in 1903 and the Nobel Prize in Chemistry in 1911.`,
			Tokens: 55,
		},
	}

	// Insert custom knowledge graph data
	logger.Info("Inserting custom entities...")
	for _, entity := range entities {
		entityData := map[string]map[string]interface{}{
			entity.Name: {
				"entity_name": entity.Name,
				"entity_type": entity.Type,
				"description": entity.Description,
				"source_id":   entity.SourceID,
			},
		}
		if err := fullEntities.Upsert(ctx, entityData); err != nil {
			logger.Errorf("Failed to insert entity %s: %v", entity.Name, err)
		}

		// Also add to graph storage
		if err := graphStorage.UpsertNode(ctx, entity.Name, map[string]string{
			"entity_type": entity.Type,
			"description": entity.Description,
			"source_id":   entity.SourceID,
		}); err != nil {
			logger.Errorf("Failed to add node %s to graph: %v", entity.Name, err)
		}
	}

	logger.Info("Inserting custom relationships...")
	for _, rel := range relationships {
		relData := map[string]map[string]interface{}{
			rel.Source + "||" + rel.Target: {
				"src_id":      rel.Source,
				"tgt_id":      rel.Target,
				"description": rel.Description,
				"keywords":    rel.Keywords,
				"weight":      rel.Weight,
				"source_id":   rel.SourceID,
			},
		}
		if err := fullRelations.Upsert(ctx, relData); err != nil {
			logger.Errorf("Failed to insert relationship %s -> %s: %v", rel.Source, rel.Target, err)
		}

		// Also add to graph storage
		if err := graphStorage.UpsertEdge(ctx, rel.Source, rel.Target, map[string]string{
			"description": rel.Description,
			"keywords":    rel.Keywords,
			"weight":      fmt.Sprintf("%f", rel.Weight),
			"source_id":   rel.SourceID,
		}); err != nil {
			logger.Errorf("Failed to add edge %s -> %s to graph: %v", rel.Source, rel.Target, err)
		}
	}

	logger.Info("Inserting source chunks...")
	for _, chunk := range chunks {
		chunkData := map[string]map[string]interface{}{
			chunk.ID: {
				"content":     chunk.Content,
				"tokens":      chunk.Tokens,
				"chunk_order": 0,
			},
		}
		if err := textChunks.Upsert(ctx, chunkData); err != nil {
			logger.Errorf("Failed to insert chunk %s: %v", chunk.ID, err)
		}
	}

	logger.Info("Custom knowledge graph data inserted successfully!")

	// Display the inserted data
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("CUSTOM KNOWLEDGE GRAPH SUMMARY")
	fmt.Println(strings.Repeat("=", 70))

	fmt.Println("\n📊 Entities:")
	for _, e := range entities {
		fmt.Printf("  • %s (%s): %s\n", e.Name, e.Type, e.Description)
	}

	fmt.Println("\n🔗 Relationships:")
	for _, r := range relationships {
		fmt.Printf("  • %s -[%s]-> %s\n", r.Source, r.Description, r.Target)
	}

	fmt.Println("\n📄 Source Chunks:")
	for _, c := range chunks {
		preview := c.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		fmt.Printf("  • %s: %s\n", c.ID, preview)
	}

	// Test querying the custom knowledge
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("QUERY RESULTS")
	fmt.Println(strings.Repeat("=", 70))

	queries := []string{
		"Who developed the theory of relativity?",
		"Who won the Nobel Prize?",
		"What did Marie Curie discover?",
	}

	for _, query := range queries {
		fmt.Printf("\n📝 Query: %s\n", query)

		param := base.NewQueryParam()
		param.Mode = "local"

		result, err := rag.Query(ctx, query, param)
		if err != nil {
			logger.Errorf("Query failed: %v", err)
			continue
		}

		response := result.Answer
		if response == "" {
			response = result.Content
		}
		fmt.Printf("Response: %s\n", response)
	}

	logger.Info("Custom KG demo completed successfully!")
}
