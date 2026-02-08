package resources

import (
	"context"
	"fmt"
	"io"
	"time"

	"school-with-ai/services/artifact"

	"github.com/google/uuid"
)

var (
	ErrResourceNotFound = fmt.Errorf("resource not found")
	ErrUnauthorized     = fmt.Errorf("unauthorized")
)

// Repository defines the storage interface for resource metadata.
type Repository interface {
	Create(ctx context.Context, res Resource) error
	List(ctx context.Context, userID string, source ResourceSource) ([]Resource, error)
	GetByID(ctx context.Context, id uuid.UUID) (Resource, error)
	Delete(ctx context.Context, id uuid.UUID, userID string) error
}

// Service manages document resources.
type Service struct {
	repo            Repository
	artifactService artifact.Service
}

// NewService creates a new Resources Service.
func NewService(repo Repository, artifactService artifact.Service) *Service {
	return &Service{
		repo:            repo,
		artifactService: artifactService,
	}
}

// ListResources returns a list of resources for the given user and source.
func (s *Service) ListResources(ctx context.Context, userID string, source ResourceSource) ([]Resource, error) {
	return s.repo.List(ctx, userID, source)
}

const (
	SystemUserID      = "system"
	LibrarySessionID  = "library"
	ResourceSessionID = "resources"
)

// UploadResource handles the upload of a new resource to S3.
func (s *Service) UploadResource(ctx context.Context, userID string, title, fileType string, content io.Reader) (Resource, error) {
	id := uuid.New()

	uploadUserID := userID
	sessionID := ResourceSessionID
	source := SourcePrivate
	var uID *string

	if userID == "" {
		uploadUserID = SystemUserID
		sessionID = LibrarySessionID
		source = SourcePublic
	} else {
		uID = &userID
	}

	// Save to artifact service (S3)
	fileName := id.String() // Use UUID as filename in S3 to avoid collisions
	if _, err := s.artifactService.Save(ctx, uploadUserID, sessionID, fileName, content); err != nil {
		return Resource{}, fmt.Errorf("saving to s3: %w", err)
	}

	res := Resource{
		ID:        id,
		UserID:    uID,
		Title:     title,
		FileType:  fileType,
		FilePath:  fileName, // Store S3 fileName in FilePath field
		Source:    source,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, res); err != nil {
		// Cleanup from S3 if DB insert fails
		_ = s.artifactService.Delete(ctx, uploadUserID, sessionID, fileName)
		return Resource{}, fmt.Errorf("storing metadata: %w", err)
	}

	return res, nil
}

// GetResourceContent returns a reader for the resource content from S3.
func (s *Service) GetResourceContent(ctx context.Context, id uuid.UUID) (Resource, io.ReadCloser, error) {
	res, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Resource{}, nil, err
	}

	userID := SystemUserID
	sessionID := LibrarySessionID
	if res.Source == SourcePrivate && res.UserID != nil {
		userID = *res.UserID
		sessionID = ResourceSessionID
	}

	content, err := s.artifactService.Load(ctx, userID, sessionID, res.FilePath)
	if err != nil {
		return Resource{}, nil, fmt.Errorf("loading from s3: %w", err)
	}

	return res, content, nil
}

// DeleteResource removes a resource from S3 and DB.
func (s *Service) DeleteResource(ctx context.Context, id uuid.UUID, userID string) error {
	res, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Security check: only owner can delete private resources
	if res.Source == SourcePrivate && (res.UserID == nil || *res.UserID != userID) {
		return fmt.Errorf("unauthorized")
	}

	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return err
	}

	delUserID := SystemUserID
	sessionID := LibrarySessionID
	if res.Source == SourcePrivate && res.UserID != nil {
		delUserID = *res.UserID
		sessionID = ResourceSessionID
	}

	return s.artifactService.Delete(ctx, delUserID, sessionID, res.FilePath)
}
