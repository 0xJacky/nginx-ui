//go:build dev

package user

import (
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDevSessionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&devSecureSession{}))
	model.Use(db)
	// The lazy migration runs once per process, so pre-migrating here keeps
	// each test independent of the order they run in.
	devSecureSessionMigration.Do(func() {})
	return db
}

// dropCachedSecureSession forces the next lookup to consult the database, which
// is what happens after a process restart.
func dropCachedSecureSession(sessionId string) {
	cache.Del(secureSessionIDCacheKey(sessionId))
}

func TestDevSecureSessionSurvivesAnEmptyCache(t *testing.T) {
	cache.InitInMemoryCache()
	newDevSessionDB(t)

	sessionID := SetSecureSessionID(42)
	require.True(t, VerifySecureSessionID(sessionID, 42))

	dropCachedSecureSession(sessionID)
	require.True(t, VerifySecureSessionID(sessionID, 42), "the session must be recovered from the database")
	require.False(t, VerifySecureSessionID(sessionID, 43), "another user must not match")
}

func TestDevSecureSessionRejectsExpiredRows(t *testing.T) {
	cache.InitInMemoryCache()
	db := newDevSessionDB(t)

	sessionID := "expired-session"
	require.NoError(t, db.Save(&devSecureSession{
		SessionID: sessionID,
		UserID:    7,
		ExpiresAt: time.Now().Add(-time.Minute).Unix(),
	}).Error)

	require.False(t, VerifySecureSessionID(sessionID, 7))

	var remaining int64
	require.NoError(t, db.Model(&devSecureSession{}).Count(&remaining).Error)
	require.Zero(t, remaining, "expired rows must be pruned")
}
