package rag

import (
	"context"
	"fmt"
	"io"
	"strings"

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

	// New Drive Operations
	ListByParentPath(ctx context.Context, ownerID, parentPath string, limit, offset int) ([]domain.Document, int, error)
	RenameDocument(ctx context.Context, scope Scope, ownerID string, id uuid.UUID, newName string) error
	MoveDocument(ctx context.Context, scope Scope, ownerID string, id uuid.UUID, targetFolderPath string) error
	TrashDocument(ctx context.Context, id uuid.UUID) error
	RestoreDocument(ctx context.Context, id uuid.UUID) error
	ListTrashed(ctx context.Context, ownerID string, limit, offset int) ([]domain.Document, int, error)
	EmptyTrash(ctx context.Context, scope Scope, ownerID string) error
	DownloadDocument(ctx context.Context, scope Scope, ownerID string, id uuid.UUID) (io.ReadCloser, *domain.Document, error)
	SearchDocuments(ctx context.Context, ownerID, query string, limit, offset int) ([]domain.Document, int, error)
}

// DocumentRepository defines the interface for document storage.
type DocumentRepository interface {
	Save(ctx context.Context, doc *domain.Document) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Document, error)
	GetByS3Key(ctx context.Context, s3Key string) (*domain.Document, error)
	List(ctx context.Context, ownerID string, prefix string, limit, offset int) ([]domain.Document, int, error)
	ListByParentPath(ctx context.Context, ownerID string, parentPath string, limit, offset int) ([]domain.Document, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByPrefix(ctx context.Context, prefix string) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.DocumentStatus) error
	Rename(ctx context.Context, id uuid.UUID, newName string, newS3Key string) error
	Move(ctx context.Context, id uuid.UUID, newParentPath, newName, newS3Key string) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	SoftDeleteByPrefix(ctx context.Context, prefix string) error
	Restore(ctx context.Context, id uuid.UUID) error
	RestoreByPrefix(ctx context.Context, prefix string) error
	ListTrashed(ctx context.Context, ownerID string, limit, offset int) ([]domain.Document, int, error)
	EmptyTrash(ctx context.Context, ownerID string) ([]domain.Document, error)
	SearchByName(ctx context.Context, ownerID, query string, limit, offset int) ([]domain.Document, int, error)
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

// New Drive Operations

func (s *ragService) ListByParentPath(ctx context.Context, ownerID, parentPath string, limit, offset int) ([]domain.Document, int, error) {
	return s.docRepo.ListByParentPath(ctx, ownerID, parentPath, limit, offset)
}

func (s *ragService) RenameDocument(ctx context.Context, scope Scope, ownerID string, id uuid.UUID, newName string) error {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get document: %w", err)
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}

	// Build new full name and S3 key
	newFullName := doc.ParentPath + newName
	if doc.IsDir && !strings.HasSuffix(newFullName, "/") {
		newFullName += "/"
	}

	// For files, we need to copy in S3 and delete old key
	if !doc.IsDir {
		// Load from old key
		reader, err := s.store.Load(ctx, scope, ownerID, doc.S3Key)
		if err != nil {
			return fmt.Errorf("load from storage: %w", err)
		}
		defer reader.Close()

		// Save to new key
		if _, err := s.store.Save(ctx, scope, ownerID, newFullName, reader); err != nil {
			return fmt.Errorf("save to storage: %w", err)
		}

		// Delete old key
		if err := s.store.Delete(ctx, scope, ownerID, doc.S3Key); err != nil {
			return fmt.Errorf("delete old storage key: %w", err)
		}
	}

	return s.docRepo.Rename(ctx, id, newFullName, newFullName)
}

func (s *ragService) MoveDocument(ctx context.Context, scope Scope, ownerID string, id uuid.UUID, targetFolderPath string) error {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get document: %w", err)
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}

	// Extract basename from current name
	basename := doc.Name
	if doc.ParentPath != "" {
		basename = strings.TrimPrefix(doc.Name, doc.ParentPath)
	}

	// Build new path
	newParentPath := targetFolderPath
	if newParentPath != "" && !strings.HasSuffix(newParentPath, "/") {
		newParentPath += "/"
	}
	newFullName := newParentPath + basename
	newS3Key := newFullName

	// Move in S3 (copy + delete) for files
	if !doc.IsDir {
		reader, err := s.store.Load(ctx, scope, ownerID, doc.S3Key)
		if err != nil {
			return fmt.Errorf("load from storage: %w", err)
		}
		defer reader.Close()

		if _, err := s.store.Save(ctx, scope, ownerID, newS3Key, reader); err != nil {
			return fmt.Errorf("save to storage: %w", err)
		}

		if err := s.store.Delete(ctx, scope, ownerID, doc.S3Key); err != nil {
			return fmt.Errorf("delete old storage key: %w", err)
		}
	}

	return s.docRepo.Move(ctx, id, newParentPath, newFullName, newS3Key)
}

func (s *ragService) TrashDocument(ctx context.Context, id uuid.UUID) error {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get document: %w", err)
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}

	// For directories, trash all children too
	if doc.IsDir {
		prefix := doc.Name
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		if err := s.docRepo.SoftDeleteByPrefix(ctx, prefix); err != nil {
			return fmt.Errorf("trash children: %w", err)
		}
	}

	return s.docRepo.SoftDelete(ctx, id)
}

func (s *ragService) RestoreDocument(ctx context.Context, id uuid.UUID) error {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get document: %w", err)
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}

	// For directories, restore all children too
	if doc.IsDir {
		prefix := doc.Name
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		if err := s.docRepo.RestoreByPrefix(ctx, prefix); err != nil {
			return fmt.Errorf("restore children: %w", err)
		}
	}

	return s.docRepo.Restore(ctx, id)
}

func (s *ragService) ListTrashed(ctx context.Context, ownerID string, limit, offset int) ([]domain.Document, int, error) {
	return s.docRepo.ListTrashed(ctx, ownerID, limit, offset)
}

func (s *ragService) EmptyTrash(ctx context.Context, scope Scope, ownerID string) error {
	docs, err := s.docRepo.EmptyTrash(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("empty trash db: %w", err)
	}

	// Delete from S3
	for _, doc := range docs {
		if !doc.IsDir {
			_ = s.store.Delete(ctx, scope, ownerID, doc.S3Key)
		}
	}

	return nil
}

func (s *ragService) DownloadDocument(ctx context.Context, scope Scope, ownerID string, id uuid.UUID) (io.ReadCloser, *domain.Document, error) {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get document: %w", err)
	}
	if doc == nil {
		return nil, nil, fmt.Errorf("document not found")
	}

	reader, err := s.store.Load(ctx, scope, ownerID, doc.S3Key)
	if err != nil {
		return nil, nil, fmt.Errorf("load from storage: %w", err)
	}

	return reader, doc, nil
}

func (s *ragService) SearchDocuments(ctx context.Context, ownerID, query string, limit, offset int) ([]domain.Document, int, error) {
	return s.docRepo.SearchByName(ctx, ownerID, query, limit, offset)
}
