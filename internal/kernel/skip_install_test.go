package kernel

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	appSettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRegisterPredefinedUserTest(t *testing.T) {
	t.Helper()

	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "app.ini")
	require.NoError(t, os.WriteFile(confPath, []byte("[app]\n[server]\nPort = 9000\n"), 0644))

	appSettings.Init(confPath)
	cSettings.ConfPath = confPath
	appSettings.NodeSettings.SkipInstallation = true

	dbPath := filepath.Join(confDir, "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Passkey{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	t.Cleanup(func() {
		appSettings.NodeSettings.SkipInstallation = false
	})
}

func TestRegisterPredefinedUserCreatesUserWhenDatabaseIsEmpty(t *testing.T) {
	setupRegisterPredefinedUserTest(t)

	t.Setenv("NGINX_UI_PREDEFINED_USER_NAME", "admin")
	t.Setenv("NGINX_UI_PREDEFINED_USER_PASSWORD", "my-secret-password")

	registerPredefinedUser(context.Background())

	user, err := query.User.Where(query.User.ID.Eq(1)).First()
	require.NoError(t, err)
	require.Equal(t, "admin", user.Name)
	require.NotEmpty(t, user.Password)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("my-secret-password")))
}

func TestRegisterPredefinedUserUpdatesEmptyPassword(t *testing.T) {
	setupRegisterPredefinedUserTest(t)

	// Simulate InitUser creating an empty-password admin user first
	require.NoError(t, query.User.Create(&model.User{
		Model: model.Model{
			ID: 1,
		},
		Name: "admin",
	}))

	t.Setenv("NGINX_UI_PREDEFINED_USER_NAME", "admin")
	t.Setenv("NGINX_UI_PREDEFINED_USER_PASSWORD", "my-secret-password")

	registerPredefinedUser(context.Background())

	user, err := query.User.Where(query.User.ID.Eq(1)).First()
	require.NoError(t, err)
	require.Equal(t, "admin", user.Name)
	require.NotEmpty(t, user.Password)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("my-secret-password")))
}

func TestRegisterPredefinedUserDoesNotOverwriteExistingPassword(t *testing.T) {
	setupRegisterPredefinedUserTest(t)

	existingHash, err := bcrypt.GenerateFromPassword([]byte("existing-password"), bcrypt.DefaultCost)
	require.NoError(t, err)

	require.NoError(t, query.User.Create(&model.User{
		Model: model.Model{
			ID: 1,
		},
		Name:     "admin",
		Password: string(existingHash),
	}))

	t.Setenv("NGINX_UI_PREDEFINED_USER_NAME", "admin")
	t.Setenv("NGINX_UI_PREDEFINED_USER_PASSWORD", "my-secret-password")

	registerPredefinedUser(context.Background())

	user, err := query.User.Where(query.User.ID.Eq(1)).First()
	require.NoError(t, err)
	require.Equal(t, string(existingHash), user.Password)
}
