package query

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"school-with-ai/domain"

	"github.com/google/uuid"
)

func TestService_Execute(t *testing.T) {
	tenantID := uuid.New()
	workspaceID := uuid.New()
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "user-123",
		Role:        "admin",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			t.Errorf("expected path /api/v1/query, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-Tenant-ID") != tenantID.String() {
			t.Errorf("expected tenant ID %s, got %s", tenantID, r.Header.Get("X-Tenant-ID"))
		}
		if r.Header.Get("X-Workspace-ID") != workspaceID.String() {
			t.Errorf("expected workspace ID %s, got %s", workspaceID, r.Header.Get("X-Workspace-ID"))
		}

		var req QueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Query != "What is AI?" {
			t.Errorf("expected query 'What is AI?', got '%s'", req.Query)
		}

		w.Header().Set("Content-Type", "application/json")
		resp := QueryResponse{
			Answer: "AI is Artificial Intelligence.",
			Mode:   "hybrid",
			Stats: QueryStats{
				TotalTimeMs: 100,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	req := &QueryRequest{
		Query: "What is AI?",
	}

	resp, err := service.Execute(context.Background(), tCtx, req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if resp.Answer != "AI is Artificial Intelligence." {
		t.Errorf("expected answer 'AI is Artificial Intelligence.', got '%s'", resp.Answer)
	}
}

func TestService_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		fmt.Fprint(w, "data: AI\n\n")
		fmt.Fprint(w, "data:  is\n\n")
		fmt.Fprint(w, "data:  cool\n\n")
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	req := &StreamQueryRequest{
		Query: "Tell me about AI",
	}

	textChan, errChan := service.Stream(context.Background(), nil, req)

	var result []string
	for text := range textChan {
		result = append(result, text)
	}

	if err := <-errChan; err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	expected := []string{"AI", " is", " cool"}
	if len(result) != len(expected) {
		t.Fatalf("expected %d segments, got %d", len(expected), len(result))
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("expected segment %d to be '%s', got '%s'", i, v, result[i])
		}
	}
}
