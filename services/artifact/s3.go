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
	if strings.Contains(fileName, "..") || strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") {
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
		obj.Close() // Close underlying if stat fails
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to stat object: %w", err)
	}

	return obj, nil
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
	objectKey := s.getObjectKey(userID, sessionID, fileName)

	err := s.client.RemoveObject(ctx, s.bucketName, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove object: %w", err)
	}
	return nil
}
