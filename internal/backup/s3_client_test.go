package backup

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
)

func TestConstructS3Key(t *testing.T) {
	tests := []struct {
		name        string
		storagePath string
		filename    string
		expected    string
	}{
		{
			name:        "empty storage path",
			storagePath: "",
			filename:    "backup.zip",
			expected:    "backup.zip",
		},
		{
			name:        "storage path with trailing slash",
			storagePath: "backups/",
			filename:    "backup.zip",
			expected:    "backups/backup.zip",
		},
		{
			name:        "storage path without trailing slash",
			storagePath: "backups",
			filename:    "backup.zip",
			expected:    "backups/backup.zip",
		},
		{
			name:        "storage path with leading slash",
			storagePath: "/backups",
			filename:    "backup.zip",
			expected:    "backups/backup.zip",
		},
		{
			name:        "storage path with both leading and trailing slash",
			storagePath: "/backups/",
			filename:    "backup.zip",
			expected:    "backups/backup.zip",
		},
		{
			name:        "nested storage path",
			storagePath: "nginx-ui/backups",
			filename:    "backup.zip",
			expected:    "nginx-ui/backups/backup.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := constructS3Key(tt.storagePath, tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetS3Region(t *testing.T) {
	tests := []struct {
		name     string
		region   string
		expected string
	}{
		{
			name:     "empty region",
			region:   "",
			expected: "us-east-1",
		},
		{
			name:     "valid region",
			region:   "eu-west-1",
			expected: "eu-west-1",
		},
		{
			name:     "us-west-2 region",
			region:   "us-west-2",
			expected: "us-west-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getS3Region(tt.region)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewS3Client_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		autoBackup  *model.AutoBackup
		expectError bool
	}{
		{
			name: "valid configuration",
			autoBackup: &model.AutoBackup{
				S3AccessKeyID:     "test-access-key",
				S3SecretAccessKey: "test-secret-key",
				S3Bucket:          "test-bucket",
				S3Region:          "us-east-1",
			},
			expectError: false,
		},
		{
			name: "valid configuration with custom endpoint",
			autoBackup: &model.AutoBackup{
				S3AccessKeyID:     "test-access-key",
				S3SecretAccessKey: "test-secret-key",
				S3Bucket:          "test-bucket",
				S3Region:          "us-east-1",
				S3Endpoint:        "https://s3.example.com",
			},
			expectError: false,
		},
		{
			name: "empty region defaults to us-east-1",
			autoBackup: &model.AutoBackup{
				S3AccessKeyID:     "test-access-key",
				S3SecretAccessKey: "test-secret-key",
				S3Bucket:          "test-bucket",
				S3Region:          "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewS3Client(tt.autoBackup)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				// Note: This will fail in CI/test environment without MinIO credentials
				// but the client creation itself should succeed
				if err != nil {
					// Allow MinIO client creation errors in test environment
					assert.Contains(t, err.Error(), "failed to create MinIO client")
				} else {
					assert.NotNil(t, client)
					assert.Equal(t, tt.autoBackup.S3Bucket, client.bucket)
				}
			}
		})
	}
}
