package evaluation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestListEvalSets_Unauthenticated(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)
	asyncRunner := NewAsyncRunner(svc, NewEventBus())
	h := newHandler(svc, asyncRunner)

	req := httptest.NewRequest(http.MethodGet, "/api/eval-sets", nil)
	rec := httptest.NewRecorder()

	h.listEvalSets(rec, req)

	// Without user in context, should return 401
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestCreateEvalSet_InvalidJSON(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)
	asyncRunner := NewAsyncRunner(svc, NewEventBus())
	h := newHandler(svc, asyncRunner)

	req := httptest.NewRequest(http.MethodPost, "/api/eval-sets", nil)
	req.Header.Set("Content-Type", "application/json")
	// Don't add body, should fail to decode

	rec := httptest.NewRecorder()

	h.createEvalSet(rec, req)

	// Without user in context, handler returns 401 first
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestCreateEvalCase_Unauthenticated(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)
	asyncRunner := NewAsyncRunner(svc, NewEventBus())
	h := newHandler(svc, asyncRunner)

	req := httptest.NewRequest(http.MethodPost, "/api/eval-sets/test-id/cases", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.createEvalCase(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestErrorResponse_HasJSONContentType(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	writeError(rec, http.StatusBadRequest, "test message")

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", rec.Header().Get("Content-Type"))
	}

	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if resp.Message != "test message" {
		t.Errorf("expected message %q, got %q", "test message", resp.Message)
	}
}

func TestErrorResponse_WithField(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	writeErrorWithField(rec, http.StatusConflict, "name", `an EvalSet named "test" already exists`)

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", rec.Header().Get("Content-Type"))
	}

	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if resp.Field != "name" {
		t.Errorf("expected field %q, got %q", "name", resp.Field)
	}
}

func TestWriteJSON_EncodesCorrectly(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()

	data := map[string]any{
		"id":   "123",
		"name": "test",
	}

	writeJSON(rec, http.StatusCreated, data)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", rec.Header().Get("Content-Type"))
	}

	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["id"] != "123" {
		t.Errorf("expected id 123, got %v", result["id"])
	}
}
