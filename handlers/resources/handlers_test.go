package resources

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"school-with-ai/domain"
	"school-with-ai/services/resources"

	"github.com/google/uuid"
)

// MockRepository for handler tests
type MockRepository struct {
	Resources map[uuid.UUID]resources.Resource
}

func (m *MockRepository) Create(ctx context.Context, res resources.Resource) error {
	m.Resources[res.ID] = res
	return nil
}

func (m *MockRepository) List(ctx context.Context, userID string, source resources.ResourceSource) ([]resources.Resource, error) {
	var list []resources.Resource
	for _, r := range m.Resources {
		if r.Source == source {
			if source == resources.SourcePublic || (r.UserID != nil && *r.UserID == userID) {
				list = append(list, r)
			}
		}
	}
	return list, nil
}

func (m *MockRepository) GetByID(ctx context.Context, id uuid.UUID) (resources.Resource, error) {
	r, ok := m.Resources[id]
	if !ok {
		return resources.Resource{}, resources.ErrResourceNotFound
	}
	return r, nil
}

func (m *MockRepository) Delete(ctx context.Context, id uuid.UUID, userID string) error {
	delete(m.Resources, id)
	return nil
}

// MockArtifactService for handler tests
type MockArtifactService struct{}

func (m *MockArtifactService) Save(ctx context.Context, userID, sessionID, fileName string, content io.Reader) (int64, error) {
	return 0, nil
}

func (m *MockArtifactService) Load(ctx context.Context, userID, sessionID, fileName string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte("test content"))), nil
}

func (m *MockArtifactService) List(ctx context.Context, userID, sessionID string) ([]string, error) {
	return nil, nil
}

func (m *MockArtifactService) Delete(ctx context.Context, userID, sessionID, fileName string) error {
	return nil
}

func TestUploadHandler(t *testing.T) {
	repo := &MockRepository{Resources: make(map[uuid.UUID]resources.Resource)}
	artSvc := &MockArtifactService{}
	svc := resources.NewService(repo, artSvc)
	h := NewHandler(svc)

	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("hello world"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/resources/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Set user in context
	user := &domain.User{ID: "user-1"}
	ctx := context.WithValue(req.Context(), domain.UserContextKey, user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.Upload(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var res resources.Resource
	json.Unmarshal(rr.Body.Bytes(), &res)
	if res.Title != "test.txt" {
		t.Errorf("Expected title test.txt, got %s", res.Title)
	}
}

func TestListHandler(t *testing.T) {
	repo := &MockRepository{Resources: make(map[uuid.UUID]resources.Resource)}
	artSvc := &MockArtifactService{}
	svc := resources.NewService(repo, artSvc)
	h := NewHandler(svc)

	id1 := uuid.New()
	repo.Resources[id1] = resources.Resource{ID: id1, Title: "Public Doc", Source: resources.SourcePublic}

	req := httptest.NewRequest(http.MethodGet, "/api/resources?source=public", nil)
	user := &domain.User{ID: "user-1"}
	ctx := context.WithValue(req.Context(), domain.UserContextKey, user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var list []resources.Resource
	json.Unmarshal(rr.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Title != "Public Doc" {
		t.Errorf("Listing failed")
	}
}
