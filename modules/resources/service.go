package resources

import (
	"context"
	"fmt"
	"io"
	"time"

	"school-with-ai/platform/artifact"

	"github.com/google/uuid"
)

var (
	ErrResourceNotFound = fmt.Errorf("resource not found")
	ErrUnauthorized     = fmt.Errorf("unauthorized")
)

// service manages document resources.
type service struct {
	repo            repository
	artifactService artifact.Service
}

// newService creates a new resources service.
func newService(repo repository, artifactService artifact.Service) *service {
	return &service{
		repo:            repo,
		artifactService: artifactService,
	}
}

const (
	systemUserID      = "system"
	librarySessionID  = "library"
	resourceSessionID = "resources"
)

// listResources returns a list of resources for the given user and source.
func (s *service) listResources(ctx context.Context, userID string, source ResourceSource) ([]Resource, error) {
	return s.repo.List(ctx, userID, source)
}

// uploadResource handles the upload of a new resource to S3.
func (s *service) uploadResource(ctx context.Context, userID string, title, fileType string, content io.Reader) (Resource, error) {
	id := uuid.New()

	uploadUserID := userID
	sessionID := resourceSessionID
	source := SourcePrivate
	var uID *string

	if userID == "" {
		uploadUserID = systemUserID
		sessionID = librarySessionID
		source = SourcePublic
	} else {
		uID = &userID
	}

	// Save to artifact service (S3)
	fileName := id.String() // Use UUID as filename in S3 to avoid collisions
	if _, err := s.artifactService.Save(ctx, uploadUserID, sessionID, fileName, fileType, content); err != nil {
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

// getResourceContent returns a reader for the resource content from S3.
func (s *service) getResourceContent(ctx context.Context, id uuid.UUID) (Resource, io.ReadCloser, error) {
	res, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Resource{}, nil, err
	}

	userID := systemUserID
	sessionID := librarySessionID
	if res.Source == SourcePrivate && res.UserID != nil {
		userID = *res.UserID
		sessionID = resourceSessionID
	}

	content, _, err := s.artifactService.Load(ctx, userID, sessionID, res.FilePath)
	if err != nil {
		return Resource{}, nil, fmt.Errorf("loading from s3: %w", err)
	}

	return res, content, nil
}

// deleteResource removes a resource from S3 and DB.
func (s *service) deleteResource(ctx context.Context, id uuid.UUID, userID string) error {
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

	delUserID := systemUserID
	sessionID := librarySessionID
	if res.Source == SourcePrivate && res.UserID != nil {
		delUserID = *res.UserID
		sessionID = resourceSessionID
	}

	return s.artifactService.Delete(ctx, delUserID, sessionID, res.FilePath)
}
