package user

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setup2FAStatusTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Passkey{}))

	model.Use(db)

	return db
}

func TestGet2FAStatusRequiresRecoveryCodeMigrationForLegacyOTPUser(t *testing.T) {
	setup2FAStatusTestDB(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", &model.User{OTPSecret: []byte("encrypted-secret")})

	status := get2FAStatus(c)

	assert.True(t, status.OTPStatus)
	assert.False(t, status.RecoveryCodesGenerated)
	assert.True(t, status.RecoveryCodesMigrationRequired)
}

func TestGet2FAStatusDoesNotRequireMigrationWhenRecoveryCodesExist(t *testing.T) {
	setup2FAStatusTestDB(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", &model.User{
		OTPSecret: []byte("encrypted-secret"),
		RecoveryCodes: model.RecoveryCodes{
			Codes: []*model.RecoveryCode{{Code: "00000-00000"}},
		},
	})

	status := get2FAStatus(c)

	assert.True(t, status.OTPStatus)
	assert.True(t, status.RecoveryCodesGenerated)
	assert.False(t, status.RecoveryCodesMigrationRequired)
}
