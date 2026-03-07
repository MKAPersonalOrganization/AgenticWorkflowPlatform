package artifact

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Service implements Service using an S3-compatible backend (MinIO).
type S3Service struct {
	client     *minio.Client
	bucketName string
}

// S3Config holds configuration for the S3Service.
type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	Region          string // Optional
}

// NewS3Service creates a new S3Service.
func NewS3Service(ctx context.Context, config S3Config) (*S3Service, error) {
	// Production-grade HTTP transport for the S3 client:
	//   - Bounded timeouts prevent goroutine leaks under network issues
	//   - Connection pool sized for 100K concurrent users with connection reuse
	//   - KeepAlive keeps connections warm to reduce TLS handshake overhead
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,  // TCP connect timeout
			KeepAlive: 30 * time.Second, // TCP keepalive interval
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second, // Wait for S3 response headers
		ExpectContinueTimeout: 2 * time.Second,  // For 100-continue on PutObject
		MaxIdleConns:          200,              // Total pool across all hosts
		MaxIdleConnsPerHost:   200,              // Single S3 endpoint
		IdleConnTimeout:       90 * time.Second, // Close stale connections
		ForceAttemptHTTP2:     true,
	}

	// Initialize minio client object.
	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure:    config.UseSSL,
		Region:    config.Region,
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize minio client: %w", err)
	}

	// Ensure bucket exists
	exists, err := minioClient.BucketExists(ctx, config.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check if bucket exists: %w", err)
	}
	if !exists {
		// Attempt to create bucket
		err = minioClient.MakeBucket(ctx, config.BucketName, minio.MakeBucketOptions{Region: config.Region})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &S3Service{
		client:     minioClient,
		bucketName: config.BucketName,
	}, nil
}

// getObjectKey constructs the S3 object key.
func (s *S3Service) getObjectKey(userID, sessionID, fileName string) string {
	// S3 keys are forward-slash delimited
	return path.Join(userID, sessionID, fileName)
}

// maxFileNameLen caps filenames to 255 bytes — the POSIX and S3 limit.
const maxFileNameLen = 255

// validateFileName checks for path traversal, control characters, null bytes,
// and excessive length in file names. This is the last line of defence before
// a user-controlled string reaches S3.
func validateFileName(fileName string) error {
	if fileName == "" {
		return fmt.Errorf("fileName is required")
	}
	if len(fileName) > maxFileNameLen {
		return ErrInvalidPath
	}
	if strings.Contains(fileName, "..") || strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") {
		return ErrInvalidPath
	}
	// Reject null bytes and control characters (ASCII 0x00–0x1F, 0x7F).
	for _, c := range fileName {
		if c <= 0x1F || c == 0x7F {
			return ErrInvalidPath
		}
	}
	return nil
}

func (s *S3Service) Save(ctx context.Context, userID, sessionID, fileName, contentType string, content io.Reader) (int64, error) {
	if userID == "" || sessionID == "" || fileName == "" {
		return 0, fmt.Errorf("userID, sessionID, and fileName are required")
	}

	// Check for naming validity (S3 is generous but let's be safe against traversal attempts in logic)
	if err := validateFileName(fileName); err != nil {
		return 0, err
	}

	objectKey := s.getObjectKey(userID, sessionID, fileName)

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	info, err := s.client.PutObject(ctx, s.bucketName, objectKey, content, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to upload object: %w", err)
	}

	return info.Size, nil
}

func (s *S3Service) Load(ctx context.Context, userID, sessionID, fileName string) (io.ReadCloser, ArtifactMeta, error) {
	if err := validateFileName(fileName); err != nil {
		return nil, ArtifactMeta{}, err
	}

	objectKey := s.getObjectKey(userID, sessionID, fileName)

	obj, err := s.client.GetObject(ctx, s.bucketName, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, ArtifactMeta{}, fmt.Errorf("failed to get object: %w", err)
	}

	// Verify existence (GetObject doesn't error immediately if doesn't exist, stat does)
	stat, err := obj.Stat()
	if err != nil {
		_ = obj.Close() // Close underlying if stat fails
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ArtifactMeta{}, ErrNotFound
		}
		return nil, ArtifactMeta{}, fmt.Errorf("failed to stat object: %w", err)
	}

	meta := ArtifactMeta{
		ContentType: stat.ContentType,
		Size:        stat.Size,
	}

	return obj, meta, nil
}

func (s *S3Service) List(ctx context.Context, userID, sessionID string) ([]string, error) {
	if userID == "" || sessionID == "" {
		return nil, fmt.Errorf("userID and sessionID are required")
	}

	prefix := path.Join(userID, sessionID) + "/"

	var files []string

	// ListObjects returns a channel
	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false, // We only want files at this level
	}

	for object := range s.client.ListObjects(ctx, s.bucketName, opts) {
		if object.Err != nil {
			return nil, fmt.Errorf("error listing objects: %w", object.Err)
		}
		// Key is full path "userID/sessionID/fileName"
		// We want just "fileName"
		_, fileName := path.Split(object.Key)
		// If recursive is false, it might return "userID/sessionID/" as a directory, ignore it
		if fileName == "" {
			continue
		}
		files = append(files, fileName)
	}

	return files, nil
}

func (s *S3Service) Delete(ctx context.Context, userID, sessionID, fileName string) error {
	if err := validateFileName(fileName); err != nil {
		return err
	}

	objectKey := s.getObjectKey(userID, sessionID, fileName)

	err := s.client.RemoveObject(ctx, s.bucketName, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove object: %w", err)
	}
	return nil
}

// HealthCheck verifies that the S3 backend is reachable by checking
// whether the configured bucket exists. This is lightweight (a single
// HEAD request) and used by readiness probes.
func (s *S3Service) HealthCheck(ctx context.Context) error {
	_, err := s.client.BucketExists(ctx, s.bucketName)
	if err != nil {
		return fmt.Errorf("s3 health check failed: %w", err)
	}
	return nil
}
