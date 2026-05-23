package cert

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates a per-test private in-memory SQLite DB with the Cert
// schema migrated, wires it into the model package, and returns the *gorm.DB
// for fixtures. Using ":memory:" (no shared cache) gives each gorm.Open call
// a fresh isolated database, preventing cross-test pollution.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Cert{}))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })
	return db
}

func TestSweepStalePendingConvertsPendingToFailure(t *testing.T) {
	db := setupTestDB(t)

	pending := model.Cert{
		Name:     "example.com",
		Filename: "example.com",
		Status:   model.CertStatusPending,
	}
	require.NoError(t, db.Create(&pending).Error)

	require.NoError(t, SweepStalePending())

	var got model.Cert
	require.NoError(t, db.First(&got, pending.ID).Error)
	assert.Equal(t, model.CertStatusFailure, got.Status)
	assert.Equal(t, "Server restarted during issuance", got.LastError)
}

func TestSweepStalePendingLeavesSuccessAndFailureAlone(t *testing.T) {
	db := setupTestDB(t)

	success := model.Cert{Name: "ok.example.com", Filename: "ok.example.com", Status: model.CertStatusSuccess}
	failure := model.Cert{Name: "bad.example.com", Filename: "bad.example.com", Status: model.CertStatusFailure, LastError: "DNS timeout"}
	empty := model.Cert{Name: "legacy.example.com", Filename: "legacy.example.com"}
	require.NoError(t, db.Create(&success).Error)
	require.NoError(t, db.Create(&failure).Error)
	require.NoError(t, db.Create(&empty).Error)

	require.NoError(t, SweepStalePending())

	var gotSuccess, gotFailure, gotEmpty model.Cert
	require.NoError(t, db.First(&gotSuccess, success.ID).Error)
	require.NoError(t, db.First(&gotFailure, failure.ID).Error)
	require.NoError(t, db.First(&gotEmpty, empty.ID).Error)
	assert.Equal(t, model.CertStatusSuccess, gotSuccess.Status)
	assert.Equal(t, model.CertStatusFailure, gotFailure.Status)
	assert.Equal(t, "DNS timeout", gotFailure.LastError)
	assert.Equal(t, "", gotEmpty.Status)
}

func TestSweepStalePendingNoDBIsNoop(t *testing.T) {
	model.Use(nil)
	assert.NoError(t, SweepStalePending())
}
