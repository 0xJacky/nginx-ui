//go:build dev

package user

import (
	"os"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

// SecureSessionDurationEnv overrides the two factor session window in dev
// builds. It is read from the environment so no runtime setting exposes it.
const SecureSessionDurationEnv = "NGINX_UI_DEV_SECURE_SESSION_MINUTES"

// MaxDevSecureSessionDuration caps the dev override at one day.
const MaxDevSecureSessionDuration = 24 * time.Hour

// SecureSessionDuration is how long a verified two-factor session is accepted.
// Dev builds allow an override so a long UI session does not need repeated
// re-authentication. Release builds ignore the variable entirely.
func SecureSessionDuration() time.Duration {
	raw := os.Getenv(SecureSessionDurationEnv)
	if raw == "" {
		return DefaultSecureSessionDuration
	}

	minutes := cast.ToInt(raw)
	if minutes <= 0 {
		logger.Warnf("%s=%q is not a positive number of minutes, falling back to %s",
			SecureSessionDurationEnv, raw, DefaultSecureSessionDuration)
		return DefaultSecureSessionDuration
	}

	// Clamp before converting so a huge value cannot overflow the int64.
	if minutes > int(MaxDevSecureSessionDuration/time.Minute) {
		logger.Warnf("%s=%q exceeds the %s cap, using the cap",
			SecureSessionDurationEnv, raw, MaxDevSecureSessionDuration)
		return MaxDevSecureSessionDuration
	}
	return time.Duration(minutes) * time.Minute
}

// devSecureSession persists a verified two factor session so rebuilding the
// backend during development does not force another verification. Release
// builds do not compile this file and keep sessions in memory only.
type devSecureSession struct {
	SessionID string `gorm:"primaryKey;size:64"`
	UserID    uint64 `gorm:"not null"`
	ExpiresAt int64  `gorm:"index;not null"`
}

func (devSecureSession) TableName() string {
	return "dev_secure_sessions"
}

var (
	devSecureSessionMigration sync.Once
	// devSecureSessionMigrationErr outlives the sync.Once so a failure on the
	// first call is still reported on every later one.
	devSecureSessionMigrationErr error
)

// devSecureSessionDB returns the database once the table exists. It returns nil
// before the database is initialised, so callers fall back to the cache.
func devSecureSessionDB() *gorm.DB {
	db := model.UseDB()
	if db == nil {
		return nil
	}
	devSecureSessionMigration.Do(func() {
		devSecureSessionMigrationErr = db.AutoMigrate(&devSecureSession{})
		if devSecureSessionMigrationErr != nil {
			logger.Warnf("dev secure session table is unavailable, falling back to the cache: %v",
				devSecureSessionMigrationErr)
		}
	})
	if devSecureSessionMigrationErr != nil {
		return nil
	}
	return db
}

func storeSecureSession(sessionId string, userId uint64, ttl time.Duration) {
	setCachedSecureSession(sessionId, userId, ttl)

	db := devSecureSessionDB()
	if db == nil {
		return
	}
	record := devSecureSession{
		SessionID: sessionId,
		UserID:    userId,
		ExpiresAt: time.Now().Add(ttl).Unix(),
	}
	if err := db.Save(&record).Error; err != nil {
		logger.Warnf("could not persist the dev secure session: %v", err)
	}
}

func lookupSecureSession(sessionId string) (uint64, bool) {
	if userId, ok := lookupCachedSecureSession(sessionId); ok {
		return userId, true
	}

	db := devSecureSessionDB()
	if db == nil {
		return 0, false
	}

	now := time.Now().Unix()
	// Drop anything already expired so the table cannot grow without bound.
	db.Where("expires_at <= ?", now).Delete(&devSecureSession{})

	var record devSecureSession
	if err := db.Where("session_id = ? AND expires_at > ?", sessionId, now).First(&record).Error; err != nil {
		return 0, false
	}

	// Warm the cache so the next lookup in this process does not hit the DB.
	setCachedSecureSession(sessionId, record.UserID, time.Until(time.Unix(record.ExpiresAt, 0)))
	return record.UserID, true
}
