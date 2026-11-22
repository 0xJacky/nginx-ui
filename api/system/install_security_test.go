package system

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	internalSystem "github.com/0xJacky/Nginx-UI/internal/system"
	"github.com/0xJacky/Nginx-UI/query"
	appSettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
)

// GHSA-jjg7-4xmx-2vvh: when app.ini stores a value it cannot write back, the
// install cannot be saved. It must be refused and rolled back, so nginx-ui is
// not left locked without an administrator and can be installed once app.ini
// is corrected.
func TestInstallNginxUIRollsBackWhenAppIniHoldsUnstorableValue(t *testing.T) {
	confDir := setupInstallHandlerTest(t)
	confPath := filepath.Join(confDir, "app.ini")
	originalBackup := *appSettings.BackupSettings
	t.Cleanup(func() { *appSettings.BackupSettings = originalBackup })

	require.NoError(t, os.WriteFile(confPath,
		[]byte("[app]\n[server]\nPort = 9000\n[backup]\nGrantedAccessPath = \"D:\\\"\n"), 0o644))
	appSettings.Init(confPath)
	cSettings.ConfPath = confPath
	cSettings.AppSettings.JwtSecret = ""
	appSettings.NodeSettings.Secret = ""

	w := postInstall(t, "admin@example.com")

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	var response struct {
		Scope  string   `json:"scope"`
		Code   int32    `json:"code"`
		Params []string `json:"params"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "settings", response.Scope)
	assert.Equal(t, int32(40007), response.Code)
	assert.Equal(t, []string{"backup.GrantedAccessPath"}, response.Params)
	assert.Empty(t, cSettings.AppSettings.JwtSecret)
	assert.Empty(t, appSettings.NodeSettings.Secret)
	assert.False(t, internalSystem.InstallLockStatus(), "a refused install must leave nginx-ui installable")
	user, err := query.User.Where(query.User.ID.Eq(1)).First()
	require.NoError(t, err)
	assert.Equal(t, "admin", user.Name)

	// Once app.ini is corrected, the install goes through.
	require.NoError(t, os.WriteFile(confPath,
		[]byte("[app]\n[server]\nPort = 9000\n[backup]\nGrantedAccessPath = D:\\backups\n"), 0o644))
	appSettings.Init(confPath)
	cSettings.ConfPath = confPath
	cSettings.AppSettings.JwtSecret = ""

	w = postInstall(t, "admin@example.com")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotEmpty(t, cSettings.AppSettings.JwtSecret)
}
