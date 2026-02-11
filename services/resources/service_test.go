package resources

import (
	"context"
	"io"
	"strings"
	"testing"

	"school-with-ai/services/artifact"

	"github.com/google/uuid"
)

// MockRepository is a mock implementation of the Repository interface.
type MockRepository struct {
	Resources map[uuid.UUID]Resource
}

func (m *MockRepository) Create(ctx context.Context, res Resource) error {
	m.Resources[res.ID] = res
	return nil
}

func (m *MockRepository) List(ctx context.Context, userID string, source ResourceSource) ([]Resource, error) {
	var list []Resource
	for _, r := range m.Resources {
		if r.Source == source {
			if source == SourcePublic || (r.UserID != nil && *r.UserID == userID) {
				list = append(list, r)
			}
		}
	}
	return list, nil
}

func (m *MockRepository) GetByID(ctx context.Context, id uuid.UUID) (Resource, error) {
	res, ok := m.Resources[id]
	if !ok {
		return Resource{}, nil // Should return error in real repo if not found
	}
	return res, nil
}

func (m *MockRepository) Delete(ctx context.Context, id uuid.UUID, userID string) error {
	delete(m.Resources, id)
	return nil
}

// MockArtifactService is a mock implementation of the artifact.Service interface.
type MockArtifactService struct {
	Files map[string][]byte
}

func (m *MockArtifactService) Save(ctx context.Context, userID, sessionID, fileName string, content io.Reader) (int64, error) {
	data, _ := io.ReadAll(content)
	key := userID + "/" + sessionID + "/" + fileName
	m.Files[key] = data
	return int64(len(data)), nil
}

func (m *MockArtifactService) Load(ctx context.Context, userID, sessionID, fileName string) (io.ReadCloser, error) {
	key := userID + "/" + sessionID + "/" + fileName
	data, ok := m.Files[key]
	if !ok {
		return nil, nil // Error in real s3
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (m *MockArtifactService) CreateDirectory(ctx context.Context, userID, sessionID, path string) error {
	return nil
}

func (m *MockArtifactService) List(ctx context.Context, userID, sessionID, prefix string) ([]artifact.ArtifactInfo, error) {
	return nil, nil
}

func (m *MockArtifactService) Delete(ctx context.Context, userID, sessionID, fileName string) error {
	key := userID + "/" + sessionID + "/" + fileName
	delete(m.Files, key)
	return nil
}

func TestUploadResource(t *testing.T) {
	repo := &MockRepository{Resources: make(map[uuid.UUID]Resource)}
	artSvc := &MockArtifactService{Files: make(map[string][]byte)}
	svc := NewService(repo, artSvc)

	ctx := context.Background()
	content := strings.NewReader("test content")

	res, err := svc.UploadResource(ctx, "user-1", "test.txt", "text/plain", content)
	if err != nil {
		t.Fatalf("UploadResource failed: %v", err)
	}

	if res.Title != "test.txt" {
		t.Errorf("Expected title test.txt, got %s", res.Title)
	}

	if res.Source != SourcePrivate {
		t.Errorf("Expected source private, got %s", res.Source)
	}

	// Verify S3 storage
	key := "user-1/resources/" + res.FilePath
	if _, ok := artSvc.Files[key]; !ok {
		t.Errorf("File not found in MockArtifactService at %s", key)
	}
}

func TestListResources(t *testing.T) {
	repo := &MockRepository{Resources: make(map[uuid.UUID]Resource)}
	artSvc := &MockArtifactService{Files: make(map[string][]byte)}
	svc := NewService(repo, artSvc)

	ctx := context.Background()

	// Pre-seed some resources
	id1 := uuid.New()
	repo.Resources[id1] = Resource{ID: id1, Title: "Public Book", Source: SourcePublic}

	uid := "user-1"
	id2 := uuid.New()
	repo.Resources[id2] = Resource{ID: id2, UserID: &uid, Title: "Private Doc", Source: SourcePrivate}

	// Test listing public
	public, _ := svc.ListResources(ctx, "user-1", SourcePublic)
	if len(public) != 1 || public[0].Title != "Public Book" {
		t.Errorf("Public listing failed")
	}

	// Test listing private
	private, _ := svc.ListResources(ctx, "user-1", SourcePrivate)
	if len(private) != 1 || private[0].Title != "Private Doc" {
		t.Errorf("Private listing failed")
	}
}
