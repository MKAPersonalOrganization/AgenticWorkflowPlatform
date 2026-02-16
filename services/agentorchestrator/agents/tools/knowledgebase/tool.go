package knowledgebase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// --- OpenAI LLM types ---

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmRequest struct {
	Model    string       `json:"model"`
	Messages []llmMessage `json:"messages"`
}

type llmResponse struct {
	Choices []struct {
		Message llmMessage `json:"message"`
	} `json:"choices"`
}

// QueryKnowledgeBaseTool implements the agent-as-tool pattern:
// it retrieves context from Edgequake via raw HTTP then calls OpenAI to synthesize an answer.
type QueryKnowledgeBaseTool struct {
	edgequakeBaseURL string
	openAIAPIKey     string
	modelID          string
}

// NewQueryKnowledgeBaseTool creates a new knowledge base query tool.
func NewQueryKnowledgeBaseTool(edgequakeBaseURL, openAIAPIKey, modelID string) *QueryKnowledgeBaseTool {
	if modelID == "" {
		modelID = "gpt-4o"
	}
	return &QueryKnowledgeBaseTool{
		edgequakeBaseURL: edgequakeBaseURL,
		openAIAPIKey:     openAIAPIKey,
		modelID:          modelID,
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
func (t *QueryKnowledgeBaseTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
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

	// --- Step 1: Retrieve context from Edgequake via raw HTTP ---

	resp, err := t.retrieveContext(ctx, a.Query, a.Namespace)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("knowledge base query failed: %w", err)), nil
	}

	// --- Step 2: Build augmented system prompt with context ---

	contextText := buildContextString(resp.Sources)
	if contextText == "" {
		return agent.TextResult("No relevant information found in the knowledge base for this query."), nil
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

	answer, err := t.callLLM(ctx, promptBuf.String(), a.Query)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("LLM synthesis failed: %w", err)), nil
	}

	return agent.TextResult(answer), nil
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

// callLLM sends the augmented prompt and user query to OpenAI for synthesis.
func (t *QueryKnowledgeBaseTool) callLLM(ctx context.Context, systemPrompt, userQuery string) (string, error) {
	reqBody, err := json.Marshal(llmRequest{
		Model: t.modelID,
		Messages: []llmMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userQuery},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal LLM request: %w", err)
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create LLM request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+t.openAIAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API returned error %d: %s", resp.StatusCode, string(body))
	}

	var llmResp llmResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return "", fmt.Errorf("failed to decode LLM response: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return llmResp.Choices[0].Message.Content, nil
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

// MarshalJSON is a helper for JSON serialization of query results.
func (t *QueryKnowledgeBaseTool) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"name":        t.Name(),
		"description": t.Description(),
	})
}
