package user

import (
	"context"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
)

func TestTokenCacheOperations(t *testing.T) {
	// Initialize cache for testing
	cache.Init(context.Background())

	// Create test token data
	testToken := &model.AuthToken{
		UserID:     12345,
		Token:      "test-jwt-token-123",
		ShortToken: "short-token-456",
		ExpiredAt:  time.Now().Add(time.Hour).Unix(),
	}

	// Test caching token
	CacheToken(testToken)

	// Test retrieving token data
	tokenData, found := GetCachedTokenData(testToken.Token)
	assert.True(t, found, "Token should be found in cache")
	assert.Equal(t, testToken.UserID, tokenData.UserID)
	assert.Equal(t, testToken.Token, tokenData.Token)
	assert.Equal(t, testToken.ShortToken, tokenData.ShortToken)
	assert.Equal(t, testToken.ExpiredAt, tokenData.ExpiredAt)

	// Test retrieving by short token
	shortTokenData, found := GetCachedShortTokenData(testToken.ShortToken)
	assert.True(t, found, "Short token should be found in cache")
	assert.Equal(t, testToken.UserID, shortTokenData.UserID)

	// Test cache invalidation
	InvalidateTokenCache(testToken.Token)
	_, found = GetCachedTokenData(testToken.Token)
	assert.False(t, found, "Token should not be found after invalidation")
	_, found = GetCachedShortTokenData(testToken.ShortToken)
	assert.False(t, found, "Short token should not be found after invalidation")
}

func TestUserCacheOperations(t *testing.T) {
	// Initialize cache for testing
	cache.Init(context.Background())

	// Create test user
	testUser := &model.User{
		Name:     "testuser",
		Status:   true,
		Language: "en",
	}
	testUser.ID = 12345

	// Test caching user
	CacheUser(testUser)

	// Test retrieving user
	cachedUser, found := GetCachedUser(testUser.ID)
	assert.True(t, found, "User should be found in cache")
	assert.Equal(t, testUser.Name, cachedUser.Name)
	assert.Equal(t, testUser.ID, cachedUser.ID)
	assert.Equal(t, testUser.Status, cachedUser.Status)

	// Test cache invalidation
	InvalidateUserCache(testUser.ID)
	_, found = GetCachedUser(testUser.ID)
	assert.False(t, found, "User should not be found after invalidation")
}

func TestExpiredTokenHandling(t *testing.T) {
	// Initialize cache for testing
	cache.Init(context.Background())

	// Create expired token
	expiredToken := &model.AuthToken{
		UserID:     12345,
		Token:      "expired-token-123",
		ShortToken: "expired-short-456",
		ExpiredAt:  time.Now().Add(-time.Hour).Unix(), // Expired 1 hour ago
	}

	// Cache the expired token
	CacheToken(expiredToken)

	// Try to retrieve expired token - should return false and clean cache
	_, found := GetCachedTokenData(expiredToken.Token)
	assert.False(t, found, "Expired token should not be returned")

	// Try to retrieve by expired short token - should return false and clean cache
	_, found = GetCachedShortTokenData(expiredToken.ShortToken)
	assert.False(t, found, "Expired short token should not be returned")
}