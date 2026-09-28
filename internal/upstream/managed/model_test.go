package managed

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
)

func intPtr(v int) *int { return &v }

func TestValidateName(t *testing.T) {
	for _, name := range []string{"backend_pool", "api-v2", "_internal", "A1"} {
		assert.NoError(t, ValidateName(name), name)
	}
	for _, name := range []string{"", "1pool", "pool.example.com", "bad name", "pool;", "../etc", "a/b",
		"x{", "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmn"} {
		assert.ErrorIs(t, ValidateName(name), ErrInvalidName, name)
	}
}

func TestRenderLeastConnWithServerParams(t *testing.T) {
	u := &Upstream{
		Name:   "backend_pool",
		Method: MethodLeastConn,
		Servers: []Server{
			{Address: "127.0.0.1:8081", Weight: intPtr(3), MaxFails: intPtr(2), FailTimeout: "10s"},
			{Address: "127.0.0.1:8082", Backup: true},
			{Address: "unix:/run/app.sock", Down: true, Params: "max_conns=100"},
		},
		Keepalive:       16,
		Zone:            true,
		ExtraDirectives: "keepalive_timeout 60s",
	}
	content, err := Prepare(u)
	require.NoError(t, err)

	assert.Equal(t, "# Managed by Nginx UI: edit this upstream from the Upstream page.\n"+
		"# Sites reference it with `proxy_pass http://backend_pool;`.\n"+
		"upstream backend_pool {\n"+
		"    least_conn;\n"+
		"    zone backend_pool 64k;\n"+
		"    server 127.0.0.1:8081 weight=3 max_fails=2 fail_timeout=10s;\n"+
		"    server 127.0.0.1:8082 backup;\n"+
		"    server unix:/run/app.sock max_conns=100 down;\n"+
		"    keepalive 16;\n"+
		"    keepalive_timeout 60s;\n"+
		"}\n", content)
}

func TestRenderRoundRobinOmitsMethodAndDefaultWeight(t *testing.T) {
	u := &Upstream{
		Name:    "rr",
		Method:  "round_robin",
		Servers: []Server{{Address: "10.0.0.1:80", Weight: intPtr(1)}},
	}
	content, err := Prepare(u)
	require.NoError(t, err)
	assert.Contains(t, content, "upstream rr {\n    server 10.0.0.1:80;\n}\n")
	assert.NotContains(t, content, "keepalive")
}

func TestRenderHashConsistent(t *testing.T) {
	u := &Upstream{
		Name:       "hashed",
		Method:     MethodHash,
		HashKey:    "$request_uri",
		Consistent: true,
		Servers:    []Server{{Address: "10.0.0.1:80"}, {Address: "10.0.0.2:80"}},
	}
	content, err := Prepare(u)
	require.NoError(t, err)
	assert.Contains(t, content, "    hash $request_uri consistent;\n")
}

func TestNormalizeDropsHashFieldsForOtherMethods(t *testing.T) {
	u := &Upstream{
		Name:       "p",
		Method:     MethodIPHash,
		HashKey:    "$host",
		Consistent: true,
		Servers:    []Server{{Address: " 10.0.0.1:80 "}},
	}
	content, err := Prepare(u)
	require.NoError(t, err)
	assert.Contains(t, content, "    ip_hash;\n    server 10.0.0.1:80;\n")
	assert.NotContains(t, content, "$host")
}

func TestParseRoundTrip(t *testing.T) {
	original := &Upstream{
		Name:       "backend_pool",
		Method:     MethodHash,
		HashKey:    "$remote_addr",
		Consistent: true,
		Servers: []Server{
			{Address: "127.0.0.1:8081", Weight: intPtr(5), MaxFails: intPtr(0), FailTimeout: "30s"},
			{Address: "[::1]:8082", Down: true, Params: "max_conns=10"},
		},
		Keepalive:       8,
		Zone:            true,
		ZoneSize:        "1m",
		ExtraDirectives: "keepalive_requests 100;",
	}
	content, err := Prepare(original)
	require.NoError(t, err)

	parsed, err := Parse("backend_pool", content)
	require.NoError(t, err)
	assert.Equal(t, original, parsed)

	// Rendering the parsed form again must be stable.
	again, err := Prepare(parsed)
	require.NoError(t, err)
	assert.Equal(t, content, again)
}

func TestParseKeepsUnknownDirectives(t *testing.T) {
	parsed, err := Parse("pool", `upstream pool {
    random two least_conn;
    server 10.0.0.1:80 slow_start=30s;
    keepalive 4;
    keepalive_requests 1000;
}`)
	require.NoError(t, err)
	assert.Equal(t, MethodRoundRobin, parsed.Method)
	assert.Equal(t, 4, parsed.Keepalive)
	assert.Equal(t, "random two least_conn;\nkeepalive_requests 1000;", parsed.ExtraDirectives)
	require.Len(t, parsed.Servers, 1)
	assert.Equal(t, "slow_start=30s", parsed.Servers[0].Params)
}

func TestParseRejectsFilesItDoesNotManage(t *testing.T) {
	cases := map[string]string{
		"wrong name":      "upstream other { server 10.0.0.1:80; }",
		"two upstreams":   "upstream pool { server 10.0.0.1:80; }\nupstream pool2 { server 10.0.0.2:80; }",
		"server block":    "upstream pool { server 10.0.0.1:80; }\nserver { listen 80; }",
		"other directive": "map $a $b { default 1; }\nupstream pool { server 10.0.0.1:80; }",
	}
	for name, content := range cases {
		_, err := Parse("pool", content)
		assertCosyError(t, err, ErrUpstreamFileNotManaged, name)
	}
}

func TestValidateRejectsUnsafeValues(t *testing.T) {
	base := func() *Upstream {
		return &Upstream{Name: "pool", Servers: []Server{{Address: "10.0.0.1:80"}}}
	}

	cases := []struct {
		name   string
		mutate func(u *Upstream)
		want   error
	}{
		{"no servers", func(u *Upstream) { u.Servers = nil }, ErrNoServers},
		{"unknown method", func(u *Upstream) { u.Method = "least_time" }, ErrInvalidMethod},
		{"hash without key", func(u *Upstream) { u.Method = MethodHash }, ErrHashKeyRequired},
		{"hash key injection", func(u *Upstream) { u.Method = MethodHash; u.HashKey = "$host; include /etc/passwd" }, ErrInvalidHashKey},
		{"address injection", func(u *Upstream) { u.Servers[0].Address = "10.0.0.1:80; }" }, ErrInvalidServerAddress},
		{"address with scheme", func(u *Upstream) { u.Servers[0].Address = "http://10.0.0.1" }, ErrInvalidServerAddress},
		{"empty address", func(u *Upstream) { u.Servers[0].Address = "  " }, ErrInvalidServerAddress},
		{"zero weight", func(u *Upstream) { u.Servers[0].Weight = intPtr(0) }, ErrInvalidWeight},
		{"negative max_fails", func(u *Upstream) { u.Servers[0].MaxFails = intPtr(-1) }, ErrInvalidMaxFails},
		{"bad fail_timeout", func(u *Upstream) { u.Servers[0].FailTimeout = "10 s" }, ErrInvalidFailTimeout},
		{"backup with ip_hash", func(u *Upstream) { u.Method = MethodIPHash; u.Servers[0].Backup = true }, ErrBackupNotSupported},
		{"backup with hash", func(u *Upstream) { u.Method = MethodHash; u.HashKey = "$host"; u.Servers[0].Backup = true }, ErrBackupNotSupported},
		{"negative keepalive", func(u *Upstream) { u.Keepalive = -1 }, ErrInvalidKeepalive},
		{"block in extra", func(u *Upstream) { u.ExtraDirectives = "}\nserver { listen 80; }" }, ErrInvalidExtraDirectives},
		{"bad server params", func(u *Upstream) { u.Servers[0].Params = "max_conns=1;" }, ErrInvalidServerParams},
	}
	for _, tc := range cases {
		u := base()
		tc.mutate(u)
		_, err := Prepare(u)
		require.Error(t, err, tc.name)
		assertCosyError(t, err, tc.want, tc.name)
	}

	// Backup servers are fine with the default and least_conn methods.
	for _, method := range []string{MethodRoundRobin, MethodLeastConn} {
		u := base()
		u.Method = method
		u.Servers = append(u.Servers, Server{Address: "10.0.0.2:80", Backup: true})
		_, err := Prepare(u)
		assert.NoError(t, err, method)
	}
}

// assertCosyError checks that err carries the scope and code of want. Errors
// wrapped with parameters are copies, so errors.Is cannot match them.
func assertCosyError(t *testing.T, err error, want error, msgAndArgs ...any) {
	t.Helper()
	var got, expected *cosy.Error
	require.True(t, errors.As(err, &got), msgAndArgs...)
	require.True(t, errors.As(want, &expected))
	assert.Equal(t, expected.Scope, got.Scope, msgAndArgs...)
	assert.Equal(t, expected.Code, got.Code, msgAndArgs...)
}

func TestZoneIsRenderedAfterTheMethod(t *testing.T) {
	u := &Upstream{Name: "pool", Method: MethodLeastConn, Zone: true, Servers: []Server{{Address: "10.0.0.1:80"}}}
	content, err := Prepare(u)
	require.NoError(t, err)
	// An empty size falls back to the default.
	assert.Equal(t, DefaultZoneSize, u.ZoneSize)
	assert.Contains(t, content, "upstream pool {\n    least_conn;\n    zone pool 64k;\n    server 10.0.0.1:80;\n}\n")
}

func TestZoneOffRendersNoZoneAndClearsSize(t *testing.T) {
	u := &Upstream{Name: "pool", ZoneSize: "128k", Servers: []Server{{Address: "10.0.0.1:80"}}}
	content, err := Prepare(u)
	require.NoError(t, err)
	assert.NotContains(t, content, "zone")
	assert.Empty(t, u.ZoneSize)
}

func TestParseLegacyGroupWithoutZoneKeepsSwitchOff(t *testing.T) {
	parsed, err := Parse("legacy", "upstream legacy {\n    server 10.0.0.1:80;\n}\n")
	require.NoError(t, err)
	assert.False(t, parsed.Zone)
	assert.Empty(t, parsed.ZoneSize)

	// Saving it again must not add a zone behind the operator's back.
	content, err := Prepare(parsed)
	require.NoError(t, err)
	assert.NotContains(t, content, "zone")
}

func TestParseReadsZoneIntoTheField(t *testing.T) {
	parsed, err := Parse("pool", "upstream pool {\n    zone pool 256k;\n    server 10.0.0.1:80;\n    keepalive_timeout 30s;\n}\n")
	require.NoError(t, err)
	assert.True(t, parsed.Zone)
	assert.Equal(t, "256k", parsed.ZoneSize)
	assert.Equal(t, "keepalive_timeout 30s;", parsed.ExtraDirectives)
}

func TestParseKeepsForeignZoneVerbatim(t *testing.T) {
	// `zone shared;` joins a zone declared by another group; it is not ours.
	parsed, err := Parse("pool", "upstream pool {\n    zone shared;\n    server 10.0.0.1:80;\n}\n")
	require.NoError(t, err)
	assert.False(t, parsed.Zone)
	assert.Equal(t, "zone shared;", parsed.ExtraDirectives)

	content, err := Prepare(parsed)
	require.NoError(t, err)
	assert.Contains(t, content, "    zone shared;\n")
}

func TestZoneInExtraDirectivesIsLiftedIntoTheField(t *testing.T) {
	u := &Upstream{
		Name:            "pool",
		Servers:         []Server{{Address: "10.0.0.1:80"}},
		ExtraDirectives: "keepalive_timeout 60s;\nzone pool 128k;",
	}
	content, err := Prepare(u)
	require.NoError(t, err)
	assert.True(t, u.Zone)
	assert.Equal(t, "128k", u.ZoneSize)
	assert.Equal(t, "keepalive_timeout 60s;", u.ExtraDirectives)
	assert.Equal(t, 1, strings.Count(content, "zone pool"))

	// With the switch on, the form size wins and the line is not duplicated.
	u = &Upstream{
		Name:            "pool",
		Zone:            true,
		ZoneSize:        "64k",
		Servers:         []Server{{Address: "10.0.0.1:80"}},
		ExtraDirectives: "zone pool 128k;",
	}
	content, err = Prepare(u)
	require.NoError(t, err)
	assert.Equal(t, "64k", u.ZoneSize)
	assert.Empty(t, u.ExtraDirectives)
	assert.Equal(t, 1, strings.Count(content, "zone pool"))
}

func TestZoneValidation(t *testing.T) {
	for _, size := range []string{"32k", "64k", "64K", "1m", "2M", "65536"} {
		u := &Upstream{Name: "pool", Zone: true, ZoneSize: size, Servers: []Server{{Address: "10.0.0.1:80"}}}
		_, err := Prepare(u)
		assert.NoError(t, err, size)
	}
	for _, size := range []string{"8k", "16k", "1000", "64 k", "64kb", "1g", "-64k", "64k;", "abc"} {
		u := &Upstream{Name: "pool", Zone: true, ZoneSize: size, Servers: []Server{{Address: "10.0.0.1:80"}}}
		_, err := Prepare(u)
		assertCosyError(t, err, ErrInvalidZoneSize, size)
	}

	// A second, differently named zone next to the switch would make nginx
	// reject the block.
	u := &Upstream{Name: "pool", Zone: true, Servers: []Server{{Address: "10.0.0.1:80"}}, ExtraDirectives: "zone other 64k;"}
	_, err := Prepare(u)
	assertCosyError(t, err, ErrZoneInExtraDirectives)

	// Without the switch a foreign zone is fine.
	u = &Upstream{Name: "pool", Servers: []Server{{Address: "10.0.0.1:80"}}, ExtraDirectives: "zone other 64k;"}
	_, err = Prepare(u)
	assert.NoError(t, err)
}
