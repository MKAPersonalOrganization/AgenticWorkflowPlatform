package rag

import (
	"context"
	"io"

	"school-with-ai/domain"

	"github.com/google/uuid"
)

type Scope string

const (
	ScopePublic  Scope = "public"
	ScopePrivate Scope = "private"
)

// Service defines the interface for the RAG service.
type Service interface {
	// Upload uploads a single file to the RAG storage.
	Upload(ctx context.Context, scope Scope, ownerID, filename string, content io.Reader) (int64, error)

	// CreateDirectory creates a directory in the workspace.
	CreateDirectory(ctx context.Context, scope Scope, ownerID, path string) error

	// List returns a list of files in the workspace with metadata.
	List(ctx context.Context, scope Scope, ownerID, prefix string) ([]FileInfo, error)

	// Load loads a file from the storage.
	Load(ctx context.Context, scope Scope, ownerID, filename string) (io.ReadCloser, error)

	// Delete deletes a file from the workspace.
	Delete(ctx context.Context, scope Scope, ownerID, filename string) error

	// DB Methods
	SaveDocument(ctx context.Context, doc *domain.Document) error
	GetDocument(ctx context.Context, id uuid.UUID) (*domain.Document, error)
	GetDocumentByS3Key(ctx context.Context, key string) (*domain.Document, error)
	ListDocumentsDB(ctx context.Context, ownerID, prefix string, limit, offset int) ([]domain.Document, int, error)
	DeleteDocumentDB(ctx context.Context, id uuid.UUID) error
}

// DocumentRepository defines the interface for document storage.
type DocumentRepository interface {
	Save(ctx context.Context, doc *domain.Document) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Document, error)
	GetByS3Key(ctx context.Context, s3Key string) (*domain.Document, error)
	List(ctx context.Context, ownerID string, prefix string, limit, offset int) ([]domain.Document, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.DocumentStatus) error
}

type ragService struct {
	store   Store
	docRepo DocumentRepository
}

// NewService creates a new RAG service.
func NewService(store Store, docRepo DocumentRepository) Service {
	return &ragService{
		store:   store,
		docRepo: docRepo,
	}
}

func (s *ragService) Upload(ctx context.Context, scope Scope, ownerID, filename string, content io.Reader) (int64, error) {
	return s.store.Save(ctx, scope, ownerID, filename, content)
}

func (s *ragService) CreateDirectory(ctx context.Context, scope Scope, ownerID, path string) error {
	return s.store.CreateDirectory(ctx, scope, ownerID, path)
}

func (s *ragService) List(ctx context.Context, scope Scope, ownerID, prefix string) ([]FileInfo, error) {
	return s.store.List(ctx, scope, ownerID, prefix)
}

func (s *ragService) Load(ctx context.Context, scope Scope, ownerID, filename string) (io.ReadCloser, error) {
	return s.store.Load(ctx, scope, ownerID, filename)
}

func (s *ragService) Delete(ctx context.Context, scope Scope, ownerID, filename string) error {
	return s.store.Delete(ctx, scope, ownerID, filename)
}

// DB Methods

func (s *ragService) SaveDocument(ctx context.Context, doc *domain.Document) error {
	return s.docRepo.Save(ctx, doc)
}

func (s *ragService) GetDocument(ctx context.Context, id uuid.UUID) (*domain.Document, error) {
	return s.docRepo.GetByID(ctx, id)
}

func (s *ragService) GetDocumentByS3Key(ctx context.Context, key string) (*domain.Document, error) {
	return s.docRepo.GetByS3Key(ctx, key)
}

func (s *ragService) ListDocumentsDB(ctx context.Context, ownerID, prefix string, limit, offset int) ([]domain.Document, int, error) {
	return s.docRepo.List(ctx, ownerID, prefix, limit, offset)
}

func (s *ragService) DeleteDocumentDB(ctx context.Context, id uuid.UUID) error {
	return s.docRepo.Delete(ctx, id)
}
