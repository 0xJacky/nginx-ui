package sites

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

// Site B uses an upstream that site A defines. Switching A to maintenance must
// keep the upstream, and nginx must never see B without it.
func TestEnableMaintenanceKeepsUpstreamUsedByAnotherSite(t *testing.T) {
	setupManagedSiteAPITest(t, "shop.example.com", "", true)
	confDir := settings.NginxSettings.ConfigDir

	require.NoError(t, os.WriteFile(sitePath(t, "shop.example.com"), []byte(`upstream wordpress_backend {
    server 127.0.0.1:9001;
}
server {
    listen 80;
    server_name shop.example.com;
    location ~ \.php$ {
        fastcgi_pass wordpress_backend;
    }
}
`), 0o644))
	otherPath := sitePath(t, "example.com")
	require.NoError(t, os.WriteFile(otherPath, []byte(`server {
    listen 80;
    server_name example.com;
    location /api/ {
        fastcgi_pass wordpress_backend;
    }
}
`), 0o644))
	otherEnabledPath, err := site.ResolveEnabledPath("example.com")
	require.NoError(t, err)
	require.NoError(t, os.Symlink(otherPath, otherEnabledPath))

	// Like nginx -t: example.com is only valid while an enabled file defines
	// the upstream.
	check := fmt.Sprintf("grep -qs 'upstream wordpress_backend' %s/*", filepath.Join(confDir, "sites-enabled"))
	settings.NginxSettings.TestConfigCmd = check
	settings.NginxSettings.ReloadCmd = check

	response := serveManagedSiteRequest(t, http.MethodPost, "/api/sites/shop.example.com/maintenance", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	maintenancePath, err := site.ResolveEnabledMaintenancePath("shop.example.com")
	require.NoError(t, err)
	maintenance, err := os.ReadFile(maintenancePath)
	require.NoError(t, err)
	require.Contains(t, string(maintenance), "upstream wordpress_backend {")

	enabledPath, err := site.ResolveEnabledPath("shop.example.com")
	require.NoError(t, err)
	_, err = os.Lstat(enabledPath)
	require.True(t, os.IsNotExist(err), "the normal site should be replaced by its maintenance config")
}
