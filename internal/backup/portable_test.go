package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInvalidatePortableCredentials(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "portable.db")
	database, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	require.NoError(t, err)

	statements := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, otp_secret BLOB, recovery_codes BLOB)`,
		`INSERT INTO users (otp_secret, recovery_codes) VALUES (x'01', x'02')`,
		`CREATE TABLE dns_credentials (id INTEGER PRIMARY KEY, config BLOB)`,
		`INSERT INTO dns_credentials (config) VALUES (x'03')`,
		`CREATE TABLE auth_tokens (token TEXT)`,
		`INSERT INTO auth_tokens (token) VALUES ('source-token')`,
		`CREATE TABLE nodes (id INTEGER PRIMARY KEY, token TEXT, encrypted_legacy_secret BLOB, credential_status TEXT, last_credential_use_at DATETIME)`,
		`INSERT INTO nodes (token, encrypted_legacy_secret, credential_status, last_credential_use_at) VALUES ('legacy', x'04', 'active', CURRENT_TIMESTAMP)`,
		`CREATE TABLE node_credentials (id INTEGER PRIMARY KEY, encrypted_private_key BLOB)`,
		`INSERT INTO node_credentials (encrypted_private_key) VALUES (x'05')`,
		`CREATE TABLE node_controller_credentials (id INTEGER PRIMARY KEY, status TEXT, revoked_at DATETIME)`,
		`INSERT INTO node_controller_credentials (status) VALUES ('active')`,
		`CREATE TABLE mcp_service_tokens (id INTEGER PRIMARY KEY, revoked_at DATETIME)`,
		`INSERT INTO mcp_service_tokens DEFAULT VALUES`,
	}
	for _, statement := range statements {
		require.NoError(t, database.Exec(statement).Error, fmt.Sprintf("statement failed: %s", statement))
	}

	require.NoError(t, invalidatePortableCredentials(databasePath))

	var user struct {
		OTPSecret     []byte
		RecoveryCodes []byte
	}
	require.NoError(t, database.Table("users").First(&user).Error)
	assert.Empty(t, user.OTPSecret)
	assert.Empty(t, user.RecoveryCodes)

	var credential struct{ Config []byte }
	require.NoError(t, database.Table("dns_credentials").First(&credential).Error)
	assert.Empty(t, credential.Config)

	var tokenCount int64
	require.NoError(t, database.Table("auth_tokens").Count(&tokenCount).Error)
	assert.Zero(t, tokenCount)

	var node struct {
		Token                 string
		EncryptedLegacySecret []byte
		CredentialStatus      string
		LastCredentialUseAt   *time.Time
	}
	require.NoError(t, database.Table("nodes").First(&node).Error)
	assert.Empty(t, node.Token)
	assert.Empty(t, node.EncryptedLegacySecret)
	assert.Equal(t, "unpaired", node.CredentialStatus)
	assert.Nil(t, node.LastCredentialUseAt)

	var nodeCredentialCount int64
	require.NoError(t, database.Table("node_credentials").Count(&nodeCredentialCount).Error)
	assert.Zero(t, nodeCredentialCount)

	var controller struct {
		Status    string
		RevokedAt *time.Time
	}
	require.NoError(t, database.Table("node_controller_credentials").First(&controller).Error)
	assert.Equal(t, "revoked", controller.Status)
	assert.NotNil(t, controller.RevokedAt)

	var serviceToken struct{ RevokedAt *time.Time }
	require.NoError(t, database.Table("mcp_service_tokens").First(&serviceToken).Error)
	assert.NotNil(t, serviceToken.RevokedAt)
}

func TestValidateSQLiteDatabaseRejectsCorruption(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "corrupt.db")
	require.NoError(t, os.WriteFile(databasePath, []byte("not a sqlite database"), 0o600))
	require.Error(t, validateSQLiteDatabase(databasePath))
}
