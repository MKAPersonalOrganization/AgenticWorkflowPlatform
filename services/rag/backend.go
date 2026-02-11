package rag

import (
	"context"
	"io"

	"school-with-ai/services/artifact"
)

// S3Store implements the Store interface using the artifact.S3Service.
type S3Store struct {
	s3 *artifact.S3Service
}

// NewS3Store creates a new S3Store.
func NewS3Store(s3 *artifact.S3Service) *S3Store {
	return &S3Store{
		s3: s3,
	}
}

const (
	SystemUserID = "system"

	// Session IDs for RAG
	PublicSessionID  = "public_rag"
	PrivateSessionID = "private_rag"
)

func (s *S3Store) resolvePath(scope Scope, ownerID string) (userID, sessionID string) {
	if scope == ScopePublic {
		return SystemUserID, PublicSessionID
	}
	// For private scope, we use the ownerID as userID and a specific sessionID
	return ownerID, PrivateSessionID
}

func (s *S3Store) Save(ctx context.Context, scope Scope, ownerID, filename string, content io.Reader) (int64, error) {
	userID, sessionID := s.resolvePath(scope, ownerID)
	return s.s3.Save(ctx, userID, sessionID, filename, content)
}

func (s *S3Store) Load(ctx context.Context, scope Scope, ownerID, filename string) (io.ReadCloser, error) {
	userID, sessionID := s.resolvePath(scope, ownerID)
	return s.s3.Load(ctx, userID, sessionID, filename)
}

func (s *S3Store) CreateDirectory(ctx context.Context, scope Scope, ownerID, path string) error {
	userID, sessionID := s.resolvePath(scope, ownerID)
	return s.s3.CreateDirectory(ctx, userID, sessionID, path)
}

func (s *S3Store) List(ctx context.Context, scope Scope, ownerID, prefix string) ([]FileInfo, error) {
	userID, sessionID := s.resolvePath(scope, ownerID)
	artifacts, err := s.s3.List(ctx, userID, sessionID, prefix)
	if err != nil {
		return nil, err
	}

	files := make([]FileInfo, len(artifacts))
	for i, a := range artifacts {
		files[i] = FileInfo{
			Name:    a.Name,
			Size:    a.Size,
			ModTime: a.ModTime,
			IsDir:   a.IsDir,
		}
	}
	return files, nil
}

func (s *S3Store) Delete(ctx context.Context, scope Scope, ownerID, filename string) error {
	userID, sessionID := s.resolvePath(scope, ownerID)
	return s.s3.Delete(ctx, userID, sessionID, filename)
}
