package backup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// S3Client wraps the MinIO client with backup-specific functionality
type S3Client struct {
	client *minio.Client
	bucket string
}

// NewS3Client creates a new S3 client from auto backup configuration.
// This function initializes the MinIO client with the provided credentials and configuration.
//
// Parameters:
//   - autoBackup: The auto backup configuration containing S3 settings
//
// Returns:
//   - *S3Client: Configured S3 client wrapper
//   - error: CosyError if client creation fails
func NewS3Client(autoBackup *model.AutoBackup) (*S3Client, error) {
	// Determine endpoint and SSL settings
	endpoint := autoBackup.S3Endpoint
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}

	var secure bool
	if strings.HasPrefix(endpoint, "https://") {
		secure = true
	}

	// Remove protocol prefix if present
	endpoint = strings.ReplaceAll(endpoint, "https://", "")
	endpoint = strings.ReplaceAll(endpoint, "http://", "")

	// Initialize MinIO client
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(autoBackup.S3AccessKeyID, autoBackup.S3SecretAccessKey, ""),
		Secure: secure,
		Region: getS3Region(autoBackup.S3Region),
	})
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrAutoBackupS3Upload, fmt.Sprintf("failed to create MinIO client: %v", err))
	}

	return &S3Client{
		client: minioClient,
		bucket: autoBackup.S3Bucket,
	}, nil
}

// UploadFile uploads a file to S3 with the specified key.
// This function handles the actual upload operation with proper error handling and logging.
//
// Parameters:
//   - ctx: Context for the upload operation
//   - key: S3 object key (path) for the uploaded file
//   - data: File content to upload
//   - contentType: MIME type of the file content
//
// Returns:
//   - error: CosyError if upload fails
func (s3c *S3Client) UploadFile(ctx context.Context, key string, data []byte, contentType string) error {
	logger.Infof("Uploading file to S3: bucket=%s, key=%s, size=%d bytes", s3c.bucket, key, len(data))

	// Create upload options
	opts := minio.PutObjectOptions{
		ContentType: contentType,
		UserMetadata: map[string]string{
			"uploaded-by":    "nginx-ui",
			"upload-time":    time.Now().UTC().Format(time.RFC3339),
			"content-length": fmt.Sprintf("%d", len(data)),
		},
	}

	// Perform the upload
	_, err := s3c.client.PutObject(ctx, s3c.bucket, key, bytes.NewReader(data), int64(len(data)), opts)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrAutoBackupS3Upload, fmt.Sprintf("failed to upload to S3: %v", err))
	}

	logger.Infof("Successfully uploaded file to S3: bucket=%s, key=%s", s3c.bucket, key)
	return nil
}

// UploadBackupFiles uploads backup files to S3 with proper naming and organization.
// This function handles uploading both the backup file and optional key file.
//
// Parameters:
//   - ctx: Context for the upload operations
//   - result: Backup execution result containing file paths
//   - autoBackup: Auto backup configuration for S3 path construction
//
// Returns:
//   - error: CosyError if any upload fails
func (s3c *S3Client) UploadBackupFiles(ctx context.Context, result *ExecutionResult, autoBackup *model.AutoBackup) error {
	// Read backup file content
	backupData, err := readFileContent(result.FilePath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrAutoBackupS3Upload, fmt.Sprintf("failed to read backup file: %v", err))
	}

	// Construct S3 key for backup file
	backupFileName := filepath.Base(result.FilePath)
	backupKey := constructS3Key(autoBackup.StoragePath, backupFileName)

	// Upload backup file
	if err := s3c.UploadFile(ctx, backupKey, backupData, "application/zip"); err != nil {
		return err
	}

	// Upload key file if it exists (for encrypted backups)
	if result.KeyPath != "" {
		keyData, err := readFileContent(result.KeyPath)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrAutoBackupS3Upload, fmt.Sprintf("failed to read key file: %v", err))
		}

		keyFileName := filepath.Base(result.KeyPath)
		keyKey := constructS3Key(autoBackup.StoragePath, keyFileName)

		if err := s3c.UploadFile(ctx, keyKey, keyData, "text/plain"); err != nil {
			return err
		}
	}

	return nil
}

// TestS3Connection tests the S3 connection and permissions.
// This function verifies that the S3 configuration is valid and accessible.
//
// Parameters:
//   - ctx: Context for the test operation
//
// Returns:
//   - error: CosyError if connection test fails
func (s3c *S3Client) TestS3Connection(ctx context.Context) error {
	logger.Infof("Testing S3 connection: bucket=%s", s3c.bucket)

	// Try to check if the bucket exists and is accessible
	exists, err := s3c.client.BucketExists(ctx, s3c.bucket)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrAutoBackupS3Upload, fmt.Sprintf("S3 connection test failed: %v", err))
	}

	if !exists {
		return cosy.WrapErrorWithParams(ErrAutoBackupS3Upload, fmt.Sprintf("S3 bucket does not exist: %s", s3c.bucket))
	}

	logger.Infof("S3 connection test successful: bucket=%s", s3c.bucket)
	return nil
}

// getS3Region returns the S3 region, defaulting to us-east-1 if not specified.
// This function ensures a valid region is always provided to the MinIO client.
//
// Parameters:
//   - region: The configured S3 region
//
// Returns:
//   - string: Valid AWS region string
func getS3Region(region string) string {
	if region == "" {
		return "us-east-1" // Default region
	}
	return region
}

// constructS3Key constructs a proper S3 object key from storage path and filename.
// This function ensures consistent S3 key formatting across the application.
//
// Parameters:
//   - storagePath: Base storage path in S3
//   - filename: Name of the file
//
// Returns:
//   - string: Properly formatted S3 object key
func constructS3Key(storagePath, filename string) string {
	// Ensure storage path doesn't start with slash and ends with slash
	if storagePath == "" {
		return filename
	}

	// Remove leading slash if present
	if storagePath[0] == '/' {
		storagePath = storagePath[1:]
	}

	// Add trailing slash if not present
	if storagePath[len(storagePath)-1] != '/' {
		storagePath += "/"
	}

	return storagePath + filename
}

// readFileContent reads the entire content of a file into memory.
// This function provides a centralized way to read file content for S3 uploads.
//
// Parameters:
//   - filePath: Path to the file to read
//
// Returns:
//   - []byte: File content
//   - error: Standard error if file reading fails
func readFileContent(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

// TestS3ConnectionForConfig tests S3 connection for a given auto backup configuration.
// This function is used by the API to validate S3 settings before saving.
//
// Parameters:
//   - autoBackup: Auto backup configuration with S3 settings
//
// Returns:
//   - error: CosyError if connection test fails
func TestS3ConnectionForConfig(autoBackup *model.AutoBackup) error {
	s3Client, err := NewS3Client(autoBackup)
	if err != nil {
		return err
	}

	return s3Client.TestS3Connection(context.Background())
}
