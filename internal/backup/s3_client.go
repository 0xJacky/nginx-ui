package backup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// S3Client wraps the AWS S3 client with backup-specific functionality
type S3Client struct {
	client *s3.Client
	bucket string
}

// NewS3Client creates a new S3 client from auto backup configuration.
// This function initializes the AWS S3 client with the provided credentials and configuration.
//
// Parameters:
//   - autoBackup: The auto backup configuration containing S3 settings
//
// Returns:
//   - *S3Client: Configured S3 client wrapper
//   - error: CosyError if client creation fails
func NewS3Client(autoBackup *model.AutoBackup) (*S3Client, error) {
	// Create AWS configuration with static credentials
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			autoBackup.S3AccessKeyID,
			autoBackup.S3SecretAccessKey,
			"", // session token (not used for static credentials)
		)),
		config.WithRegion(getS3Region(autoBackup.S3Region)),
	)
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrAutoBackupS3Upload, fmt.Sprintf("failed to load AWS config: %v", err))
	}

	// Create S3 client with custom endpoint if provided
	var s3Client *s3.Client
	if autoBackup.S3Endpoint != "" {
		s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(autoBackup.S3Endpoint)
			o.UsePathStyle = true // Use path-style addressing for custom endpoints
		})
	} else {
		s3Client = s3.NewFromConfig(cfg)
	}

	return &S3Client{
		client: s3Client,
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

	// Create upload input
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s3c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		Metadata: map[string]string{
			"uploaded-by":    "nginx-ui",
			"upload-time":    time.Now().UTC().Format(time.RFC3339),
			"content-length": fmt.Sprintf("%d", len(data)),
		},
	}

	// Perform the upload
	_, err := s3c.client.PutObject(ctx, input)
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
func (s3c *S3Client) UploadBackupFiles(ctx context.Context, result *BackupExecutionResult, autoBackup *model.AutoBackup) error {
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

	// Try to head the bucket to verify access
	_, err := s3c.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s3c.bucket),
	})
	if err != nil {
		return cosy.WrapErrorWithParams(ErrAutoBackupS3Upload, fmt.Sprintf("S3 connection test failed: %v", err))
	}

	logger.Infof("S3 connection test successful: bucket=%s", s3c.bucket)
	return nil
}

// getS3Region returns the S3 region, defaulting to us-east-1 if not specified.
// This function ensures a valid region is always provided to the AWS SDK.
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
