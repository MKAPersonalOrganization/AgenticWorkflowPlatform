// Package main demonstrates LightRAG with OpenAI API.
//
// This example shows how to:
// 1. Configure LightRAG to use OpenAI for LLM and embeddings
// 2. Insert documents into the knowledge graph
// 3. Query using different modes
//
// Requirements:
// - OpenAI API key (set OPENAI_API_KEY environment variable)
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lightrag/lightrag-go/pkg/core/base"
	"github.com/lightrag/lightrag-go/pkg/core/lightrag"
	"github.com/lightrag/lightrag-go/pkg/core/operate"
	"github.com/lightrag/lightrag-go/pkg/llm"
	"github.com/lightrag/lightrag-go/pkg/storage"
	"github.com/sirupsen/logrus"
)

// getEnvOrDefault returns the environment variable value or a default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	// Configure logging
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Get OpenAI API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		logger.Fatal("Error: OPENAI_API_KEY environment variable is not set")
	}

	// Configuration from environment
	llmModel := getEnvOrDefault("OPENAI_MODEL", "gpt-4o-mini")
	embeddingModel := getEnvOrDefault("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
	workingDir := getEnvOrDefault("WORKING_DIR", "./openai_demo_data")

	logger.Info("Starting LightRAG with OpenAI Demo")
	logger.Infof("LLM Model: %s", llmModel)
	logger.Infof("Embedding Model: %s", embeddingModel)
	logger.Infof("Working Directory: %s", workingDir)

	// Create working directory
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		logger.Fatalf("Failed to create working directory: %v", err)
	}

	// Initialize OpenAI client
	openaiClient := llm.NewOpenAIClient(&llm.LLMConfig{
		APIKey:         apiKey,
		Model:          llmModel,
		EmbeddingModel: embeddingModel,
		Timeout:        120,
	}, logger)

	// Create LLM function wrapper
	llmFunc := func(ctx context.Context, prompt string, opts ...operate.LLMOption) (string, error) {
		messages := []llm.Message{
			{Role: "user", Content: prompt},
		}
		resp, err := openaiClient.Complete(ctx, &llm.CompletionRequest{Messages: messages})
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	}

	// Create embedding function
	embeddingFunc := func(ctx context.Context, texts []string) ([][]float64, error) {
		resp, err := openaiClient.Embed(ctx, &llm.EmbeddingRequest{Input: texts})
		if err != nil {
			return nil, err
		}
		return resp.Embeddings, nil
	}

	// Test embedding to get dimension
	logger.Info("Testing embedding function...")
	testEmbedding, err := embeddingFunc(context.Background(), []string{"test"})
	if err != nil {
		logger.Fatalf("Failed to test embedding: %v", err)
	}
	embeddingDim := len(testEmbedding[0])
	logger.Infof("Detected embedding dimension: %d", embeddingDim)

	// Create LightRAG configuration
	config := lightrag.DefaultConfig()
	config.WorkingDir = workingDir
	config.LLMModelFunc = llmFunc
	config.LLMModelName = llmModel
	config.EmbeddingFunc = &base.EmbeddingFunc{
		Func:         embeddingFunc,
		EmbeddingDim: embeddingDim,
		MaxTokenSize: 8192,
		ModelName:    embeddingModel,
	}

	// Create LightRAG instance
	rag, err := lightrag.New(config)
	if err != nil {
		logger.Fatalf("Failed to create LightRAG instance: %v", err)
	}

	// Create storage instances
	logger.Info("Initializing storage backends...")

	// Storage configuration helper functions
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

	// Sample document - A Tale of Two Cities opening
	sampleDoc := `
A Tale of Two Cities by Charles Dickens begins with one of the most famous opening lines in literature:
"It was the best of times, it was the worst of times, it was the age of wisdom, it was the age of foolishness."

The novel is set in London and Paris before and during the French Revolution. 
The main characters include Charles Darnay, a French aristocrat who renounces his heritage,
and Sydney Carton, a dissipated English lawyer who sacrifices himself for love.

Dr. Alexandre Manette was imprisoned in the Bastille for 18 years before being released.
His daughter Lucie Manette becomes the emotional center of the novel.
Madame Defarge knits the names of those condemned to die during the Revolution.

The novel explores themes of resurrection, sacrifice, and the duality of human nature.
It contrasts the chaos of revolutionary France with the stability of England.
`

	// Insert the document
	logger.Info("Inserting sample document...")
	if err := rag.Insert(ctx, sampleDoc); err != nil {
		logger.Fatalf("Failed to insert document: %v", err)
	}
	logger.Info("Document inserted successfully!")

	// Test queries
	queries := []string{
		"Who is Sydney Carton?",
		"What happened to Dr. Manette?",
		"What are the main themes of the novel?",
	}

	modes := []string{"naive", "local", "global", "hybrid"}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("QUERY RESULTS - A Tale of Two Cities")
	fmt.Println(strings.Repeat("=", 70))

	for _, query := range queries {
		fmt.Printf("\n📝 Query: %s\n", query)
		fmt.Println(strings.Repeat("-", 70))

		for _, mode := range modes {
			fmt.Printf("\n🔍 Mode: %s\n", mode)

			param := base.NewQueryParam()
			param.Mode = mode

			result, err := rag.Query(ctx, query, param)
			if err != nil {
				logger.Errorf("Query failed with mode %s: %v", mode, err)
				continue
			}

			// Get response text
			response := result.Answer
			if response == "" {
				response = result.Content
			}

			// Truncate long responses
			if len(response) > 500 {
				response = response[:500] + "..."
			}
			fmt.Printf("Response: %s\n", response)
		}
		fmt.Println()
	}

	logger.Info("Demo completed successfully!")
}
