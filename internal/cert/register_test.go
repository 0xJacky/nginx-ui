package cert

import (
	"context"
	stderrors "errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupDefaultACMEUserTest gives the test a file-backed database (shared by
// every connection, unlike ":memory:"), the certificate settings and a fake
// CA registration that counts its calls. registerErr, when set, is returned
// by the fake registration.
func setupDefaultACMEUserTest(t *testing.T, email, caDir string) (*gorm.DB, *atomic.Int32, *error) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "acme.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AcmeUser{}))
	model.Use(db)
	query.SetDefault(db)

	previousEmail := settings.CertSettings.Email
	previousCADir := settings.CertSettings.CADir
	previousRegister := registerACMEUser
	settings.CertSettings.Email = email
	settings.CertSettings.CADir = caDir

	var calls atomic.Int32
	var registerErr error
	registerACMEUser = func(user *model.AcmeUser) error {
		calls.Add(1)
		if registerErr != nil {
			return registerErr
		}
		user.Registration = model.AcmeRegistration{URI: user.CADir + "/account/1"}
		return nil
	}

	t.Cleanup(func() {
		settings.CertSettings.Email = previousEmail
		settings.CertSettings.CADir = previousCADir
		registerACMEUser = previousRegister
		model.Use(nil)
	})
	return db, &calls, &registerErr
}

func countACMEUsers(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&model.AcmeUser{}).Count(&count).Error)
	return count
}

func TestGetACMEUserRegistersMissingDefaultUser(t *testing.T) {
	db, calls, _ := setupDefaultACMEUserTest(t, "admin@example.com", "https://ca.example.com/directory")

	user, err := (&ConfigPayload{}).GetACMEUser()

	require.NoError(t, err)
	assert.Equal(t, "admin@example.com", user.Email)
	assert.Equal(t, "https://ca.example.com/directory", user.CADir)
	assert.Equal(t, "https://ca.example.com/directory/account/1", user.Registration.URI)
	assert.NotZero(t, user.ID)
	assert.EqualValues(t, 1, calls.Load())

	// The stored user is reused, not registered again.
	again, err := (&ConfigPayload{}).GetACMEUser()
	require.NoError(t, err)
	assert.Equal(t, user.ID, again.ID)
	assert.EqualValues(t, 1, calls.Load())
	assert.EqualValues(t, 1, countACMEUsers(t, db))
}

func TestGetACMEUserFallsBackToLazyDefaultUserForUnknownID(t *testing.T) {
	_, calls, _ := setupDefaultACMEUserTest(t, "admin@example.com", "https://ca.example.com/directory")

	user, err := (&ConfigPayload{ACMEUserID: 42}).GetACMEUser()

	require.NoError(t, err)
	assert.Equal(t, "admin@example.com", user.Email)
	assert.EqualValues(t, 1, calls.Load())
}

func TestGetACMEUserRegistersAgainAfterCADirChange(t *testing.T) {
	db, calls, _ := setupDefaultACMEUserTest(t, "admin@example.com", "https://ca-one.example.com/directory")

	first, err := GetDefaultACMEUser()
	require.NoError(t, err)

	settings.CertSettings.CADir = "https://ca-two.example.com/directory"
	second, err := GetDefaultACMEUser()
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID)
	assert.Equal(t, "https://ca-two.example.com/directory", second.CADir)
	assert.EqualValues(t, 2, calls.Load())
	assert.EqualValues(t, 2, countACMEUsers(t, db))
}

func TestGetDefaultACMEUserWithoutEmailDoesNotRegister(t *testing.T) {
	_, calls, _ := setupDefaultACMEUserTest(t, "", "")

	_, err := GetDefaultACMEUser()

	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.Contains(t, err.Error(), "get default user error")
	assert.Zero(t, calls.Load())
}

func TestGetDefaultACMEUserRegistrationFailure(t *testing.T) {
	db, calls, registerErr := setupDefaultACMEUserTest(t, "admin@example.com", "https://ca.example.com/directory")
	*registerErr = stderrors.New("CA unreachable")

	_, err := GetDefaultACMEUser()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get default user error")
	assert.Contains(t, err.Error(), "register default ACME user")
	assert.Contains(t, err.Error(), "CA unreachable")
	assert.Zero(t, countACMEUsers(t, db), "a failed registration must not be stored")

	// The next call tries again and succeeds once the CA answers.
	*registerErr = nil
	user, err := GetDefaultACMEUser()
	require.NoError(t, err)
	assert.Equal(t, "admin@example.com", user.Email)
	assert.EqualValues(t, 2, calls.Load())
}

func TestInitRegisterIsIdempotentAndConcurrencySafe(t *testing.T) {
	db, calls, _ := setupDefaultACMEUserTest(t, "admin@example.com", "https://ca.example.com/directory")

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			InitRegister(context.Background())
		})
	}
	wg.Wait()
	InitRegister(context.Background())

	assert.EqualValues(t, 1, calls.Load())
	assert.EqualValues(t, 1, countACMEUsers(t, db))
}

func TestInitRegisterSkipsWithoutEmail(t *testing.T) {
	db, calls, _ := setupDefaultACMEUserTest(t, "", "")

	InitRegister(context.Background())

	assert.Zero(t, calls.Load())
	assert.Zero(t, countACMEUsers(t, db))
}
