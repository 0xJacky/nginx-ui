package system

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	internalSystem "github.com/0xJacky/Nginx-UI/internal/system"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	appSettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupInstallHandlerTest(t *testing.T) string {
	t.Helper()

	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "app.ini")
	require.NoError(t, os.WriteFile(confPath, []byte("[app]\n[server]\nPort = 9000\n"), 0644))

	appSettings.Init(confPath)
	cSettings.ConfPath = confPath
	cSettings.AppSettings.JwtSecret = ""
	appSettings.NodeSettings.SkipInstallation = false
	appSettings.NodeSettings.Secret = ""

	dbPath := filepath.Join(confDir, "install.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)
	require.NoError(t, db.Create(&model.User{Model: model.Model{ID: 1}, Name: "admin"}).Error)

	require.NoError(t, internalSystem.EnsureInstallSecret())

	t.Cleanup(func() {
		_ = internalSystem.CleanupInstallSecret()
		cSettings.AppSettings.JwtSecret = ""
		appSettings.NodeSettings.SkipInstallation = false
		appSettings.NodeSettings.Secret = ""
	})

	return confDir
}

func TestInstallNginxUIConsumesInstallSecret(t *testing.T) {
	confDir := setupInstallHandlerTest(t)

	body, err := json.Marshal(InstallJson{
		Email:    "admin@example.com",
		Username: "new-admin",
		Password: "Passw0rd123",
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/install", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	InstallNginxUI(c)

	require.Equal(t, http.StatusOK, w.Code)

	_, err = os.Stat(filepath.Join(confDir, internalSystem.InstallSecretFileName))
	require.True(t, os.IsNotExist(err))

	user, err := query.User.Where(query.User.ID.Eq(1)).First()
	require.NoError(t, err)
	require.Equal(t, "new-admin", user.Name)
	require.NotEmpty(t, cSettings.AppSettings.JwtSecret)
}
