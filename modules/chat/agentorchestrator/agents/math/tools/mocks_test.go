package tools

import (
	"context"
	"io"
	"aicompass/platform/artifact"
)

// MockArtifactStore is a mock implementation of artifact.Service for testing.
type MockArtifactStore struct {
	SaveCalled bool
	SaveFunc   func(ctx context.Context, userID, sessionID, fileName, contentType string, content io.Reader) (int64, error)
	LoadFunc   func(ctx context.Context, userID, sessionID, fileName string) (io.ReadCloser, artifact.ArtifactMeta, error)
	ListFunc   func(ctx context.Context, userID, sessionID string) ([]string, error)
	DeleteFunc func(ctx context.Context, userID, sessionID, fileName string) error
}

func (m *MockArtifactStore) Save(ctx context.Context, userID, sessionID, fileName, contentType string, content io.Reader) (int64, error) {
	m.SaveCalled = true
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, userID, sessionID, fileName, contentType, content)
	}
	data, _ := io.ReadAll(content)
	return int64(len(data)), nil
}

func (m *MockArtifactStore) Load(ctx context.Context, userID, sessionID, fileName string) (io.ReadCloser, artifact.ArtifactMeta, error) {
	if m.LoadFunc != nil {
		return m.LoadFunc(ctx, userID, sessionID, fileName)
	}
	return nil, artifact.ArtifactMeta{}, artifact.ErrNotFound
}

func (m *MockArtifactStore) List(ctx context.Context, userID, sessionID string) ([]string, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, userID, sessionID)
	}
	return nil, nil
}

func (m *MockArtifactStore) Delete(ctx context.Context, userID, sessionID, fileName string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, userID, sessionID, fileName)
	}
	return nil
}

func (m *MockArtifactStore) HealthCheck(ctx context.Context) error {
	return nil
}
