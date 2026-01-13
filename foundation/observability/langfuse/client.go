package langfuse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Client is a custom Langfuse HTTP client for sending traces
type Client struct {
	baseURL    string
	publicKey  string
	secretKey  string
	httpClient *http.Client
}

// Config holds configuration for the Langfuse client
type Config struct {
	BaseURL    string // e.g., "http://localhost:3000"
	PublicKey  string
	SecretKey  string
	HTTPClient *http.Client // optional, will use default if nil
}

// NewClient creates a new Langfuse client
func NewClient(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &Client{
		baseURL:    cfg.BaseURL,
		publicKey:  cfg.PublicKey,
		secretKey:  cfg.SecretKey,
		httpClient: httpClient,
	}
}

// sendRequest sends an HTTP request to Langfuse API
func (c *Client) sendRequest(ctx context.Context, method, path string, body any) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.publicKey, c.secretKey)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close response body: %w", closeErr)
		}
	}()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("langfuse API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	return err
}

// CreateTrace creates a new trace in Langfuse
func (c *Client) CreateTrace(ctx context.Context, trace *Trace) error {
	return c.sendRequest(ctx, "POST", "/api/public/traces", trace)
}

// CreateGeneration creates a new LLM generation in Langfuse
func (c *Client) CreateGeneration(ctx context.Context, gen *Generation) error {
	return c.sendRequest(ctx, "POST", "/api/public/generations", gen)
}

// CreateSpan creates a new span in Langfuse
func (c *Client) CreateSpan(ctx context.Context, span *Span) error {
	return c.sendRequest(ctx, "POST", "/api/public/spans", span)
}

// CreateEvent creates a new event in Langfuse
func (c *Client) CreateEvent(ctx context.Context, event *Event) error {
	return c.sendRequest(ctx, "POST", "/api/public/events", event)
}

// UpdateGeneration updates an existing generation with completion data
func (c *Client) UpdateGeneration(ctx context.Context, gen *Generation) error {
	// For PATCH, Langfuse expects generationId instead of id
	updatePayload := map[string]any{
		"generationId": gen.ID,
		"traceId":      gen.TraceID,
	}

	if gen.EndTime != nil {
		updatePayload["endTime"] = gen.EndTime
	}
	if gen.Output != nil {
		updatePayload["output"] = gen.Output
	}
	if gen.Usage != nil {
		updatePayload["usage"] = gen.Usage
	}
	if gen.Level != "" {
		updatePayload["level"] = gen.Level
	}
	if gen.StatusMessage != "" {
		updatePayload["statusMessage"] = gen.StatusMessage
	}
	if gen.Metadata != nil {
		updatePayload["metadata"] = gen.Metadata
	}

	return c.sendRequest(ctx, "PATCH", "/api/public/generations", updatePayload)
}

// Trace represents a Langfuse trace
type Trace struct {
	ID        string         `json:"id"`
	Name      string         `json:"name,omitempty"`
	UserID    string         `json:"userId,omitempty"`
	SessionID string         `json:"sessionId,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	Timestamp *time.Time     `json:"timestamp,omitempty"`
	Public    bool           `json:"public,omitempty"`
	Release   string         `json:"release,omitempty"` // Application release version
	Version   string         `json:"version,omitempty"` // Component or model version
}

// Generation represents an LLM generation (completion)
type Generation struct {
	ID                  string         `json:"id"`
	TraceID             string         `json:"traceId"`
	Name                string         `json:"name,omitempty"`
	StartTime           *time.Time     `json:"startTime,omitempty"`
	EndTime             *time.Time     `json:"endTime,omitempty"`
	Model               string         `json:"model,omitempty"`
	ModelParameters     map[string]any `json:"modelParameters,omitempty"`
	Input               any            `json:"input,omitempty"`
	Output              any            `json:"output,omitempty"`
	Metadata            map[string]any `json:"metadata,omitempty"`
	Level               string         `json:"level,omitempty"` // "DEBUG", "DEFAULT", "WARNING", "ERROR"
	StatusMessage       string         `json:"statusMessage,omitempty"`
	ParentObservationID string         `json:"parentObservationId,omitempty"`
	Usage               *Usage         `json:"usage,omitempty"`
	CompletionStartTime *time.Time     `json:"completionStartTime,omitempty"`
}

// Span represents a span within a trace
type Span struct {
	ID                  string         `json:"id"`
	TraceID             string         `json:"traceId"`
	Name                string         `json:"name,omitempty"`
	StartTime           *time.Time     `json:"startTime,omitempty"`
	EndTime             *time.Time     `json:"endTime,omitempty"`
	Input               any            `json:"input,omitempty"`
	Output              any            `json:"output,omitempty"`
	Metadata            map[string]any `json:"metadata,omitempty"`
	Level               string         `json:"level,omitempty"`
	StatusMessage       string         `json:"statusMessage,omitempty"`
	ParentObservationID string         `json:"parentObservationId,omitempty"`
}

// Event represents a discrete event within a trace
type Event struct {
	ID                  string         `json:"id"`
	TraceID             string         `json:"traceId"`
	Name                string         `json:"name,omitempty"`
	StartTime           *time.Time     `json:"startTime,omitempty"`
	Input               any            `json:"input,omitempty"`
	Output              any            `json:"output,omitempty"`
	Metadata            map[string]any `json:"metadata,omitempty"`
	Level               string         `json:"level,omitempty"`
	StatusMessage       string         `json:"statusMessage,omitempty"`
	ParentObservationID string         `json:"parentObservationId,omitempty"`
}

// Usage represents token usage for LLM generations
type Usage struct {
	PromptTokens     int `json:"promptTokens,omitempty"`
	CompletionTokens int `json:"completionTokens,omitempty"`
	TotalTokens      int `json:"totalTokens,omitempty"`
}

// Helper functions

// NewTrace creates a new trace with a generated ID
func NewTrace(name string) *Trace {
	return &Trace{
		ID:   uuid.New().String(),
		Name: name,
	}
}

// NewGeneration creates a new generation with a generated ID
func NewGeneration(traceID, name, model string) *Generation {
	now := time.Now()
	return &Generation{
		ID:        uuid.New().String(),
		TraceID:   traceID,
		Name:      name,
		Model:     model,
		StartTime: &now,
	}
}

// NewSpan creates a new span with a generated ID
func NewSpan(traceID, name string) *Span {
	now := time.Now()
	return &Span{
		ID:        uuid.New().String(),
		TraceID:   traceID,
		Name:      name,
		StartTime: &now,
	}
}

// NewEvent creates a new event with a generated ID
func NewEvent(traceID, name string) *Event {
	now := time.Now()
	return &Event{
		ID:        uuid.New().String(),
		TraceID:   traceID,
		Name:      name,
		StartTime: &now,
	}
}
