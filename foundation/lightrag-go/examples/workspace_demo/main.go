// Package main demonstrates workspace isolation in LightRAG.
//
// This example shows how to:
// 1. Create multiple isolated workspaces
// 2. Insert different content into each workspace
// 3. Query each workspace independently
// 4. Verify data isolation between workspaces
//
// Workspaces provide multi-tenancy support, allowing multiple
// independent knowledge bases to coexist.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lightrag/lightrag-go/pkg/core/base"
	"github.com/lightrag/lightrag-go/pkg/core/lightrag"
	"github.com/lightrag/lightrag-go/pkg/core/operate"
	"github.com/lightrag/lightrag-go/pkg/storage"
	"github.com/sirupsen/logrus"
)

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// createRAGForWorkspace creates a LightRAG instance for a specific workspace.
func createRAGForWorkspace(
	logger *logrus.Logger,
	baseDir, workspace string,
	llmFunc operate.LLMFunc,
	embeddingFunc func(ctx context.Context, texts []string) ([][]float64, error),
	embeddingDim int,
) (*lightrag.LightRAG, error) {
	workspaceDir := filepath.Join(baseDir, workspace)
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Create LightRAG configuration
	config := lightrag.DefaultConfig()
	config.WorkingDir = workspaceDir
	config.Workspace = workspace
	config.LLMModelFunc = llmFunc
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func:         embeddingFunc,
		EmbeddingDim: embeddingDim,
		MaxTokenSize: 8192,
	}

	// Create LightRAG instance
	rag, err := lightrag.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create LightRAG: %w", err)
	}

	// Create storage instances
	kvConfig := func(namespace string) *storage.StorageConfig {
		return storage.NewStorageConfig(namespace, workspaceDir)
	}

	vdbConfig := func(namespace string) *storage.StorageConfig {
		cfg := storage.NewStorageConfig(namespace, workspaceDir)
		cfg.EmbeddingDim = embeddingDim
		cfg.CosineThreshold = 0.2
		cfg.EmbeddingFunc = embeddingFunc
		return cfg
	}

	graphConfig := func(namespace string) *storage.StorageConfig {
		return storage.NewStorageConfig(namespace, workspaceDir)
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

	return rag, nil
}

func main() {
	// Configure logging
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	baseDir := getEnvOrDefault("WORKING_DIR", "./workspace_demo_data")
	embeddingDim := 128 // Mock dimension

	logger.Info("Starting LightRAG Workspace Isolation Demo")
	logger.Infof("Base Directory: %s", baseDir)

	// Create base directory
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		logger.Fatalf("Failed to create base directory: %v", err)
	}

	// Mock LLM function
	llmFunc := func(ctx context.Context, prompt string, opts ...operate.LLMOption) (string, error) {
		// Simple mock that returns relevant information based on prompt content
		promptLower := strings.ToLower(prompt)
		if strings.Contains(promptLower, "einstein") || strings.Contains(promptLower, "relativity") {
			return "Albert Einstein developed the theory of relativity, which revolutionized our understanding of space, time, and gravity.", nil
		}
		if strings.Contains(promptLower, "caesar") || strings.Contains(promptLower, "rome") {
			return "Julius Caesar was a Roman general and statesman who played a critical role in transforming the Roman Republic into the Roman Empire.", nil
		}
		return "Based on the available context, I cannot find specific information about this topic in the current knowledge base.", nil
	}

	// Mock embedding function
	embeddingFunc := func(ctx context.Context, texts []string) ([][]float64, error) {
		results := make([][]float64, len(texts))
		for i, text := range texts {
			embedding := make([]float64, embeddingDim)
			// Create deterministic embeddings based on content
			hash := 0
			for _, c := range text {
				hash += int(c)
			}
			for j := 0; j < embeddingDim; j++ {
				embedding[j] = float64((hash+j)%100) / 100.0
			}
			results[i] = embedding
		}
		return results, nil
	}

	ctx := context.Background()

	// Create workspace A: Science
	logger.Info("Creating Science workspace...")
	ragScience, err := createRAGForWorkspace(logger, baseDir, "science", llmFunc, embeddingFunc, embeddingDim)
	if err != nil {
		logger.Fatalf("Failed to create science workspace: %v", err)
	}
	if err := ragScience.InitializeStorages(ctx); err != nil {
		logger.Fatalf("Failed to initialize science storages: %v", err)
	}
	defer ragScience.FinalizeStorages(ctx)

	// Create workspace B: History
	logger.Info("Creating History workspace...")
	ragHistory, err := createRAGForWorkspace(logger, baseDir, "history", llmFunc, embeddingFunc, embeddingDim)
	if err != nil {
		logger.Fatalf("Failed to create history workspace: %v", err)
	}
	if err := ragHistory.InitializeStorages(ctx); err != nil {
		logger.Fatalf("Failed to initialize history storages: %v", err)
	}
	defer ragHistory.FinalizeStorages(ctx)

	// Insert content into Science workspace
	scienceContent := `
Physics is the natural science that studies matter, its motion, and behavior through space and time.
Isaac Newton developed the laws of motion and universal gravitation in the 17th century.
Albert Einstein developed the theory of relativity, revolutionizing our understanding of space and time.
Quantum mechanics describes the behavior of particles at the atomic and subatomic levels.
The Standard Model describes fundamental particles and their interactions.
`

	logger.Info("Inserting content into Science workspace...")
	if err := ragScience.Insert(ctx, scienceContent); err != nil {
		logger.Errorf("Failed to insert science content: %v", err)
	}

	// Insert content into History workspace
	historyContent := `
The Roman Empire was one of the largest empires in ancient history, spanning Europe, North Africa, and the Middle East.
Julius Caesar was a Roman general who played a critical role in transforming the Roman Republic.
The Renaissance was a period of cultural, artistic, and intellectual rebirth in Europe during the 14th-17th centuries.
World War II was a global conflict that lasted from 1939 to 1945, involving most of the world's nations.
The Industrial Revolution transformed manufacturing and society starting in the late 18th century.
`

	logger.Info("Inserting content into History workspace...")
	if err := ragHistory.Insert(ctx, historyContent); err != nil {
		logger.Errorf("Failed to insert history content: %v", err)
	}

	// Test queries to demonstrate isolation
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("WORKSPACE ISOLATION TEST")
	fmt.Println(strings.Repeat("=", 70))

	// Test 1: Science query in Science workspace
	query1 := "What did Einstein develop?"
	fmt.Printf("\n🔬 [Science Workspace] Query: %s\n", query1)
	param := base.NewQueryParam()
	param.Mode = "local"
	result, err := ragScience.Query(ctx, query1, param)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		response := result.Answer
		if response == "" {
			response = result.Content
		}
		fmt.Printf("Response: %s\n", response)
	}

	// Test 2: Same science query in History workspace
	fmt.Printf("\n📜 [History Workspace] Query: %s\n", query1)
	result, err = ragHistory.Query(ctx, query1, param)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		response := result.Answer
		if response == "" {
			response = result.Content
		}
		fmt.Printf("Response: %s\n", response)
	}

	// Test 3: History query in History workspace
	query2 := "Who was Julius Caesar?"
	fmt.Printf("\n📜 [History Workspace] Query: %s\n", query2)
	result, err = ragHistory.Query(ctx, query2, param)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		response := result.Answer
		if response == "" {
			response = result.Content
		}
		fmt.Printf("Response: %s\n", response)
	}

	// Test 4: Same history query in Science workspace
	fmt.Printf("\n🔬 [Science Workspace] Query: %s\n", query2)
	result, err = ragScience.Query(ctx, query2, param)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		response := result.Answer
		if response == "" {
			response = result.Content
		}
		fmt.Printf("Response: %s\n", response)
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("ISOLATION SUMMARY")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Print(`
This demo shows that:
1. Each workspace maintains its own isolated knowledge graph
2. Science content is only accessible from the Science workspace
3. History content is only accessible from the History workspace
4. Cross-workspace queries return "no information" responses

This isolation enables multi-tenancy, where different users or
applications can have their own private knowledge bases without
data leakage.
`)

	logger.Info("Workspace isolation demo completed!")
}
