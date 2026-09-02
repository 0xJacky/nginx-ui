package mcp

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"gorm.io/gorm"
)

const ServiceTokenPrincipalKey = "ServiceTokenPrincipal"

type ServiceTokenPrincipal struct {
	PublicID  string
	Name      string
	Scopes    []string
	CreatorID uint64
}

func (principal *ServiceTokenPrincipal) HasScope(scope string) bool {
	if principal == nil {
		return false
	}
	if slices.Contains(principal.Scopes, scope) {
		return true
	}
	switch scope {
	case model.MCPTokenScopeRead:
		return slices.Contains(principal.Scopes, model.MCPTokenScopeWrite)
	case model.APITokenScopeRead:
		return slices.Contains(principal.Scopes, model.APITokenScopeWrite)
	default:
		return false
	}
}

func CreateServiceToken(name string, scopes []string, expiresAt *time.Time, creatorID uint64) (*model.MCPServiceToken, string, error) {
	if model.UseDB() == nil {
		return nil, "", errors.New("database unavailable")
	}
	normalizedScopes, err := normalizeServiceTokenScopes(scopes)
	if err != nil {
		return nil, "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", errors.New("token name is required")
	}
	if utf8.RuneCountInString(name) > 64 {
		return nil, "", errors.New("token name must not exceed 64 characters")
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		return nil, "", errors.New("token expiry must be in the future")
	}

	publicID, secret, err := generateServiceTokenParts()
	if err != nil {
		return nil, "", err
	}
	verifier, err := serviceTokenVerifier(publicID, secret)
	if err != nil {
		return nil, "", err
	}
	record := &model.MCPServiceToken{
		PublicID:  publicID,
		Name:      name,
		Verifier:  verifier,
		Scopes:    normalizedScopes,
		ExpiresAt: expiresAt,
		CreatorID: creatorID,
	}
	if err := model.UseDB().Create(record).Error; err != nil {
		return nil, "", err
	}
	return record, formatServiceToken(publicID, secret), nil
}

func RotateServiceToken(publicID string) (*model.MCPServiceToken, string, error) {
	database := model.UseDB()
	if database == nil {
		return nil, "", errors.New("database unavailable")
	}
	var record model.MCPServiceToken
	if err := database.Where(
		"public_id = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)",
		publicID,
		time.Now(),
	).First(&record).Error; err != nil {
		return nil, "", err
	}
	_, secret, err := generateServiceTokenParts()
	if err != nil {
		return nil, "", err
	}
	verifier, err := serviceTokenVerifier(record.PublicID, secret)
	if err != nil {
		return nil, "", err
	}
	if err := database.Model(&record).Updates(map[string]any{
		"verifier":     verifier,
		"last_used_at": nil,
	}).Error; err != nil {
		return nil, "", err
	}
	record.Verifier = verifier
	record.LastUsedAt = nil
	return &record, formatServiceToken(record.PublicID, secret), nil
}

func RevokeServiceToken(publicID string) error {
	now := time.Now()
	result := model.UseDB().Model(&model.MCPServiceToken{}).
		Where("public_id = ? AND revoked_at IS NULL", publicID).
		Update("revoked_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func VerifyServiceToken(rawToken string, now time.Time) (*ServiceTokenPrincipal, error) {
	publicID, secret, err := parseServiceToken(rawToken)
	if err != nil {
		return nil, err
	}
	database := model.UseDB()
	if database == nil {
		return nil, errors.New("database unavailable")
	}
	var record model.MCPServiceToken
	if err := database.Where("public_id = ? AND revoked_at IS NULL", publicID).First(&record).Error; err != nil {
		return nil, errors.New("service token is invalid")
	}
	if record.ExpiresAt != nil && !record.ExpiresAt.After(now) {
		return nil, errors.New("service token is expired")
	}
	expected, err := serviceTokenVerifier(publicID, secret)
	if err != nil {
		return nil, err
	}
	if len(record.Verifier) != len(expected) || subtle.ConstantTimeCompare(record.Verifier, expected) != 1 {
		return nil, errors.New("service token is invalid")
	}
	if err := database.Model(&record).Update("last_used_at", now).Error; err != nil {
		return nil, err
	}
	return &ServiceTokenPrincipal{
		PublicID:  record.PublicID,
		Name:      record.Name,
		Scopes:    append([]string(nil), record.Scopes...),
		CreatorID: record.CreatorID,
	}, nil
}

func generateServiceTokenParts() (string, string, error) {
	publicIDBytes := make([]byte, 12)
	secretBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, publicIDBytes); err != nil {
		return "", "", err
	}
	if _, err := io.ReadFull(rand.Reader, secretBytes); err != nil {
		return "", "", err
	}
	return base64.RawURLEncoding.EncodeToString(publicIDBytes),
		base64.RawURLEncoding.EncodeToString(secretBytes), nil
}

func formatServiceToken(publicID, secret string) string {
	return "nui_pat_" + publicID + "_" + secret
}

func parseServiceToken(rawToken string) (string, string, error) {
	const prefix = "nui_pat_"
	const publicIDLength = 16

	rawToken = strings.TrimSpace(rawToken)
	separatorIndex := len(prefix) + publicIDLength
	if len(rawToken) != separatorIndex+1+43 || !strings.HasPrefix(rawToken, prefix) ||
		rawToken[separatorIndex] != '_' {
		return "", "", errors.New("service token is invalid")
	}
	publicID := rawToken[len(prefix):separatorIndex]
	secret := rawToken[separatorIndex+1:]
	if !validServiceTokenPart(publicID, publicIDLength) || !validServiceTokenPart(secret, 43) {
		return "", "", errors.New("service token is invalid")
	}
	return publicID, secret, nil
}

func validServiceTokenPart(value string, expectedLength int) bool {
	if len(value) != expectedLength {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func serviceTokenVerifier(publicID, secret string) ([]byte, error) {
	rootSecret := strings.TrimSpace(settings.CryptoSettings.Secret)
	instanceID := strings.TrimSpace(settings.NodeSettings.InstanceID)
	if rootSecret == "" || instanceID == "" {
		return nil, errors.New("service token verifier key is unavailable")
	}
	key, err := hkdf.Key(sha256.New, []byte(rootSecret), []byte(instanceID), "nginx-ui/mcp-service-token/hmac-sha256/v1", 32)
	if err != nil {
		return nil, fmt.Errorf("derive MCP service token verifier key: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("nui_pat_"))
	_, _ = mac.Write([]byte(publicID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(secret))
	return mac.Sum(nil), nil
}

func normalizeServiceTokenScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return nil, errors.New("at least one service token scope is required")
	}
	seen := make(map[string]struct{}, len(scopes))
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != model.MCPTokenScopeRead && scope != model.MCPTokenScopeWrite &&
			scope != model.APITokenScopeRead && scope != model.APITokenScopeWrite {
			return nil, fmt.Errorf("unsupported service token scope %q", scope)
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	slices.Sort(result)
	return result, nil
}
