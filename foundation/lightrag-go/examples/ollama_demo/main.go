// Package main demonstrates LightRAG with Ollama for local LLM inference.
//
// This example shows how to:
// 1. Configure LightRAG to use Ollama for both LLM and embeddings
// 2. Insert documents into the knowledge graph
// 3. Query using different modes (naive, local, global, hybrid)
//
// Requirements:
// - Ollama running locally (or specify OLLAMA_HOST)
// - Models: qwen2.5-coder:7b (or change OLLAMA_MODEL)
// - Embedding model: bge-m3:latest (or change OLLAMA_EMBEDDING_MODEL)
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

	// Configuration from environment
	ollamaHost := getEnvOrDefault("OLLAMA_HOST", "http://localhost:11434")
	llmModel := getEnvOrDefault("OLLAMA_MODEL", "qwen2.5-coder:7b")
	embeddingModel := getEnvOrDefault("OLLAMA_EMBEDDING_MODEL", "bge-m3:latest")
	workingDir := getEnvOrDefault("WORKING_DIR", "./ollama_demo_data")

	logger.Info("Starting LightRAG with Ollama Demo")
	logger.Infof("Ollama Host: %s", ollamaHost)
	logger.Infof("LLM Model: %s", llmModel)
	logger.Infof("Embedding Model: %s", embeddingModel)
	logger.Infof("Working Directory: %s", workingDir)

	// Create working directory
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		logger.Fatalf("Failed to create working directory: %v", err)
	}

	// Initialize Ollama client
	ollamaClient := llm.NewOllamaClient(&llm.LLMConfig{
		BaseURL:        ollamaHost,
		Model:          llmModel,
		EmbeddingModel: embeddingModel,
		Timeout:        300,
	}, logger)

	// Create LLM function wrapper
	llmFunc := func(ctx context.Context, prompt string, opts ...operate.LLMOption) (string, error) {
		messages := []llm.Message{
			{Role: "user", Content: prompt},
		}
		resp, err := ollamaClient.Complete(ctx, &llm.CompletionRequest{Messages: messages})
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	}

	// Create embedding function
	embeddingFunc := func(ctx context.Context, texts []string) ([][]float64, error) {
		resp, err := ollamaClient.Embed(ctx, &llm.EmbeddingRequest{Input: texts})
		if err != nil {
			return nil, err
		}
		return resp.Embeddings, nil
	}

	// Test embedding to get dimension
	logger.Info("Testing embedding function...")
	testEmbedding, err := embeddingFunc(context.Background(), []string{"This is a test string for embedding."})
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

	// Storage configuration for all backends
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

	// Sample document for insertion
	sampleDoc := `
Artificial Intelligence (AI) is a branch of computer science that aims to create intelligent machines.
Machine learning is a subset of AI that enables systems to learn from data and improve from experience.
Deep learning is a subset of machine learning using neural networks with many layers.
Natural Language Processing (NLP) allows machines to understand, interpret, and generate human language.
Computer Vision enables machines to interpret and make decisions based on visual data.
Reinforcement Learning teaches agents to make decisions by rewarding desired behaviors.
Large Language Models (LLMs) like GPT and Claude are trained on vast amounts of text data.
RAG (Retrieval-Augmented Generation) combines retrieval systems with language models for better accuracy.
Knowledge graphs store information as entities and relationships, enabling structured knowledge representation.
Vector databases enable efficient similarity search for AI applications.
`

	// Insert the document
	logger.Info("Inserting sample document...")
	if err := rag.Insert(ctx, sampleDoc); err != nil {
		logger.Fatalf("Failed to insert document: %v", err)
	}
	logger.Info("Document inserted successfully!")

	// Test queries with different modes
	queries := []string{
		"What is machine learning?",
		"How does RAG work?",
		"What are the relationships between AI, ML, and DL?",
	}

	modes := []string{"naive", "local", "global", "hybrid"}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("QUERY RESULTS")
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

			// Truncate long responses for display
			if len(response) > 500 {
				response = response[:500] + "..."
			}
			fmt.Printf("Response: %s\n", response)
		}
		fmt.Println()
	}

	logger.Info("Demo completed successfully!")
}
