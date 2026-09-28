package serverstate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const siteConfig = `# upstream commented { server 10.0.0.9:80; }
upstream backend {
    least_conn;
    server 127.0.0.1:8081 weight=2;   # primary
    server 127.0.0.1:8082 max_fails=3 fail_timeout=10s;
    server 127.0.0.1:8083 backup down;
    keepalive 16;
}

upstream "quoted_pool" {
    server [::1]:9000;
    server unix:/run/app.sock;
}

server {
    listen 80;
    set $target "backend";
    location / {
        proxy_pass http://backend;
        add_header X-Upstream "${target}";
    }
}
`

func TestParseBlocks(t *testing.T) {
	blocks := ParseBlocks(siteConfig)
	require.Len(t, blocks, 2)

	backend := blocks[0]
	assert.Equal(t, "backend", backend.Name)
	require.Len(t, backend.Servers, 3)
	assert.Equal(t, "127.0.0.1:8081", backend.Servers[0].Address)
	assert.Equal(t, 2, backend.Servers[0].Weight())
	assert.False(t, backend.Servers[0].Down())
	assert.Equal(t, "max_fails=3 fail_timeout=10s", backend.Servers[1].OtherParams())
	assert.True(t, backend.Servers[2].Down())
	assert.True(t, backend.Servers[2].Backup())

	quoted := blocks[1]
	assert.Equal(t, "quoted_pool", quoted.Name)
	require.Len(t, quoted.Servers, 2)
	assert.Equal(t, "[::1]:9000", quoted.Servers[0].Address)
	assert.Equal(t, "unix:/run/app.sock", quoted.Servers[1].Address)
}

func TestParseBlocksIgnoresServerOutsideUpstream(t *testing.T) {
	blocks := ParseBlocks("server { listen 80; server_name a; }\nstream { upstream tcp { server 10.0.0.1:53; } }\n")
	require.Len(t, blocks, 1)
	assert.Equal(t, "tcp", blocks[0].Name)
	require.Len(t, blocks[0].Servers, 1)
}

func TestSetDownDisablesOnlyTheTargetLine(t *testing.T) {
	out, matched, changed := SetDown(siteConfig, "backend", "127.0.0.1:8081", true)
	assert.Equal(t, 1, matched)
	assert.True(t, changed)
	// The rest of the file, including the comment, is untouched.
	assert.Contains(t, out, "    server 127.0.0.1:8081 weight=2 down;   # primary\n")
	assert.Equal(t, len(siteConfig)+len(" down"), len(out))

	// Disabling again is a no-op.
	again, matched, changed := SetDown(out, "backend", "127.0.0.1:8081", true)
	assert.Equal(t, 1, matched)
	assert.False(t, changed)
	assert.Equal(t, out, again)
}

func TestSetDownEnableRestoresOriginal(t *testing.T) {
	disabled, _, _ := SetDown(siteConfig, "backend", "127.0.0.1:8082", true)
	enabled, matched, changed := SetDown(disabled, "backend", "127.0.0.1:8082", false)
	assert.Equal(t, 1, matched)
	assert.True(t, changed)
	assert.Equal(t, siteConfig, enabled)
}

func TestSetDownRemovesEveryDownToken(t *testing.T) {
	content := "upstream a {\n    server 10.0.0.1:80 down weight=2 down;\n}\n"
	out, _, changed := SetDown(content, "a", "10.0.0.1:80", false)
	assert.True(t, changed)
	assert.Equal(t, "upstream a {\n    server 10.0.0.1:80 weight=2;\n}\n", out)
}

func TestSetDownKeepsCommentLineBreak(t *testing.T) {
	content := "upstream a {\n    server 10.0.0.1:80 # note\n        down;\n}\n"
	out, _, changed := SetDown(content, "a", "10.0.0.1:80", false)
	assert.True(t, changed)
	assert.Equal(t, "upstream a {\n    server 10.0.0.1:80 # note\n;\n}\n", out)
	blocks := ParseBlocks(out)
	require.Len(t, blocks, 1)
	require.Len(t, blocks[0].Servers, 1)
	assert.False(t, blocks[0].Servers[0].Down())
}

func TestSetDownDoesNotConfuseAddressNamedDown(t *testing.T) {
	content := "upstream a {\n    server down:8080;\n    server down;\n}\n"
	out, matched, changed := SetDown(content, "a", "down", true)
	assert.Equal(t, 1, matched)
	assert.True(t, changed)
	assert.Equal(t, "upstream a {\n    server down:8080;\n    server down down;\n}\n", out)

	blocks := ParseBlocks(out)
	assert.False(t, blocks[0].Servers[0].Down())
	assert.True(t, blocks[0].Servers[1].Down())

	restored, _, _ := SetDown(out, "a", "down", false)
	assert.Equal(t, content, restored)
}

func TestSetDownOnlyTouchesNamedUpstream(t *testing.T) {
	content := "upstream a { server 10.0.0.1:80; }\nupstream b { server 10.0.0.1:80; }\n"
	out, matched, changed := SetDown(content, "b", "10.0.0.1:80", true)
	assert.Equal(t, 1, matched)
	assert.True(t, changed)
	assert.Equal(t, "upstream a { server 10.0.0.1:80; }\nupstream b { server 10.0.0.1:80 down; }\n", out)

	_, matched, changed = SetDown(content, "missing", "10.0.0.1:80", true)
	assert.Equal(t, 0, matched)
	assert.False(t, changed)
}

func TestSetDownIgnoresCommentedServers(t *testing.T) {
	content := "upstream a {\n    # server 10.0.0.2:80;\n    server 10.0.0.1:80;\n}\n"
	_, matched, changed := SetDown(content, "a", "10.0.0.2:80", true)
	assert.Equal(t, 0, matched)
	assert.False(t, changed)
}

func TestParseBlocksReportsDirectivesCommentsAndOffsets(t *testing.T) {
	content := "http {\n    upstream pool {\n        hash \"$request_uri\" consistent; # sticky\n" +
		"        server 127.0.0.1:8081;\n        check { interval 3; }\n    }\n}\n"
	blocks := ParseBlocks(content)
	require.Len(t, blocks, 1)

	block := blocks[0]
	assert.Equal(t, "http", block.Parent)
	assert.Equal(t, []string{"check"}, block.NestedBlocks)
	assert.Equal(t, []string{"# sticky"}, block.Comments)
	require.Len(t, block.Directives, 2)
	assert.Equal(t, Directive{
		Name:    "hash",
		Args:    []string{"$request_uri", "consistent"},
		RawArgs: []string{`"$request_uri"`, "consistent"},
	}, block.Directives[0])
	assert.Equal(t, "server", block.Directives[1].Name)
	// Directives of the nested block do not belong to the upstream.
	require.Len(t, block.Servers, 1)

	assert.True(t, strings.HasPrefix(content[block.Start:], "upstream pool {"))
	assert.True(t, strings.HasSuffix(content[:block.End], "interval 3; }\n    }"))
}
