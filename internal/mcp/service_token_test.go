package mcp

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupServiceTokenTest(t *testing.T) *gorm.DB {
	t.Helper()
	originalSecret := settings.CryptoSettings.Secret
	originalInstanceID := settings.NodeSettings.InstanceID
	t.Cleanup(func() {
		settings.CryptoSettings.Secret = originalSecret
		settings.NodeSettings.InstanceID = originalInstanceID
		model.Use(nil)
	})
	settings.CryptoSettings.Secret = "mcp-token-test-root"
	settings.NodeSettings.InstanceID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.MCPServiceToken{}))
	model.Use(database)
	return database
}

func TestServiceTokenStoredAsKeyedVerifierAndDisplayedOnce(t *testing.T) {
	database := setupServiceTokenTest(t)
	record, rawToken, err := CreateServiceToken(
		"automation",
		[]string{model.MCPTokenScopeRead},
		nil,
		42,
	)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(rawToken, "nui_pat_"+record.PublicID+"_"))
	assert.NotEmpty(t, record.Verifier)
	assert.False(t, bytes.Contains(record.Verifier, []byte(rawToken)))
	assert.NotContains(t, string(record.Verifier), strings.TrimPrefix(rawToken, "nui_pat_"+record.PublicID+"_"))

	var stored model.MCPServiceToken
	require.NoError(t, database.First(&stored, record.ID).Error)
	assert.Empty(t, stored.LastUsedAt)
	assert.Equal(t, []string{model.MCPTokenScopeRead}, stored.Scopes)

	principal, err := VerifyServiceToken(rawToken, time.Now())
	require.NoError(t, err)
	assert.True(t, principal.HasScope(model.MCPTokenScopeRead))
	assert.False(t, principal.HasScope(model.MCPTokenScopeWrite))

	require.NoError(t, database.First(&stored, record.ID).Error)
	assert.NotNil(t, stored.LastUsedAt)
}

func TestServiceTokenWriteScopeIncludesReadAndRejectsUnsupportedScope(t *testing.T) {
	setupServiceTokenTest(t)
	_, rawToken, err := CreateServiceToken("writer", []string{model.MCPTokenScopeWrite}, nil, 1)
	require.NoError(t, err)
	principal, err := VerifyServiceToken(rawToken, time.Now())
	require.NoError(t, err)
	assert.True(t, principal.HasScope(model.MCPTokenScopeWrite))
	assert.True(t, principal.HasScope(model.MCPTokenScopeRead))

	_, _, err = CreateServiceToken("invalid", []string{"admin"}, nil, 1)
	require.ErrorContains(t, err, "unsupported")
}

func TestServiceTokenExpiryRevocationAndRotation(t *testing.T) {
	setupServiceTokenTest(t)
	expiresAt := time.Now().Add(time.Minute)
	record, originalToken, err := CreateServiceToken(
		"rotating",
		[]string{model.MCPTokenScopeRead, model.MCPTokenScopeWrite},
		&expiresAt,
		1,
	)
	require.NoError(t, err)

	_, rotatedToken, err := RotateServiceToken(record.PublicID)
	require.NoError(t, err)
	assert.NotEqual(t, originalToken, rotatedToken)
	_, err = VerifyServiceToken(originalToken, time.Now())
	require.Error(t, err)
	_, err = VerifyServiceToken(rotatedToken, time.Now())
	require.NoError(t, err)

	_, err = VerifyServiceToken(rotatedToken, expiresAt.Add(time.Second))
	require.ErrorContains(t, err, "expired")

	require.NoError(t, RevokeServiceToken(record.PublicID))
	_, err = VerifyServiceToken(rotatedToken, time.Now())
	require.Error(t, err)
}
