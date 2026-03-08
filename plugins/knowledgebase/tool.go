package knowledgebase

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"school-with-ai/foundation/agent"
)

// DefaultKBSystemPrompt is the system prompt template for the knowledge base sub-agent.
// Mirrors the proven prompt from the RAG CLI (rag_to_openai.go).
const DefaultKBSystemPrompt = `---Role---

You are an expert AI assistant specializing in synthesizing information from a provided knowledge base. Your primary function is to answer user queries accurately by ONLY using the information within the provided **Context**.

---Goal---

Generate a comprehensive, well-structured answer to the user query.
The answer must integrate relevant facts from the Knowledge Graph and Document Chunks found in the **Context**.

---Instructions---

1. Step-by-Step Reasoning:
  - Carefully determine the user's query intent to fully understand the information need.
  - Scrutinize both Knowledge Graph Data (Entities and Relationships) and Document Chunks in the **Context**. Identify and extract all pieces of information that are directly relevant to answering the user query.
  - Weave the extracted facts into a coherent and logical response. Your own knowledge must ONLY be used to formulate fluent sentences and connect ideas, NOT to introduce any external information.

2. Content & Grounding:
  - Strictly adhere to the provided context; DO NOT invent, assume, or infer any information not explicitly stated.
  - If the answer cannot be fully determined from the **Context**, state what information IS available and note what is missing. A partial answer with specific data is better than a generic "insufficient information" response.

3. Formatting & Language:
  - The response MUST be in the same language as the user query.
  - Use Markdown formatting for clarity (headings, bold text, bullet points).

---Context---

{{.ContextText}}`

// --- Edgequake Retrieval API types (mirrors rag_to_openai.go) ---

type sourceReference struct {
	SourceType       string   `json:"source_type"` // chunk, entity, relationship
	ID               string   `json:"id"`
	Score            float32  `json:"score"`
	RerankScore      *float32 `json:"rerank_score,omitempty"`
	Snippet          *string  `json:"snippet,omitempty"`
	ReferenceID      *int     `json:"reference_id,omitempty"`
	DocumentID       *string  `json:"document_id,omitempty"`
	FilePath         *string  `json:"file_path,omitempty"`
	SourceDocumentID *string  `json:"source_document_id,omitempty"`
	SourceFilePath   *string  `json:"source_file_path,omitempty"`
	StartLine        *int     `json:"start_line,omitempty"`
	EndLine          *int     `json:"end_line,omitempty"`
	ChunkIndex       *int     `json:"chunk_index,omitempty"`
}

type queryRequest struct {
	Query             string `json:"query"`
	Mode              string `json:"mode,omitempty"`
	ContextOnly       bool   `json:"context_only"`
	IncludeReferences bool   `json:"include_references"`
	EnableRerank      bool   `json:"enable_rerank,omitempty"`
}

type queryResponse struct {
	Answer  string            `json:"answer"`
	Mode    string            `json:"mode"`
	Sources []sourceReference `json:"sources"`
}

// --- Structured KB Result (returned to frontend as JSON) ---

// KBSourceChunk represents a document chunk source for the UI.
type KBSourceChunk struct {
	DocumentID string  `json:"document_id"`
	FilePath   string  `json:"file_path,omitempty"`
	Content    string  `json:"content"`
	Score      float32 `json:"score"`
	StartLine  *int    `json:"start_line,omitempty"`
	EndLine    *int    `json:"end_line,omitempty"`
	ChunkIndex *int    `json:"chunk_index,omitempty"`
}

// KBSourceEntity represents a knowledge graph entity for the UI.
type KBSourceEntity struct {
	ID               string  `json:"id"`
	Label            string  `json:"label"`
	Relevance        float32 `json:"relevance"`
	SourceDocumentID string  `json:"source_document_id,omitempty"`
	SourceFilePath   string  `json:"source_file_path,omitempty"`
}

// KBSourceRelationship represents a knowledge graph relationship for the UI.
type KBSourceRelationship struct {
	Source           string  `json:"source"`
	Target           string  `json:"target"`
	Type             string  `json:"type"`
	Relevance        float32 `json:"relevance"`
	SourceDocumentID string  `json:"source_document_id,omitempty"`
	SourceFilePath   string  `json:"source_file_path,omitempty"`
}

// KBSources groups all source types for the UI.
type KBSources struct {
	Chunks        []KBSourceChunk        `json:"chunks"`
	Entities      []KBSourceEntity       `json:"entities"`
	Relationships []KBSourceRelationship `json:"relationships"`
}

// KBGraphNode represents a node in the knowledge graph visualization.
type KBGraphNode struct {
	ID          string          `json:"id"`
	Label       string          `json:"label"`
	NodeType    string          `json:"node_type"`
	Description string          `json:"description,omitempty"`
	Degree      int             `json:"degree"`
	Properties  json.RawMessage `json:"properties,omitempty"`
}

// KBGraphEdge represents an edge in the knowledge graph visualization.
type KBGraphEdge struct {
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	EdgeType string  `json:"edge_type"`
	Weight   float32 `json:"weight"`
}

// KBGraph holds the knowledge graph data for inline visualization.
type KBGraph struct {
	Nodes       []KBGraphNode `json:"nodes"`
	Edges       []KBGraphEdge `json:"edges"`
	TotalNodes  int           `json:"total_nodes"`
	TotalEdges  int           `json:"total_edges"`
	IsTruncated bool          `json:"is_truncated"`
}

// KBMetrics holds generation performance metrics.
type KBMetrics struct {
	RetrievalDurationMs  int64  `json:"retrieval_duration_ms"`
	GenerationDurationMs int64  `json:"generation_duration_ms"`
	GraphDurationMs      int64  `json:"graph_duration_ms"`
	TotalDurationMs      int64  `json:"total_duration_ms"`
	Model                string `json:"model"`
	Mode                 string `json:"mode"`
	PromptTokens         int    `json:"prompt_tokens,omitempty"`
	CompletionTokens     int    `json:"completion_tokens,omitempty"`
	TotalTokens          int    `json:"total_tokens,omitempty"`
}

// KBResult is the structured result returned to the frontend.
type KBResult struct {
	Answer  string    `json:"answer"`
	Sources KBSources `json:"sources"`
	Graph   *KBGraph  `json:"graph,omitempty"`
	Metrics KBMetrics `json:"metrics"`
}

// --- OpenAI LLM types ---

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmRequest struct {
	Model    string       `json:"model"`
	Messages []llmMessage `json:"messages"`
}

type llmUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type llmResponse struct {
	Choices []struct {
		Message llmMessage `json:"message"`
	} `json:"choices"`
	Usage llmUsage `json:"usage"`
}

// QueryKnowledgeBaseTool implements the agent-as-tool pattern:
// it retrieves context from Edgequake via raw HTTP then calls OpenAI to synthesize an answer.
type QueryKnowledgeBaseTool struct {
	edgequakeBaseURL  string
	openAIAPIKey      string
	modelID           string
	allowedNamespaces map[string]bool // Enforced at runtime — rejects any namespace not in this set
}

// NewQueryKnowledgeBaseTool creates a new knowledge base query tool.
// allowedNamespaces restricts which workspace IDs the tool may query.
// If nil or empty, all namespaces are denied (secure default).
func NewQueryKnowledgeBaseTool(edgequakeBaseURL, openAIAPIKey, modelID string, allowedNamespaces []string) *QueryKnowledgeBaseTool {
	if modelID == "" {
		modelID = "gpt-4o"
	}
	nsSet := make(map[string]bool, len(allowedNamespaces))
	for _, ns := range allowedNamespaces {
		nsSet[ns] = true
	}
	return &QueryKnowledgeBaseTool{
		edgequakeBaseURL:  edgequakeBaseURL,
		openAIAPIKey:      openAIAPIKey,
		modelID:           modelID,
		allowedNamespaces: nsSet,
	}
}

func (t *QueryKnowledgeBaseTool) Name() string {
	return "query_knowledge_base"
}

func (t *QueryKnowledgeBaseTool) Description() string {
	return "Search and retrieve relevant information from a knowledge base workspace. Use this tool when the user asks questions that might be answered by documents in the attached knowledge bases. Provide a clear search query and the workspace name (namespace) to search in."
}

func (t *QueryKnowledgeBaseTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type:        agent.TypeObject,
		Description: "Query parameters for knowledge base search",
		Properties: map[string]*agent.Schema{
			"query": {
				Type:        agent.TypeString,
				Description: "The search query to find relevant information in the knowledge base. Be specific and descriptive.",
			},
			"namespace": {
				Type:        agent.TypeString,
				Description: "The workspace ID (namespace) to search in. Must be one of the attached knowledge base workspace IDs.",
			},
		},
		Required: []string{"query", "namespace"},
	}
}

// queryArgs represents the tool arguments.
type queryArgs struct {
	Query     string `json:"query"`
	Namespace string `json:"namespace"`
}

// Execute implements the agent-as-tool pattern:
// 1. Retrieve context from Edgequake via raw HTTP (context_only=true, no LLM on the RAG side)
// 2. Build a system prompt with the retrieved context injected
// 3. Call OpenAI to synthesize an answer grounded in the context
// Returns a structured JSON result with answer, sources, and generation metrics.
func (t *QueryKnowledgeBaseTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	return t.executeInternal(ctx, args, nil)
}

// ExecuteStreaming implements the StreamingExecutor interface.
// It streams the LLM answer tokens in real-time via the stream callback,
// while still returning the complete structured result at the end.
func (t *QueryKnowledgeBaseTool) ExecuteStreaming(ctx context.Context, args agent.Arguments, stream agent.StreamCallback) (agent.Result, error) {
	return t.executeInternal(ctx, args, stream)
}

// executeInternal is the shared implementation for both Execute and ExecuteStreaming.
// When stream is non-nil, the LLM answer tokens are streamed in real-time.
func (t *QueryKnowledgeBaseTool) executeInternal(ctx context.Context, args agent.Arguments, stream agent.StreamCallback) (agent.Result, error) {
	totalStart := time.Now()

	var a queryArgs
	if err := args.Decode(&a); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse arguments: %w", err)), nil
	}

	if a.Query == "" {
		return agent.ErrorResult(fmt.Errorf("query parameter is required")), nil
	}
	if a.Namespace == "" {
		return agent.ErrorResult(fmt.Errorf("namespace parameter is required")), nil
	}

	// Runtime enforcement: reject namespaces not in the allowed set.
	if len(t.allowedNamespaces) == 0 || !t.allowedNamespaces[a.Namespace] {
		return agent.ErrorResult(fmt.Errorf("namespace %q is not an allowed workspace for this agent", a.Namespace)), nil
	}

	// --- Step 1: Retrieve context from Edgequake via raw HTTP ---

	retrievalStart := time.Now()
	resp, err := t.retrieveContext(ctx, a.Query, a.Namespace)
	retrievalDuration := time.Since(retrievalStart)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("knowledge base query failed: %w", err)), nil
	}

	// --- Build structured sources for the UI ---

	sources := buildStructuredSources(resp.Sources)

	// --- Step 2: Build augmented system prompt with context ---

	contextText := buildContextString(resp.Sources)
	if contextText == "" {
		emptyResult := KBResult{
			Answer:  "No relevant information found in the knowledge base for this query.",
			Sources: sources,
			Graph:   nil,
			Metrics: KBMetrics{
				RetrievalDurationMs: retrievalDuration.Milliseconds(),
				TotalDurationMs:     time.Since(totalStart).Milliseconds(),
				Model:               t.modelID,
				Mode:                resp.Mode,
			},
		}
		return agent.JSONResult(emptyResult), nil
	}

	tmpl, err := template.New("kb-prompt").Parse(DefaultKBSystemPrompt)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse prompt template: %w", err)), nil
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, struct{ ContextText string }{ContextText: contextText}); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to execute prompt template: %w", err)), nil
	}

	// --- Step 3: Call OpenAI to synthesize the answer ---

	genStart := time.Now()
	var answer string
	var llmUsageData llmUsage

	if stream != nil {
		// Streaming path: stream tokens in real-time via callback
		streamedAnswer, usage, err := t.callLLMStreaming(ctx, promptBuf.String(), a.Query, stream)
		if err != nil {
			return agent.ErrorResult(fmt.Errorf("LLM synthesis failed: %w", err)), nil
		}
		answer = streamedAnswer
		llmUsageData = usage
	} else {
		// Non-streaming path: wait for complete response
		llmResp, err := t.callLLMFull(ctx, promptBuf.String(), a.Query)
		if err != nil {
			return agent.ErrorResult(fmt.Errorf("LLM synthesis failed: %w", err)), nil
		}
		if len(llmResp.Choices) > 0 {
			answer = llmResp.Choices[0].Message.Content
		}
		llmUsageData = llmResp.Usage
	}
	genDuration := time.Since(genStart)

	// --- Step 4: Fetch knowledge graph subgraph for visualization ---

	var kbGraph *KBGraph
	var graphDuration time.Duration
	if len(sources.Entities) > 0 {
		graphStart := time.Now()
		kbGraph = t.fetchGraphForEntities(ctx, a.Namespace, sources.Entities)
		graphDuration = time.Since(graphStart)
	}

	result := KBResult{
		Answer:  answer,
		Sources: sources,
		Graph:   kbGraph,
		Metrics: KBMetrics{
			RetrievalDurationMs:  retrievalDuration.Milliseconds(),
			GenerationDurationMs: genDuration.Milliseconds(),
			GraphDurationMs:      graphDuration.Milliseconds(),
			TotalDurationMs:      time.Since(totalStart).Milliseconds(),
			Model:                t.modelID,
			Mode:                 resp.Mode,
			PromptTokens:         llmUsageData.PromptTokens,
			CompletionTokens:     llmUsageData.CompletionTokens,
			TotalTokens:          llmUsageData.TotalTokens,
		},
	}

	return agent.JSONResult(result), nil
}

// retrieveContext makes a raw HTTP POST to the Edgequake /api/v1/query endpoint.
// Mirrors EdgeQuakeRetriever.RetrieveContext from rag_to_openai.go.
func (t *QueryKnowledgeBaseTool) retrieveContext(ctx context.Context, query, workspaceID string) (*queryResponse, error) {
	reqBody, err := json.Marshal(queryRequest{
		Query:             query,
		Mode:              "hybrid",
		ContextOnly:       true, // No LLM generation on the RAG side — we do that ourselves
		IncludeReferences: true,
		EnableRerank:      true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query request: %w", err)
	}

	httpClient := &http.Client{Timeout: 120 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "POST", t.edgequakeBaseURL+"/api/v1/query", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create retrieval request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-ID", workspaceID)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("retrieval API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("retrieval API returned error %d: %s", resp.StatusCode, string(body))
	}

	var queryResp queryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("failed to decode retrieval response: %w", err)
	}

	return &queryResp, nil
}

// callLLMFull sends the augmented prompt and user query to OpenAI and returns the full response.
func (t *QueryKnowledgeBaseTool) callLLMFull(ctx context.Context, systemPrompt, userQuery string) (*llmResponse, error) {
	reqBody, err := json.Marshal(llmRequest{
		Model: t.modelID,
		Messages: []llmMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userQuery},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal LLM request: %w", err)
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+t.openAIAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API returned error %d: %s", resp.StatusCode, string(body))
	}

	var llmResp llmResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return nil, fmt.Errorf("failed to decode LLM response: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	return &llmResp, nil
}

// --- OpenAI Streaming SSE types ---

type llmStreamRequest struct {
	Model    string       `json:"model"`
	Messages []llmMessage `json:"messages"`
	Stream   bool         `json:"stream"`
	// Request usage stats in the final streamed chunk
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type llmStreamDelta struct {
	Content string `json:"content"`
}

type llmStreamChoice struct {
	Delta        llmStreamDelta `json:"delta"`
	FinishReason *string        `json:"finish_reason"`
}

type llmStreamChunk struct {
	Choices []llmStreamChoice `json:"choices"`
	Usage   *llmUsage         `json:"usage,omitempty"` // Only present in final chunk with stream_options
}

// callLLMStreaming sends the augmented prompt to OpenAI with streaming enabled.
// It streams each token via the stream callback and returns the accumulated answer + usage.
func (t *QueryKnowledgeBaseTool) callLLMStreaming(ctx context.Context, systemPrompt, userQuery string, stream agent.StreamCallback) (string, llmUsage, error) {
	reqBody, err := json.Marshal(llmStreamRequest{
		Model: t.modelID,
		Messages: []llmMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userQuery},
		},
		Stream: true,
		StreamOptions: &streamOptions{
			IncludeUsage: true,
		},
	})
	if err != nil {
		return "", llmUsage{}, fmt.Errorf("failed to marshal streaming LLM request: %w", err)
	}

	httpClient := &http.Client{Timeout: 120 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", llmUsage{}, fmt.Errorf("failed to create streaming LLM request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+t.openAIAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", llmUsage{}, fmt.Errorf("OpenAI streaming API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", llmUsage{}, fmt.Errorf("OpenAI streaming API returned error %d: %s", resp.StatusCode, string(body))
	}

	// Parse SSE stream
	var answerBuilder strings.Builder
	var usage llmUsage
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: "data: {json}" or "data: [DONE]"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk llmStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // Skip malformed chunks
		}

		// Extract usage from final chunk
		if chunk.Usage != nil {
			usage = *chunk.Usage
		}

		// Stream content deltas
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta.Content
			if delta != "" {
				answerBuilder.WriteString(delta)
				// Stream the token to the UI
				if !stream(delta, "answer") {
					// Consumer disconnected
					break
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return answerBuilder.String(), usage, fmt.Errorf("error reading SSE stream: %w", err)
	}

	return answerBuilder.String(), usage, nil
}

// buildStructuredSources converts raw source references into structured UI-ready sources.
func buildStructuredSources(sources []sourceReference) KBSources {
	result := KBSources{
		Chunks:        []KBSourceChunk{},
		Entities:      []KBSourceEntity{},
		Relationships: []KBSourceRelationship{},
	}

	for _, s := range sources {
		snippet := ""
		if s.Snippet != nil {
			snippet = *s.Snippet
		}

		switch s.SourceType {
		case "chunk":
			chunk := KBSourceChunk{
				Content:    snippet,
				Score:      s.Score,
				StartLine:  s.StartLine,
				EndLine:    s.EndLine,
				ChunkIndex: s.ChunkIndex,
			}
			if s.DocumentID != nil {
				chunk.DocumentID = *s.DocumentID
			}
			if s.FilePath != nil {
				chunk.FilePath = *s.FilePath
			}
			result.Chunks = append(result.Chunks, chunk)

		case "entity":
			entity := KBSourceEntity{
				ID:        s.ID,
				Label:     s.ID,
				Relevance: s.Score,
			}
			if s.SourceDocumentID != nil {
				entity.SourceDocumentID = *s.SourceDocumentID
			}
			if s.SourceFilePath != nil {
				entity.SourceFilePath = *s.SourceFilePath
			}
			result.Entities = append(result.Entities, entity)

		case "relationship":
			rel := KBSourceRelationship{
				Relevance: s.Score,
			}
			// Parse relationship snippet: typically "source -> TYPE -> target"
			if snippet != "" {
				parts := strings.SplitN(snippet, " -> ", 3)
				if len(parts) == 3 {
					rel.Source = strings.TrimSpace(parts[0])
					rel.Type = strings.TrimSpace(parts[1])
					rel.Target = strings.TrimSpace(parts[2])
				} else {
					rel.Source = snippet
					rel.Type = "related_to"
					rel.Target = ""
				}
			}
			if s.SourceDocumentID != nil {
				rel.SourceDocumentID = *s.SourceDocumentID
			}
			if s.SourceFilePath != nil {
				rel.SourceFilePath = *s.SourceFilePath
			}
			result.Relationships = append(result.Relationships, rel)
		}
	}

	return result
}

// buildContextString formats retrieved sources into a context string for the prompt.
// Mirrors Agent.buildContextString from rag_to_openai.go.
func buildContextString(sources []sourceReference) string {
	if len(sources) == 0 {
		return ""
	}

	var chunks, entities, relationships []string

	for _, s := range sources {
		snippet := ""
		if s.Snippet != nil {
			snippet = *s.Snippet
		}

		switch s.SourceType {
		case "chunk":
			docRef := ""
			if s.DocumentID != nil {
				docRef = *s.DocumentID
			} else if s.FilePath != nil {
				docRef = *s.FilePath
			}
			if docRef != "" {
				chunks = append(chunks, fmt.Sprintf("[%d] (score: %.3f, doc: %s)\n%s", len(chunks)+1, s.Score, docRef, snippet))
			} else {
				chunks = append(chunks, fmt.Sprintf("[%d] (score: %.3f)\n%s", len(chunks)+1, s.Score, snippet))
			}
		case "entity":
			entities = append(entities, fmt.Sprintf("- **%s**: %s", s.ID, snippet))
		case "relationship":
			relationships = append(relationships, fmt.Sprintf("- %s", snippet))
		}
	}

	var sb strings.Builder
	if len(entities) > 0 {
		sb.WriteString("### Knowledge Graph Data (Entities)\n\n")
		sb.WriteString(strings.Join(entities, "\n"))
		sb.WriteString("\n\n")
	}
	if len(relationships) > 0 {
		sb.WriteString("### Knowledge Graph Data (Relationships)\n\n")
		sb.WriteString(strings.Join(relationships, "\n"))
		sb.WriteString("\n\n")
	}
	if len(chunks) > 0 {
		sb.WriteString("### Document Chunks\n\n")
		sb.WriteString(strings.Join(chunks, "\n\n"))
		sb.WriteString("\n")
	}
	return sb.String()
}

// --- Graph API types ---

type graphNodeAPIResponse struct {
	ID          string          `json:"id"`
	Label       string          `json:"label"`
	NodeType    string          `json:"node_type"`
	Description string          `json:"description"`
	Degree      int             `json:"degree"`
	Properties  json.RawMessage `json:"properties"`
}

type graphEdgeAPIResponse struct {
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	EdgeType string  `json:"edge_type"`
	Weight   float32 `json:"weight"`
}

type knowledgeGraphAPIResponse struct {
	Nodes       []graphNodeAPIResponse `json:"nodes"`
	Edges       []graphEdgeAPIResponse `json:"edges"`
	IsTruncated bool                   `json:"is_truncated"`
	TotalNodes  int                    `json:"total_nodes"`
	TotalEdges  int                    `json:"total_edges"`
}

type searchNodesAPIResponse struct {
	Nodes        []graphNodeAPIResponse `json:"nodes"`
	Edges        []graphEdgeAPIResponse `json:"edges"`
	TotalMatches int                    `json:"total_matches"`
	IsTruncated  bool                   `json:"is_truncated"`
}

// fetchGraphForEntities retrieves a knowledge graph subgraph centered on the entities found during retrieval.
// It tries multiple strategies to get the best possible graph:
// 1. Search for each entity to find matching nodes
// 2. Fetch the subgraph centered on the highest-degree entity with depth=2
// 3. Fall back to combining search results into a merged graph
func (t *QueryKnowledgeBaseTool) fetchGraphForEntities(ctx context.Context, workspaceID string, entities []KBSourceEntity) *KBGraph {
	if len(entities) == 0 {
		return nil
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	// Strategy: Search for entity nodes and collect matching node IDs,
	// then fetch a subgraph centered on the top entity.
	var bestStartNode string
	var bestDegree int
	allNodes := make(map[string]graphNodeAPIResponse)
	allEdges := make(map[string]graphEdgeAPIResponse)

	// Search for each entity (limit to top 5 to avoid too many requests)
	searchLimit := 5
	if len(entities) < searchLimit {
		searchLimit = len(entities)
	}

	for i := 0; i < searchLimit; i++ {
		entity := entities[i]
		searchResp, err := t.searchGraphNodes(ctx, httpClient, workspaceID, entity.ID, 10)
		if err != nil {
			continue
		}

		for _, node := range searchResp.Nodes {
			allNodes[node.ID] = node
			if node.Degree > bestDegree {
				bestDegree = node.Degree
				bestStartNode = node.ID
			}
		}
		for _, edge := range searchResp.Edges {
			key := edge.Source + "-" + edge.Target + "-" + edge.EdgeType
			allEdges[key] = edge
		}
	}

	// If we found a high-degree entity, fetch a richer subgraph centered on it
	if bestStartNode != "" {
		graphResp, err := t.fetchSubgraph(ctx, httpClient, workspaceID, bestStartNode, 2, 75)
		if err == nil && len(graphResp.Nodes) > 0 {
			// Merge: keep subgraph nodes + any additional nodes from search
			for _, node := range graphResp.Nodes {
				allNodes[node.ID] = node
			}
			for _, edge := range graphResp.Edges {
				key := edge.Source + "-" + edge.Target + "-" + edge.EdgeType
				allEdges[key] = edge
			}
		}
	}

	// Build the final KBGraph
	if len(allNodes) == 0 {
		return nil
	}

	nodes := make([]KBGraphNode, 0, len(allNodes))
	for _, n := range allNodes {
		nodes = append(nodes, KBGraphNode(n))
	}

	// Only include edges where both source and target are in our node set
	nodeSet := make(map[string]struct{}, len(allNodes))
	for id := range allNodes {
		nodeSet[id] = struct{}{}
	}

	edges := make([]KBGraphEdge, 0, len(allEdges))
	for _, e := range allEdges {
		if _, srcOk := nodeSet[e.Source]; !srcOk {
			continue
		}
		if _, tgtOk := nodeSet[e.Target]; !tgtOk {
			continue
		}
		edges = append(edges, KBGraphEdge(e))
	}

	return &KBGraph{
		Nodes:       nodes,
		Edges:       edges,
		TotalNodes:  len(nodes),
		TotalEdges:  len(edges),
		IsTruncated: len(nodes) >= 75,
	}
}

// searchGraphNodes searches for nodes matching a query via GET /api/v1/graph/nodes/search.
func (t *QueryKnowledgeBaseTool) searchGraphNodes(ctx context.Context, httpClient *http.Client, workspaceID, query string, limit int) (*searchNodesAPIResponse, error) {
	u := fmt.Sprintf("%s/api/v1/graph/nodes/search?q=%s&limit=%d", t.edgequakeBaseURL, url.QueryEscape(query), limit)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Workspace-ID", workspaceID)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search nodes returned %d", resp.StatusCode)
	}

	var result searchNodesAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// fetchSubgraph retrieves a subgraph centered on a start node via GET /api/v1/graph.
func (t *QueryKnowledgeBaseTool) fetchSubgraph(ctx context.Context, httpClient *http.Client, workspaceID, startNode string, depth, maxNodes int) (*knowledgeGraphAPIResponse, error) {
	u := fmt.Sprintf("%s/api/v1/graph?start_node=%s&depth=%d&max_nodes=%d", t.edgequakeBaseURL, url.QueryEscape(startNode), depth, maxNodes)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Workspace-ID", workspaceID)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graph API returned %d", resp.StatusCode)
	}

	var result knowledgeGraphAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// MarshalJSON is a helper for JSON serialization of query results.
func (t *QueryKnowledgeBaseTool) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"name":        t.Name(),
		"description": t.Description(),
	})
}

// Verify QueryKnowledgeBaseTool implements both ExecutableTool and StreamingExecutableTool.
var (
	_ agent.ExecutableTool          = (*QueryKnowledgeBaseTool)(nil)
	_ agent.StreamingExecutableTool = (*QueryKnowledgeBaseTool)(nil)
)
