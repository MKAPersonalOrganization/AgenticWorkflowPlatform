package artifact

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

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
	// Initialize minio client object.
	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
		Region: config.Region,
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

func (s *S3Service) Save(ctx context.Context, userID, sessionID, fileName string, content io.Reader) (int64, error) {
	if userID == "" || sessionID == "" || fileName == "" {
		return 0, fmt.Errorf("userID, sessionID, and fileName are required")
	}

	// Check for naming validity (S3 is generous but let's be safe against traversal attempts in logic)
	// Allow forward slashes for subdirectories, but block traversal and backslashes
	if strings.Contains(fileName, "..") || strings.Contains(fileName, "\\") {
		return 0, ErrInvalidPath
	}

	objectKey := s.getObjectKey(userID, sessionID, fileName)

	// Prepare for PutObject. We need to know size or let PutObject determine it.
	// Since we take io.Reader, if it's mostly a buffer we might know size.
	// PutObject takes objectSize. -1 means unknown.

	// Read into buffer if needed to get size? No, io.Reader can be streamed.
	// However, without size, MinIO client buffers in memory up to part size.
	// Let's assume content is typically small text files for now.

	info, err := s.client.PutObject(ctx, s.bucketName, objectKey, content, -1, minio.PutObjectOptions{
		ContentType: "text/plain", // Default to text/plain for our artifacts
	})
	if err != nil {
		return 0, fmt.Errorf("failed to upload object: %w", err)
	}

	return info.Size, nil
}

func (s *S3Service) Load(ctx context.Context, userID, sessionID, fileName string) (io.ReadCloser, error) {
	objectKey := s.getObjectKey(userID, sessionID, fileName)

	obj, err := s.client.GetObject(ctx, s.bucketName, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	// Verify existence (GetObject doesn't error immediately if doesn't exist, stat does)
	_, err = obj.Stat()
	if err != nil {
		_ = obj.Close() // Close underlying if stat fails
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to stat object: %w", err)
	}

	return obj, nil
}

// CreateDirectory creates a directory (or placeholder) in the artifact store.
func (s *S3Service) CreateDirectory(ctx context.Context, userID, sessionID, pathStr string) error {
	if userID == "" || sessionID == "" || pathStr == "" {
		return fmt.Errorf("userID, sessionID, and path are required")
	}

	// Ensure path ends with /
	if !strings.HasSuffix(pathStr, "/") {
		pathStr += "/"
	}

	// Use generic getObjectKey but manually append / because path.Join strips it
	objectKey := s.getObjectKey(userID, sessionID, pathStr)
	if !strings.HasSuffix(objectKey, "/") {
		objectKey += "/"
	}

	// Create 0-byte object
	_, err := s.client.PutObject(ctx, s.bucketName, objectKey, strings.NewReader(""), 0, minio.PutObjectOptions{
		ContentType: "application/x-directory",
	})
	if err != nil {
		return fmt.Errorf("failed to create directory object: %w", err)
	}

	return nil
}

func (s *S3Service) List(ctx context.Context, userID, sessionID, dirPrefix string) ([]ArtifactInfo, error) {
	if userID == "" || sessionID == "" {
		return nil, fmt.Errorf("userID and sessionID are required")
	}

	// Base prefix: userID/sessionID/
	basePrefix := path.Join(userID, sessionID) + "/"

	// Full prefix to list: userID/sessionID/dirPrefix
	// Ensure dirPrefix ends with / if it's a directory we are listing,
	// unless it's empty (root).
	fullPrefix := basePrefix
	if dirPrefix != "" {
		// Clean the prefix path
		cleanPrefix := strings.Trim(dirPrefix, "/")
		if cleanPrefix != "" {
			fullPrefix += cleanPrefix + "/"
		}
	}

	var artifacts []ArtifactInfo

	// ListObjects returns a channel
	opts := minio.ListObjectsOptions{
		Prefix:    fullPrefix,
		Recursive: false, // We want to emulate directory listing, so not recursive
	}

	for object := range s.client.ListObjects(ctx, s.bucketName, opts) {
		if object.Err != nil {
			return nil, fmt.Errorf("error listing objects: %w", object.Err)
		}

		// Key is full path "userID/sessionID/folder/file"
		// We want just the name relative to fullPrefix
		relativePath := strings.TrimPrefix(object.Key, fullPrefix)

		if relativePath == "" {
			// This is the directory object itself, skip
			continue
		}

		// In MinIO, "Virtual" folders might be returned if they exist in other keys?
		// Since Recursive=false, MinIO returns "common prefixes" as objects ending in /

		isDir := strings.HasSuffix(object.Key, "/")
		name := strings.TrimSuffix(relativePath, "/")

		artifacts = append(artifacts, ArtifactInfo{
			Name:    name,
			Size:    object.Size,
			ModTime: object.LastModified,
			IsDir:   isDir,
		})
	}

	return artifacts, nil
}

func (s *S3Service) Delete(ctx context.Context, userID, sessionID, fileName string) error {
	objectKey := s.getObjectKey(userID, sessionID, fileName)

	err := s.client.RemoveObject(ctx, s.bucketName, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove object: %w", err)
	}
	return nil
}
