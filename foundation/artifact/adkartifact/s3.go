// Package adkartifact implements google.golang.org/adk/artifact.Service backed
// by S3-compatible storage (MinIO, AWS S3, etc.).
//
// Design for scale (millions of users, hundreds of stateless pods):
//   - Zero in-memory state: all data lives in S3. Any pod can serve any request.
//   - S3 keys are prefix-partitioned by appName/userID — S3 auto-shards by prefix,
//     so hot-user load is distributed across storage partitions.
//   - Versioning uses path-based version numbers (same pattern as ADK-Go's GCS
//     implementation). Version discovery is a prefix-list that touches only the
//     target artifact's key range.
//   - Connection pooling and keep-alive are configured for high concurrency.
//   - User-scoped artifacts ("user:" prefix) are stored under a well-known
//     "user" session ID, accessible across all sessions for a given app+user.
package adkartifact

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"google.golang.org/adk/artifact"
	"google.golang.org/genai"
)

// S3Config holds configuration for the S3-backed artifact service.
type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	Region          string
}

// s3Service implements artifact.Service using S3-compatible storage.
type s3Service struct {
	client     *minio.Client
	bucketName string
}

// NewS3Service creates a new S3-backed ADK artifact service.
func NewS3Service(ctx context.Context, cfg S3Config) (artifact.Service, error) {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 2 * time.Second,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   200,
		IdleConnTimeout:       90 * time.Second,
		ForceAttemptHTTP2:     true,
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure:    cfg.UseSSL,
		Region:    cfg.Region,
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("adkartifact: failed to init minio client: %w", err)
	}

	exists, err := client.BucketExists(ctx, cfg.BucketName)
	if err != nil {
		return nil, fmt.Errorf("adkartifact: failed to check bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, fmt.Errorf("adkartifact: failed to create bucket: %w", err)
		}
	}

	return &s3Service{client: client, bucketName: cfg.BucketName}, nil
}

// userScopedSessionID is the sentinel session ID for user-scoped artifacts.
const userScopedSessionID = "user"

// fileHasUserNamespace checks if a filename indicates a user-scoped artifact.
func fileHasUserNamespace(filename string) bool {
	return strings.HasPrefix(filename, "user:")
}

// effectiveSessionID returns "user" for user-scoped artifacts, otherwise the real session ID.
func effectiveSessionID(sessionID, fileName string) string {
	if fileHasUserNamespace(fileName) {
		return userScopedSessionID
	}
	return sessionID
}

// objectKey builds the S3 key: appName/userID/sessionID/fileName/version
func objectKey(appName, userID, sessionID, fileName string, version int64) string {
	sid := effectiveSessionID(sessionID, fileName)
	return fmt.Sprintf("%s/%s/%s/%s/%d", appName, userID, sid, fileName, version)
}

// objectPrefix builds the S3 prefix for listing versions of an artifact.
func objectPrefix(appName, userID, sessionID, fileName string) string {
	sid := effectiveSessionID(sessionID, fileName)
	return fmt.Sprintf("%s/%s/%s/%s/", appName, userID, sid, fileName)
}

// sessionPrefix builds the S3 prefix for listing all artifacts in a session.
func sessionPrefix(appName, userID, sessionID string) string {
	return fmt.Sprintf("%s/%s/%s/", appName, userID, sessionID)
}

// userPrefix builds the S3 prefix for listing user-scoped artifacts.
func userPrefix(appName, userID string) string {
	return fmt.Sprintf("%s/%s/%s/", appName, userID, userScopedSessionID)
}

// Save implements artifact.Service.
func (s *s3Service) Save(ctx context.Context, req *artifact.SaveRequest) (*artifact.SaveResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("adkartifact: %w", err)
	}

	// Determine next version by listing existing versions.
	nextVersion := int64(1)
	versions, err := s.listVersions(ctx, req.AppName, req.UserID, req.SessionID, req.FileName)
	if err != nil {
		return nil, fmt.Errorf("adkartifact: listing versions for save: %w", err)
	}
	if len(versions) > 0 {
		nextVersion = slices.Max(versions) + 1
	}

	key := objectKey(req.AppName, req.UserID, req.SessionID, req.FileName, nextVersion)

	var data []byte
	var contentType string

	if req.Part.InlineData != nil {
		data = req.Part.InlineData.Data
		contentType = req.Part.InlineData.MIMEType
	} else {
		data = []byte(req.Part.Text)
		contentType = "text/plain; charset=utf-8"
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = s.client.PutObject(ctx, s.bucketName, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("adkartifact: upload failed: %w", err)
	}

	return &artifact.SaveResponse{Version: nextVersion}, nil
}

// Load implements artifact.Service.
func (s *s3Service) Load(ctx context.Context, req *artifact.LoadRequest) (*artifact.LoadResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("adkartifact: %w", err)
	}

	version, err := s.resolveVersion(ctx, req.AppName, req.UserID, req.SessionID, req.FileName, req.Version)
	if err != nil {
		return nil, err
	}

	key := objectKey(req.AppName, req.UserID, req.SessionID, req.FileName, version)

	obj, err := s.client.GetObject(ctx, s.bucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("adkartifact: get object failed: %w", err)
	}
	defer func() { _ = obj.Close() }()

	stat, err := obj.Stat()
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, fmt.Errorf("artifact not found: %w", fs.ErrNotExist)
		}
		return nil, fmt.Errorf("adkartifact: stat failed: %w", err)
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("adkartifact: read failed: %w", err)
	}

	part := genai.NewPartFromBytes(data, stat.ContentType)
	return &artifact.LoadResponse{Part: part}, nil
}

// Delete implements artifact.Service.
func (s *s3Service) Delete(ctx context.Context, req *artifact.DeleteRequest) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("adkartifact: %w", err)
	}

	if req.Version != 0 {
		key := objectKey(req.AppName, req.UserID, req.SessionID, req.FileName, req.Version)
		return s.client.RemoveObject(ctx, s.bucketName, key, minio.RemoveObjectOptions{})
	}

	// Delete all versions.
	versions, err := s.listVersions(ctx, req.AppName, req.UserID, req.SessionID, req.FileName)
	if err != nil {
		return fmt.Errorf("adkartifact: listing versions for delete: %w", err)
	}
	for _, v := range versions {
		key := objectKey(req.AppName, req.UserID, req.SessionID, req.FileName, v)
		if err := s.client.RemoveObject(ctx, s.bucketName, key, minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("adkartifact: delete version %d failed: %w", v, err)
		}
	}
	return nil
}

// List implements artifact.Service.
func (s *s3Service) List(ctx context.Context, req *artifact.ListRequest) (*artifact.ListResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("adkartifact: %w", err)
	}

	files := map[string]bool{}

	// Session-scoped artifacts.
	if err := s.collectFileNames(ctx, sessionPrefix(req.AppName, req.UserID, req.SessionID), files); err != nil {
		return nil, fmt.Errorf("adkartifact: listing session artifacts: %w", err)
	}

	// User-scoped artifacts.
	if err := s.collectFileNames(ctx, userPrefix(req.AppName, req.UserID), files); err != nil {
		return nil, fmt.Errorf("adkartifact: listing user artifacts: %w", err)
	}

	names := make([]string, 0, len(files))
	for f := range files {
		names = append(names, f)
	}
	sort.Strings(names)

	return &artifact.ListResponse{FileNames: names}, nil
}

// Versions implements artifact.Service.
func (s *s3Service) Versions(ctx context.Context, req *artifact.VersionsRequest) (*artifact.VersionsResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("adkartifact: %w", err)
	}

	versions, err := s.listVersions(ctx, req.AppName, req.UserID, req.SessionID, req.FileName)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("artifact not found: %w", fs.ErrNotExist)
	}
	return &artifact.VersionsResponse{Versions: versions}, nil
}

// GetArtifactVersion implements artifact.Service.
func (s *s3Service) GetArtifactVersion(ctx context.Context, req *artifact.GetArtifactVersionRequest) (*artifact.GetArtifactVersionResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("adkartifact: %w", err)
	}

	version, err := s.resolveVersion(ctx, req.AppName, req.UserID, req.SessionID, req.FileName, req.Version)
	if err != nil {
		return nil, err
	}

	key := objectKey(req.AppName, req.UserID, req.SessionID, req.FileName, version)

	stat, err := s.client.StatObject(ctx, s.bucketName, key, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, fmt.Errorf("artifact not found: %w", fs.ErrNotExist)
		}
		return nil, fmt.Errorf("adkartifact: stat failed: %w", err)
	}

	canonicalURI := fmt.Sprintf("s3://%s/%s", s.bucketName, key)

	customMeta := make(map[string]any, len(stat.UserMetadata))
	for k, v := range stat.UserMetadata {
		customMeta[k] = v
	}

	return &artifact.GetArtifactVersionResponse{
		ArtifactVersion: &artifact.ArtifactVersion{
			Version:        version,
			CanonicalURI:   canonicalURI,
			CustomMetadata: customMeta,
			CreateTime:     float64(stat.LastModified.Unix()),
			MimeType:       stat.ContentType,
		},
	}, nil
}

// ── internal helpers ─────────────────────────────────────────────────────────

// listVersions lists all version numbers for an artifact (does NOT error on empty).
func (s *s3Service) listVersions(ctx context.Context, appName, userID, sessionID, fileName string) ([]int64, error) {
	prefix := objectPrefix(appName, userID, sessionID, fileName)

	var versions []int64
	for obj := range s.client.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("listing objects: %w", obj.Err)
		}
		segments := strings.Split(obj.Key, "/")
		if len(segments) == 0 {
			continue
		}
		v, err := strconv.ParseInt(segments[len(segments)-1], 10, 64)
		if err != nil {
			continue // skip non-numeric keys
		}
		versions = append(versions, v)
	}
	return versions, nil
}

// resolveVersion returns the given version if > 0, otherwise finds the latest.
func (s *s3Service) resolveVersion(ctx context.Context, appName, userID, sessionID, fileName string, version int64) (int64, error) {
	if version > 0 {
		return version, nil
	}
	versions, err := s.listVersions(ctx, appName, userID, sessionID, fileName)
	if err != nil {
		return 0, fmt.Errorf("adkartifact: listing versions: %w", err)
	}
	if len(versions) == 0 {
		return 0, fmt.Errorf("artifact not found: %w", fs.ErrNotExist)
	}
	return slices.Max(versions), nil
}

// collectFileNames lists objects under prefix and extracts file names into the set.
// S3 path layout: .../fileName/version — fileName is the second-to-last segment.
func (s *s3Service) collectFileNames(ctx context.Context, prefix string, out map[string]bool) error {
	for obj := range s.client.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return fmt.Errorf("listing objects: %w", obj.Err)
		}
		segments := strings.Split(obj.Key, "/")
		if len(segments) < 2 {
			continue
		}
		fileName := segments[len(segments)-2]
		if fileName != "" {
			out[fileName] = true
		}
	}
	return nil
}

var _ artifact.Service = (*s3Service)(nil)
