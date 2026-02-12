package tasks

import (
"context"
"encoding/json"
"net/http"
"net/http/httptest"
"testing"

"github.com/google/uuid"
)

func TestService_Get(t *testing.T) {
trackID := "test-track-123"
mockResponse := TaskResponse{
TrackID:     trackID,
TenantID:    uuid.New().String(),
WorkspaceID: uuid.New().String(),
TaskType:    "document_processing",
Status:      "processing",
CreatedAt:   "2024-01-01T00:00:00Z",
UpdatedAt:   "2024-01-01T00:00:00Z",
RetryCount:  0,
MaxRetries:  3,
}

server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodGet {
t.Errorf("Expected method GET, got %s", r.Method)
}
if r.URL.Path != "/api/v1/tasks/"+trackID {
t.Errorf("Expected path /api/v1/tasks/%s, got %s", trackID, r.URL.Path)
}
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(mockResponse)
}))
defer server.Close()

service := NewService(server.URL, nil)
resp, err := service.Get(context.Background(), nil, trackID)
if err != nil {
t.Fatalf("Get failed: %v", err)
}

if resp.TrackID != trackID {
t.Errorf("Expected TrackID %s, got %s", trackID, resp.TrackID)
}
if resp.Status != "processing" {
t.Errorf("Expected Status processing, got %s", resp.Status)
}
}

func TestService_List(t *testing.T) {
mockResponse := TaskListResponse{
Tasks: []TaskResponse{
{
TrackID:     "task-1",
TenantID:    uuid.New().String(),
WorkspaceID: uuid.New().String(),
TaskType:    "upload",
Status:      "pending",
CreatedAt:   "2024-01-01T00:00:00Z",
UpdatedAt:   "2024-01-01T00:00:00Z",
RetryCount:  0,
MaxRetries:  3,
},
{
TrackID:     "task-2",
TenantID:    uuid.New().String(),
WorkspaceID: uuid.New().String(),
TaskType:    "document_processing",
Status:      "processing",
CreatedAt:   "2024-01-01T00:00:00Z",
UpdatedAt:   "2024-01-01T00:00:00Z",
RetryCount:  0,
MaxRetries:  3,
},
},
Pagination: PaginationInfo{
Total:      2,
Page:       1,
PageSize:   20,
TotalPages: 1,
},
Statistics: StatisticsInfo{
Pending:    1,
Processing: 1,
Indexed:    0,
Failed:     0,
Cancelled:  0,
},
}

server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodGet {
t.Errorf("Expected method GET, got %s", r.Method)
}
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(mockResponse)
}))
defer server.Close()

service := NewService(server.URL, nil)
resp, err := service.List(context.Background(), nil, nil)
if err != nil {
t.Fatalf("List failed: %v", err)
}

if len(resp.Tasks) != 2 {
t.Errorf("Expected 2 tasks, got %d", len(resp.Tasks))
}
if resp.Pagination.Total != 2 {
t.Errorf("Expected total 2, got %d", resp.Pagination.Total)
}
if resp.Statistics.Pending != 1 {
t.Errorf("Expected 1 pending task, got %d", resp.Statistics.Pending)
}
}

func TestService_Cancel(t *testing.T) {
trackID := "test-track-123"
mockResponse := TaskResponse{
TrackID:     trackID,
TenantID:    uuid.New().String(),
WorkspaceID: uuid.New().String(),
TaskType:    "document_processing",
Status:      "cancelled",
CreatedAt:   "2024-01-01T00:00:00Z",
UpdatedAt:   "2024-01-01T00:00:00Z",
RetryCount:  0,
MaxRetries:  3,
}

server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPost {
t.Errorf("Expected method POST, got %s", r.Method)
}
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(mockResponse)
}))
defer server.Close()

service := NewService(server.URL, nil)
resp, err := service.Cancel(context.Background(), nil, trackID)
if err != nil {
t.Fatalf("Cancel failed: %v", err)
}

if resp.Status != "cancelled" {
t.Errorf("Expected Status cancelled, got %s", resp.Status)
}
}

func TestService_Retry(t *testing.T) {
trackID := "test-track-123"
mockResponse := TaskResponse{
TrackID:     trackID,
TenantID:    uuid.New().String(),
WorkspaceID: uuid.New().String(),
TaskType:    "document_processing",
Status:      "pending",
CreatedAt:   "2024-01-01T00:00:00Z",
UpdatedAt:   "2024-01-01T00:00:00Z",
RetryCount:  1,
MaxRetries:  3,
}

server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPost {
t.Errorf("Expected method POST, got %s", r.Method)
}
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(mockResponse)
}))
defer server.Close()

service := NewService(server.URL, nil)
resp, err := service.Retry(context.Background(), nil, trackID)
if err != nil {
t.Fatalf("Retry failed: %v", err)
}

if resp.Status != "pending" {
t.Errorf("Expected Status pending, got %s", resp.Status)
}
}
