package user

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

const (
	// Cache key prefixes
	tokenCachePrefix      = "auth_token:"
	shortTokenCachePrefix = "short_token:"
	userCachePrefix       = "user:"
	
	// Cache TTL
	tokenCacheTTL = 24 * time.Hour
)

// TokenCacheData stores token information in cache
type TokenCacheData struct {
	UserID     uint64    `json:"user_id"`
	Token      string    `json:"token"`
	ShortToken string    `json:"short_token"`
	ExpiredAt  int64     `json:"expired_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// UserCacheData stores user information in cache
type UserCacheData struct {
	*model.User
	CachedAt time.Time `json:"cached_at"`
}

var (
	cacheMutex = &sync.RWMutex{}
)

// InitTokenCache loads all active tokens into cache on startup
func InitTokenCache(ctx context.Context) {
	logger.Info("Initializing token cache...")
	
	q := query.AuthToken
	authTokens, err := q.Where(q.ExpiredAt.Gte(time.Now().Unix())).Find()
	if err != nil {
		logger.Error("Failed to load auth tokens:", err)
		return
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	loaded := 0
	for _, authToken := range authTokens {
		cacheData := &TokenCacheData{
			UserID:     authToken.UserID,
			Token:      authToken.Token,
			ShortToken: authToken.ShortToken,
			ExpiredAt:  authToken.ExpiredAt,
			CreatedAt:  time.Now(),
		}

		// Cache by token
		if authToken.Token != "" {
			tokenKey := tokenCachePrefix + authToken.Token
			cache.Set(tokenKey, cacheData, tokenCacheTTL)
		}

		// Cache by short token
		if authToken.ShortToken != "" {
			shortTokenKey := shortTokenCachePrefix + authToken.ShortToken
			cache.Set(shortTokenKey, cacheData, tokenCacheTTL)
		}
		
		loaded++
	}

	logger.Info(fmt.Sprintf("Loaded %d auth tokens into cache", loaded))
}

// CacheToken stores a token in cache
func CacheToken(authToken *model.AuthToken) {
	if authToken == nil {
		return
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	cacheData := &TokenCacheData{
		UserID:     authToken.UserID,
		Token:      authToken.Token,
		ShortToken: authToken.ShortToken,
		ExpiredAt:  authToken.ExpiredAt,
		CreatedAt:  time.Now(),
	}

	// Cache by token
	if authToken.Token != "" {
		tokenKey := tokenCachePrefix + authToken.Token
		cache.Set(tokenKey, cacheData, tokenCacheTTL)
	}

	// Cache by short token
	if authToken.ShortToken != "" {
		shortTokenKey := shortTokenCachePrefix + authToken.ShortToken
		cache.Set(shortTokenKey, cacheData, tokenCacheTTL)
	}
}

// GetCachedTokenData retrieves token data from cache
func GetCachedTokenData(token string) (*TokenCacheData, bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	tokenKey := tokenCachePrefix + token
	data, found := cache.Get(tokenKey)
	if !found {
		return nil, false
	}

	tokenData, ok := data.(*TokenCacheData)
	if !ok {
		// Invalid cache data, remove it
		cache.Del(tokenKey)
		return nil, false
	}

	// Check if token is expired
	if tokenData.ExpiredAt < time.Now().Unix() {
		// Token expired, remove from cache
		cache.Del(tokenKey)
		if tokenData.ShortToken != "" {
			cache.Del(shortTokenCachePrefix + tokenData.ShortToken)
		}
		return nil, false
	}

	return tokenData, true
}

// GetCachedShortTokenData retrieves short token data from cache
func GetCachedShortTokenData(shortToken string) (*TokenCacheData, bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	shortTokenKey := shortTokenCachePrefix + shortToken
	data, found := cache.Get(shortTokenKey)
	if !found {
		return nil, false
	}

	tokenData, ok := data.(*TokenCacheData)
	if !ok {
		// Invalid cache data, remove it
		cache.Del(shortTokenKey)
		return nil, false
	}

	// Check if token is expired
	if tokenData.ExpiredAt < time.Now().Unix() {
		// Token expired, remove from cache
		cache.Del(shortTokenKey)
		if tokenData.Token != "" {
			cache.Del(tokenCachePrefix + tokenData.Token)
		}
		return nil, false
	}

	return tokenData, true
}

// CacheUser stores user data in cache
func CacheUser(user *model.User) {
	if user == nil {
		return
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	userKey := fmt.Sprintf("%s%d", userCachePrefix, user.ID)
	cacheData := &UserCacheData{
		User:     user,
		CachedAt: time.Now(),
	}
	
	cache.Set(userKey, cacheData, tokenCacheTTL)
}

// GetCachedUser retrieves user data from cache
func GetCachedUser(userID uint64) (*model.User, bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	userKey := fmt.Sprintf("%s%d", userCachePrefix, userID)
	data, found := cache.Get(userKey)
	if !found {
		return nil, false
	}

	userData, ok := data.(*UserCacheData)
	if !ok {
		// Invalid cache data, remove it
		cache.Del(userKey)
		return nil, false
	}

	// Check if cache is too old (refresh every hour)
	if time.Since(userData.CachedAt) > time.Hour {
		cache.Del(userKey)
		return nil, false
	}

	return userData.User, true
}

// InvalidateTokenCache removes token from cache
func InvalidateTokenCache(token string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Try to get token data first to also remove short token
	tokenKey := tokenCachePrefix + token
	if data, found := cache.Get(tokenKey); found {
		if tokenData, ok := data.(*TokenCacheData); ok && tokenData.ShortToken != "" {
			cache.Del(shortTokenCachePrefix + tokenData.ShortToken)
		}
	}
	
	cache.Del(tokenKey)
}

// InvalidateUserCache removes user from cache
func InvalidateUserCache(userID uint64) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	userKey := fmt.Sprintf("%s%d", userCachePrefix, userID)
	cache.Del(userKey)
}

// ClearExpiredTokens removes expired tokens from cache
func ClearExpiredTokens() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	now := time.Now().Unix()
	
	// Note: ristretto doesn't provide a way to iterate over all keys
	// Expired tokens will be removed when accessed via GetCachedTokenData/GetCachedShortTokenData
	// or when the cache reaches capacity limits
	
	logger.Debug(fmt.Sprintf("Cache cleanup completed at %d", now))
}