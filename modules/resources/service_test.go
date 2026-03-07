package resources

import (
	"context"
	"io"
	"strings"
	"testing"

	"school-with-ai/platform/artifact"

	"github.com/google/uuid"
)

// mockRepository is a mock implementation of the repository interface.
type mockRepository struct {
	resources map[uuid.UUID]Resource
}

func (m *mockRepository) Create(ctx context.Context, res Resource) error {
	m.resources[res.ID] = res
	return nil
}

func (m *mockRepository) List(ctx context.Context, userID string, source ResourceSource) ([]Resource, error) {
	var list []Resource
	for _, r := range m.resources {
		if r.Source == source {
			if source == SourcePublic || (r.UserID != nil && *r.UserID == userID) {
				list = append(list, r)
			}
		}
	}
	return list, nil
}

func (m *mockRepository) GetByID(ctx context.Context, id uuid.UUID) (Resource, error) {
	res, ok := m.resources[id]
	if !ok {
		return Resource{}, nil
	}
	return res, nil
}

func (m *mockRepository) Delete(ctx context.Context, id uuid.UUID, userID string) error {
	delete(m.resources, id)
	return nil
}

// mockArtifactService is a mock implementation of the artifact.Service interface.
type mockArtifactService struct {
	files map[string][]byte
}

func (m *mockArtifactService) Save(ctx context.Context, userID, sessionID, fileName, contentType string, content io.Reader) (int64, error) {
	data, _ := io.ReadAll(content)
	key := userID + "/" + sessionID + "/" + fileName
	m.files[key] = data
	return int64(len(data)), nil
}

func (m *mockArtifactService) Load(ctx context.Context, userID, sessionID, fileName string) (io.ReadCloser, artifact.ArtifactMeta, error) {
	key := userID + "/" + sessionID + "/" + fileName
	data, ok := m.files[key]
	if !ok {
		return nil, artifact.ArtifactMeta{}, nil
	}
	return io.NopCloser(strings.NewReader(string(data))), artifact.ArtifactMeta{}, nil
}

func (m *mockArtifactService) List(ctx context.Context, userID, sessionID string) ([]string, error) {
	return nil, nil
}

func (m *mockArtifactService) Delete(ctx context.Context, userID, sessionID, fileName string) error {
	key := userID + "/" + sessionID + "/" + fileName
	delete(m.files, key)
	return nil
}

func (m *mockArtifactService) HealthCheck(ctx context.Context) error {
	return nil
}

func TestUploadResource(t *testing.T) {
	repo := &mockRepository{resources: make(map[uuid.UUID]Resource)}
	artSvc := &mockArtifactService{files: make(map[string][]byte)}
	svc := newService(repo, artSvc)

	ctx := context.Background()
	content := strings.NewReader("test content")

	res, err := svc.uploadResource(ctx, "user-1", "test.txt", "text/plain", content)
	if err != nil {
		t.Fatalf("uploadResource failed: %v", err)
	}

	if res.Title != "test.txt" {
		t.Errorf("Expected title test.txt, got %s", res.Title)
	}

	if res.Source != SourcePrivate {
		t.Errorf("Expected source private, got %s", res.Source)
	}

	// Verify S3 storage
	key := "user-1/resources/" + res.FilePath
	if _, ok := artSvc.files[key]; !ok {
		t.Errorf("File not found in mockArtifactService at %s", key)
	}
}

func TestListResources(t *testing.T) {
	repo := &mockRepository{resources: make(map[uuid.UUID]Resource)}
	artSvc := &mockArtifactService{files: make(map[string][]byte)}
	svc := newService(repo, artSvc)

	ctx := context.Background()

	// Pre-seed some resources
	id1 := uuid.New()
	repo.resources[id1] = Resource{ID: id1, Title: "Public Book", Source: SourcePublic}

	uid := "user-1"
	id2 := uuid.New()
	repo.resources[id2] = Resource{ID: id2, UserID: &uid, Title: "Private Doc", Source: SourcePrivate}

	// Test listing public
	public, _ := svc.listResources(ctx, "user-1", SourcePublic)
	if len(public) != 1 || public[0].Title != "Public Book" {
		t.Errorf("Public listing failed")
	}

	// Test listing private
	private, _ := svc.listResources(ctx, "user-1", SourcePrivate)
	if len(private) != 1 || private[0].Title != "Private Doc" {
		t.Errorf("Private listing failed")
	}
}
