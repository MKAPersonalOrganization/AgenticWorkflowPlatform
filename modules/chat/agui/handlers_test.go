package agui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aicompass/modules/chat/projection/agui"
	"aicompass/shared"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// mockProjection implements agui.AguiProjection for testing.
type mockProjection struct {
	threads      []agui.ThreadSummary
	returnError  error
	lastRequest  agui.ThreadListRequest
	callCount    int
	totalThreads int // Total threads in "database"
}

func (m *mockProjection) GetThreadState(ctx context.Context, threadID string) (*agui.AguiThreadState, bool) {
	return nil, false
}

func (m *mockProjection) GetMessages(ctx context.Context, threadID string) []aguievents.Message {
	return nil
}

func (m *mockProjection) ListThreads(ctx context.Context, req agui.ThreadListRequest) (*agui.ThreadListResponse, error) {
	m.lastRequest = req
	m.callCount++

	if m.returnError != nil {
		return nil, m.returnError
	}

	// Simulate pagination
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}

	// Filter based on cursor
	var filtered []agui.ThreadSummary
	includingAfterCursor := req.Cursor == ""

	for _, t := range m.threads {
		if !includingAfterCursor {
			// Check if we've passed the cursor
			if t.UpdatedAt.Format(time.RFC3339Nano) == req.Cursor {
				includingAfterCursor = true
			}
			continue
		}
		if req.Cursor != "" && t.UpdatedAt.Format(time.RFC3339Nano) == req.Cursor {
			continue // Skip the cursor item itself
		}
		filtered = append(filtered, t)
	}

	// Apply limit
	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[:limit]
	}

	response := &agui.ThreadListResponse{
		Threads: filtered,
		HasMore: hasMore,
	}

	if hasMore && len(filtered) > 0 {
		response.NextCursor = filtered[len(filtered)-1].UpdatedAt.Format(time.RFC3339Nano)
	}

	return response, nil
}

// mockProjectionService implements ProjectionService for testing.
type mockProjectionService struct{}

// Helper to create test user context.
func contextWithUser(userID string) context.Context {
	user := &shared.User{ID: userID, Email: "test@example.com"}
	return context.WithValue(context.Background(), shared.UserContextKey, user)
}

func TestListThreads_FirstPage(t *testing.T) {
	// Create mock with 25 threads (more than one page)
	threads := make([]agui.ThreadSummary, 25)
	baseTime := time.Date(2026, 1, 19, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 25; i++ {
		threads[i] = agui.ThreadSummary{
			ID:        fmt.Sprintf("thread-%d", i),
			Title:     fmt.Sprintf("Thread %d", i),
			UpdatedAt: baseTime.Add(-time.Duration(i) * time.Hour), // Descending order
		}
	}

	mock := &mockProjection{threads: threads}
	handler := NewAguiProjectionHandler(mock, &mockProjectionService{})

	// Create request with user context
	req := httptest.NewRequest(http.MethodGet, "/api/agui/threads?limit=20", nil)
	req = req.WithContext(contextWithUser("user-123"))
	rec := httptest.NewRecorder()

	handler.ListThreads(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response agui.ThreadListResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should return 20 threads (limit)
	if len(response.Threads) != 20 {
		t.Errorf("Expected 20 threads, got %d", len(response.Threads))
	}

	// Should have more pages
	if !response.HasMore {
		t.Error("Expected HasMore to be true")
	}

	// Should have a cursor
	if response.NextCursor == "" {
		t.Error("Expected NextCursor to be set")
	}

	// Verify request was made with correct user ID
	if mock.lastRequest.UserID != "user-123" {
		t.Errorf("Expected UserID 'user-123', got %q", mock.lastRequest.UserID)
	}
}

func TestListThreads_WithCursor(t *testing.T) {
	// Create mock with threads
	threads := make([]agui.ThreadSummary, 30)
	baseTime := time.Date(2026, 1, 19, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 30; i++ {
		threads[i] = agui.ThreadSummary{
			ID:        fmt.Sprintf("thread-%d", i),
			Title:     fmt.Sprintf("Thread %d", i),
			UpdatedAt: baseTime.Add(-time.Duration(i) * time.Hour),
		}
	}

	mock := &mockProjection{threads: threads}
	handler := NewAguiProjectionHandler(mock, &mockProjectionService{})

	// Use cursor from thread 19 (index 19)
	cursor := threads[19].UpdatedAt.Format(time.RFC3339Nano)
	req := httptest.NewRequest(http.MethodGet, "/api/agui/threads?limit=20&cursor="+cursor, nil)
	req = req.WithContext(contextWithUser("user-123"))
	rec := httptest.NewRecorder()

	handler.ListThreads(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response agui.ThreadListResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should return remaining 10 threads (30 total - 20 from first page)
	if len(response.Threads) != 10 {
		t.Errorf("Expected 10 threads, got %d", len(response.Threads))
	}

	// Should not have more pages
	if response.HasMore {
		t.Error("Expected HasMore to be false")
	}

	// Verify cursor was passed correctly
	if mock.lastRequest.Cursor != cursor {
		t.Errorf("Expected cursor %q, got %q", cursor, mock.lastRequest.Cursor)
	}
}

func TestListThreads_EmptyResults(t *testing.T) {
	mock := &mockProjection{threads: []agui.ThreadSummary{}}
	handler := NewAguiProjectionHandler(mock, &mockProjectionService{})

	req := httptest.NewRequest(http.MethodGet, "/api/agui/threads", nil)
	req = req.WithContext(contextWithUser("user-123"))
	rec := httptest.NewRecorder()

	handler.ListThreads(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	var response agui.ThreadListResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Threads) != 0 {
		t.Errorf("Expected 0 threads, got %d", len(response.Threads))
	}

	if response.HasMore {
		t.Error("Expected HasMore to be false")
	}
}

func TestListThreads_InvalidCursor(t *testing.T) {
	mock := &mockProjection{
		threads:     []agui.ThreadSummary{},
		returnError: fmt.Errorf("invalid cursor format"),
	}
	handler := NewAguiProjectionHandler(mock, &mockProjectionService{})

	req := httptest.NewRequest(http.MethodGet, "/api/agui/threads?cursor=invalid", nil)
	req = req.WithContext(contextWithUser("user-123"))
	rec := httptest.NewRecorder()

	handler.ListThreads(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}
}

func TestListThreads_InvalidLimit(t *testing.T) {
	mock := &mockProjection{threads: []agui.ThreadSummary{}}
	handler := NewAguiProjectionHandler(mock, &mockProjectionService{})

	req := httptest.NewRequest(http.MethodGet, "/api/agui/threads?limit=abc", nil)
	req = req.WithContext(contextWithUser("user-123"))
	rec := httptest.NewRecorder()

	handler.ListThreads(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}
}

func TestListThreads_NoAuth(t *testing.T) {
	mock := &mockProjection{threads: []agui.ThreadSummary{}}
	handler := NewAguiProjectionHandler(mock, &mockProjectionService{})

	// Request without user context
	req := httptest.NewRequest(http.MethodGet, "/api/agui/threads", nil)
	rec := httptest.NewRecorder()

	handler.ListThreads(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestListThreads_LastPage(t *testing.T) {
	// Create exactly one page of threads
	threads := make([]agui.ThreadSummary, 15)
	baseTime := time.Date(2026, 1, 19, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 15; i++ {
		threads[i] = agui.ThreadSummary{
			ID:        fmt.Sprintf("thread-%d", i),
			Title:     fmt.Sprintf("Thread %d", i),
			UpdatedAt: baseTime.Add(-time.Duration(i) * time.Hour),
		}
	}

	mock := &mockProjection{threads: threads}
	handler := NewAguiProjectionHandler(mock, &mockProjectionService{})

	req := httptest.NewRequest(http.MethodGet, "/api/agui/threads?limit=20", nil)
	req = req.WithContext(contextWithUser("user-123"))
	rec := httptest.NewRecorder()

	handler.ListThreads(rec, req)

	var response agui.ThreadListResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should return all 15 threads
	if len(response.Threads) != 15 {
		t.Errorf("Expected 15 threads, got %d", len(response.Threads))
	}

	// Should not have more pages
	if response.HasMore {
		t.Error("Expected HasMore to be false for last page")
	}

	// NextCursor should be empty
	if response.NextCursor != "" {
		t.Errorf("Expected empty NextCursor, got %q", response.NextCursor)
	}
}
