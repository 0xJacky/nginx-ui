package convert

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(v bool) *bool {
	return &v
}

// zoneLines returns the zone directives of a group file.
func zoneLines(content string) []string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "zone ") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

func TestConvertAddsDefaultZone(t *testing.T) {
	confDir := setupConvertTest(t)
	enabledSite(t, confDir, "legacy.test", legacySite)

	detail, err := Convert(context.Background(), Request{Site: "legacy.test", Upstream: "legacy_pool"}, "admin")
	require.NoError(t, err)

	content := readFile(t, groupPath(confDir, "legacy_pool"))
	assert.Equal(t, []string{"zone legacy_pool 64k;"}, zoneLines(content))
	assert.True(t, detail.Zone)
	assert.Equal(t, "64k", detail.ZoneSize)
}

func TestConvertWithZoneOffAddsNone(t *testing.T) {
	confDir := setupConvertTest(t)
	enabledSite(t, confDir, "legacy.test", legacySite)

	detail, err := Convert(context.Background(),
		Request{Site: "legacy.test", Upstream: "legacy_pool", Zone: boolPtr(false), ZoneSize: "1m"}, "admin")
	require.NoError(t, err)

	assert.Equal(t, legacyGroupNoZone, readFile(t, groupPath(confDir, "legacy_pool")))
	assert.False(t, detail.Zone)
	assert.Empty(t, detail.ZoneSize)
}

func TestConvertWithCustomZoneSize(t *testing.T) {
	confDir := setupConvertTest(t)
	enabledSite(t, confDir, "legacy.test", legacySite)

	detail, err := Convert(context.Background(),
		Request{Site: "legacy.test", Upstream: "legacy_pool", Zone: boolPtr(true), ZoneSize: " 1m "}, "admin")
	require.NoError(t, err)

	assert.Equal(t, []string{"zone legacy_pool 1m;"}, zoneLines(readFile(t, groupPath(confDir, "legacy_pool"))))
	assert.Equal(t, "1m", detail.ZoneSize)
}

func TestConvertRejectsTooSmallZoneBeforeWriting(t *testing.T) {
	confDir := setupConvertTest(t)
	sitePath := enabledSite(t, confDir, "legacy.test", legacySite)

	_, err := Convert(context.Background(),
		Request{Site: "legacy.test", Upstream: "legacy_pool", ZoneSize: "8k"}, "admin")
	requireCosyCode(t, err, codeOf(managed.ErrInvalidZoneSize))

	assert.Equal(t, legacySite, readFile(t, sitePath))
	_, statErr := os.Stat(groupPath(confDir, "legacy_pool"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestConvertKeepsExistingZone(t *testing.T) {
	cases := []struct {
		name     string
		zoneLine string
		request  Request
		// wantZone reports whether the zone maps onto the zone switch.
		wantZone bool
		wantSize string
	}{
		{
			name: "own zone with its size", zoneLine: "zone legacy_pool 128k;",
			request:  Request{Site: "legacy.test", Upstream: "legacy_pool"},
			wantZone: true, wantSize: "128k",
		},
		{
			name: "own zone kept with the switch off and another size", zoneLine: "zone legacy_pool 128k;",
			request:  Request{Site: "legacy.test", Upstream: "legacy_pool", Zone: boolPtr(false), ZoneSize: "1m"},
			wantZone: true, wantSize: "128k",
		},
		{
			name: "zone of another name", zoneLine: "zone shared_pool 256k;",
			request: Request{Site: "legacy.test", Upstream: "legacy_pool", ZoneSize: "1m"},
		},
		{
			name: "joins a zone declared elsewhere", zoneLine: "zone shared_pool;",
			request: Request{Site: "legacy.test", Upstream: "legacy_pool"},
		},
		{
			name: "own name without a size", zoneLine: "zone legacy_pool;",
			request: Request{Site: "legacy.test", Upstream: "legacy_pool"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			confDir := setupConvertTest(t)
			content := "upstream legacy_pool {\n    " + tc.zoneLine + "\n    server 127.0.0.1:8081;\n    server 127.0.0.1:8082;\n}\n" +
				"server { listen 80; location / { proxy_pass http://legacy_pool; } }\n"
			enabledSite(t, confDir, "legacy.test", content)

			detail, err := Convert(context.Background(), tc.request, "admin")
			require.NoError(t, err)

			// The zone line is carried over as written and no other is added.
			group := readFile(t, groupPath(confDir, "legacy_pool"))
			assert.Equal(t, []string{tc.zoneLine}, zoneLines(group))
			assert.Equal(t, tc.wantZone, detail.Zone)
			assert.Equal(t, tc.wantSize, detail.ZoneSize)
			if !tc.wantZone {
				assert.Equal(t, tc.zoneLine, detail.ExtraDirectives)
			}
		})
	}
}

func TestConvertExplainsZoneNameClashAndRestoresBothFiles(t *testing.T) {
	cases := []struct {
		name    string
		content string
		zone    string
	}{
		{name: "default zone", content: legacySite, zone: "legacy_pool"},
		{
			name: "kept zone of another name",
			content: "upstream legacy_pool {\n    zone limits 64k;\n    server 127.0.0.1:8081;\n}\n" +
				"server { listen 80; location / { proxy_pass http://legacy_pool; } }\n",
			zone: "limits",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			confDir := setupConvertTest(t)
			sitePath := enabledSite(t, confDir, "legacy.test", tc.content)
			// nginx -t output when a limit_req_zone already declared the name.
			settings.NginxSettings.TestConfigCmd = "echo 'nginx: [emerg] the shared memory zone \"" + tc.zone +
				"\" is already declared for a different use in /etc/nginx/conf.d/upstream-legacy_pool.conf:4' >&2; " +
				"echo 'nginx: configuration file /etc/nginx/nginx.conf test failed' >&2; exit 1"

			_, err := Convert(context.Background(), Request{Site: "legacy.test", Upstream: "legacy_pool"}, "admin")
			requireCosyCode(t, err, codeOf(managed.ErrZoneNameConflict))
			assert.Contains(t, err.Error(), `the shared memory zone "`+tc.zone+`" is already declared for a different use`)
			assert.NotContains(t, err.Error(), "[emerg]")

			// Both files are exactly as before the conversion.
			assert.Equal(t, tc.content, readFile(t, sitePath))
			_, statErr := os.Stat(groupPath(confDir, "legacy_pool"))
			assert.True(t, os.IsNotExist(statErr), "the group file must be removed again")
		})
	}
}

func TestConvertMirrorsZoneChoiceToSyncNodes(t *testing.T) {
	cases := []struct {
		name    string
		request Request
		want    Request
	}{
		{
			name:    "zone off",
			request: Request{Site: "legacy.test", Upstream: "legacy_pool", Zone: boolPtr(false), ZoneSize: "1m"},
			want:    Request{Site: "legacy.test", Upstream: "legacy_pool", Zone: boolPtr(false)},
		},
		{
			name:    "custom size",
			request: Request{Site: "legacy.test", Upstream: "legacy_pool", ZoneSize: "1m"},
			want:    Request{Site: "legacy.test", Upstream: "legacy_pool", Zone: boolPtr(true), ZoneSize: "1m"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			confDir := setupConvertTest(t)
			sitePath := enabledSite(t, confDir, "legacy.test", legacySite)

			originalSecret := settings.CryptoSettings.Secret
			originalInstanceID := settings.NodeSettings.InstanceID
			t.Cleanup(func() {
				settings.CryptoSettings.Secret = originalSecret
				settings.NodeSettings.InstanceID = originalInstanceID
			})
			settings.CryptoSettings.Secret = "upstream-convert-zone-test-root"
			settings.NodeSettings.InstanceID = "33333333-3333-4333-8333-333333333333"

			var (
				mu       sync.Mutex
				received []Request
				bodies   []map[string]any
			)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				bodies = append(bodies, body)
				raw, _ := json.Marshal(body)
				var req Request
				_ = json.Unmarshal(raw, &req)
				received = append(received, req)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"name":"legacy_pool"}`))
			}))
			t.Cleanup(server.Close)

			node := &model.Node{
				Name:             "node-1",
				URL:              server.URL,
				AuthMethod:       model.NodeAuthMethodLegacy,
				CredentialStatus: model.NodeCredentialStatusActive,
				Enabled:          true,
			}
			db := model.UseDB()
			require.NoError(t, db.Create(node).Error)
			secret, err := nodeauth.EncryptPrivateCredential(nodeauth.LegacyCredentialPurpose(node.ID), []byte("node-secret"))
			require.NoError(t, err)
			require.NoError(t, db.Model(node).Update("encrypted_legacy_secret", secret).Error)
			require.NoError(t, query.Site.Create(&model.Site{Path: sitePath, SyncNodeIDs: []uint64{node.ID}}))

			_, err = Convert(context.Background(), tc.request, "admin")
			require.NoError(t, err)
			WaitForSync()

			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, []Request{tc.want}, received)
			// The flag is on the wire even when it is false.
			require.Len(t, bodies, 1)
			assert.Equal(t, *tc.want.Zone, bodies[0]["zone"])
		})
	}
}
