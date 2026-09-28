package nginx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readServerBlocksFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "server_blocks", name))
	require.NoError(t, err)
	return string(data)
}

func TestServerBlockParseListen(t *testing.T) {
	tests := []struct {
		args   []string
		want   ServerListen
		wantOK bool
	}{
		{[]string{"80"}, ServerListen{Port: "80"}, true},
		{[]string{"8080"}, ServerListen{Port: "8080"}, true},
		{[]string{"127.0.0.1"}, ServerListen{Addr: "127.0.0.1", Port: "80"}, true},
		{[]string{"127.0.0.1:80"}, ServerListen{Addr: "127.0.0.1", Port: "80"}, true},
		{[]string{"*:80"}, ServerListen{Port: "80"}, true},
		{[]string{"0.0.0.0:80"}, ServerListen{Port: "80"}, true},
		{[]string{"[::]:80"}, ServerListen{Port: "80", IPv6: true}, true},
		{[]string{"[::]"}, ServerListen{Port: "80", IPv6: true}, true},
		{[]string{"[::1]:443", "ssl"}, ServerListen{Addr: "::1", Port: "443", IPv6: true, SSL: true}, true},
		{[]string{"[::]:80", "ipv6only=on", "default_server"}, ServerListen{Port: "80", IPv6: true, DefaultServer: true}, true},
		{[]string{"80", "default"}, ServerListen{Port: "80", DefaultServer: true}, true},
		{[]string{"443", "ssl", "http2", "default_server"}, ServerListen{Port: "443", SSL: true, DefaultServer: true}, true},
		{[]string{"Example.COM:8080"}, ServerListen{Addr: "example.com", Port: "8080"}, true},
		{[]string{"443", "quic", "reuseport"}, ServerListen{}, false},
		{[]string{"[::]:443", "quic"}, ServerListen{}, false},
		{[]string{"unix:/run/nginx.sock"}, ServerListen{}, false},
		{[]string{"[::1"}, ServerListen{}, false},
	}

	for _, tt := range tests {
		t.Run(filepath.Join(tt.args...), func(t *testing.T) {
			got, ok := parseServerListen(tt.args)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestServerBlockTokenizer(t *testing.T) {
	content := `# leading comment { not a block
server { # trailing comment }
    listen 80;
    server_name "quoted } name" 'single \' { quote' a#b;
    set $x "${host}suffix";
    return 200 ${scheme}://$host;
    content_by_lua_block {
        -- a comment with } and '
        local s = "}{"
        local l = [[ } ]]
        if true then ngx.say("{") end
    }
    location / { return 204; }
}
}
server { listen 81; }`

	directives := parseSbDirectives(content, "/etc/nginx/test.conf")
	require.Len(t, directives, 2)

	server := directives[0]
	assert.Equal(t, "server", server.name)
	assert.True(t, server.hasBlock)
	assert.Equal(t, "/etc/nginx/test.conf", server.file)

	var names []string
	var names2 []string
	for _, d := range server.block {
		names = append(names, d.name)
		if d.name == "server_name" {
			names2 = d.args
		}
		if d.name == "return" {
			assert.Equal(t, []string{"200", "${scheme}://$host"}, d.args)
		}
		if d.name == "set" {
			assert.Equal(t, []string{"$x", "${host}suffix"}, d.args)
		}
	}
	assert.Equal(t, []string{"listen", "server_name", "set", "return", "content_by_lua_block", "location"}, names)
	assert.Equal(t, []string{"quoted } name", "single ' { quote", "a#b"}, names2)

	assert.Equal(t, "server", directives[1].name)
	require.Len(t, directives[1].block, 1)
	assert.Equal(t, []string{"81"}, directives[1].block[0].args)
}

func TestServerBlockSplitNginxTSections(t *testing.T) {
	sections := splitNginxTSections(readServerBlocksFixture(t, "windows_nginx_t.txt"))
	var paths []string
	for _, s := range sections {
		paths = append(paths, s.path)
	}
	assert.Equal(t, []string{
		`C:\nginx/conf/nginx.conf`,
		`C:\nginx/conf/mime.types`,
		`C:\nginx/conf/sites-enabled/shop.conf`,
		`C:\nginx/conf/conf.d/extra.conf`,
		`C:\Nginx/Conf/Streams-Enabled/rdp.conf`,
	}, paths)

	assert.Empty(t, splitNginxTSections("nginx: [emerg] unknown directive \"foo\"\nnginx: configuration file /etc/nginx/nginx.conf test failed\n"))
}

func TestParseServerBlocksLinux(t *testing.T) {
	blocks := ParseServerBlocks(readServerBlocksFixture(t, "linux_nginx_t.txt"))

	type summary struct {
		File    string
		Names   []string
		Listens []ServerListen
		Return  string
	}
	var got []summary
	for _, b := range blocks {
		got = append(got, summary{File: b.File, Names: b.ServerNames, Listens: b.Listens, Return: b.Return})
	}

	port80 := []ServerListen{{Port: "80"}}
	want := []summary{
		{"/etc/nginx/nginx.conf", []string{"_"}, []ServerListen{
			{Port: "80", DefaultServer: true},
			{Port: "80", IPv6: true, DefaultServer: true},
		}, "444"},
		{"/etc/nginx/conf.d/default.conf", []string{"localhost"}, port80, ""},
		{"/etc/nginx/sites-enabled/multi.conf", []string{"a.example.com", "b.example.com"}, []ServerListen{
			{Port: "80"}, {Port: "80", IPv6: true},
		}, ""},
		{"/etc/nginx/sites-enabled/multi.conf", []string{"a.example.com", "b.example.com"}, []ServerListen{
			{Port: "443", SSL: true},
		}, ""},
		{"/etc/nginx/sites-enabled/multi.conf", []string{"c.example.com"}, []ServerListen{
			{Addr: "10.0.0.5", Port: "80"},
		}, ""},
		{"/etc/nginx/sites-enabled/wildcards.conf", []string{"*.example.org"}, port80, "200 leading-short"},
		{"/etc/nginx/sites-enabled/wildcards.conf", []string{"*.api.example.org"}, port80, "200 leading-long"},
		{"/etc/nginx/sites-enabled/wildcards.conf", []string{"www.example.*"}, port80, "200 trailing"},
		{"/etc/nginx/sites-enabled/wildcards.conf", []string{`~^(?<sub>[a-z]+)\.regex\.example\.net$`, `~^second\.regex\.example\.net$`}, port80, "200 regex-first"},
		{"/etc/nginx/sites-enabled/wildcards.conf", []string{`~^.+\.regex\.example\.net$`, `~[invalid`}, port80, "200 regex-second"},
		{"/etc/nginx/sites-enabled/wildcards.conf", []string{".dot.example.com"}, port80, "200 dot"},
		// Only a unix socket listen: no implicit port 80.
		{"/etc/nginx/sites-enabled/wildcards.conf", []string{"internal.example.com"}, nil, ""},
		// Unreached section: kept because it has a server_name, while the
		// proxy_pass-only (stream-like) server next to it is dropped.
		{"/etc/nginx/orphan.conf", []string{"orphan.example.com"}, []ServerListen{{Port: "8082"}}, ""},
	}
	assert.Equal(t, want, got)

	// Locations: the included snippet is expanded in place, only top-level
	// locations are listed, and returns inside if blocks are ignored.
	assert.Equal(t, []ServerLocation{
		{Modifier: "^~", Path: "/.well-known/acme-challenge/", ProxyPass: "http://127.0.0.1:9180"},
		{Path: "/", Return: "301 https://$host$request_uri"},
	}, blocks[2].Locations)
	assert.Equal(t, []ServerLocation{{Path: "/", ProxyPass: "http://backend"}}, blocks[3].Locations)
	assert.Equal(t, []ServerLocation{
		{Path: "/{quoted}", Return: "200 it's } fine"},
		{Path: "/app/", ProxyPass: "http://app${suffix}"},
	}, blocks[4].Locations)
}

func TestParseServerBlocksWindowsPaths(t *testing.T) {
	blocks := ParseServerBlocks(readServerBlocksFixture(t, "windows_nginx_t.txt"))
	require.Len(t, blocks, 3)

	assert.Equal(t, `C:\nginx/conf/nginx.conf`, blocks[0].File)
	assert.Equal(t, []string{"localhost"}, blocks[0].ServerNames)

	// Included through an absolute backslash glob.
	assert.Equal(t, `C:\nginx/conf/sites-enabled/shop.conf`, blocks[1].File)
	assert.Equal(t, []ServerListen{{Addr: "192.168.1.20", Port: "80"}}, blocks[1].Listens)

	// Included through a relative forward-slash glob.
	assert.Equal(t, `C:\nginx/conf/conf.d/extra.conf`, blocks[2].File)

	// The stream include matches the differently-cased section path, so the
	// stream server is excluded even though it has no server_name.
	for _, b := range blocks {
		assert.NotContains(t, b.File, "rdp.conf")
	}

	sockets := ResolveServerSockets(blocks, "shop.example.com", "80")
	require.Len(t, sockets, 2)
	assert.Equal(t, ServerListen{Port: "80"}, sockets[0].Listen)
	assert.Equal(t, []string{"localhost"}, sockets[0].Block.ServerNames)
	assert.False(t, sockets[0].ByName)
	assert.Equal(t, ServerListen{Addr: "192.168.1.20", Port: "80"}, sockets[1].Listen)
	assert.Equal(t, `C:\nginx/conf/sites-enabled/shop.conf`, sockets[1].Block.File)
	assert.True(t, sockets[1].ByName)
}

func TestParseServerBlocksCRLFAndPreamble(t *testing.T) {
	blocks := ParseServerBlocks(readServerBlocksFixture(t, "crlf_nginx_t.txt"))
	require.Len(t, blocks, 1)
	assert.Equal(t, "/etc/nginx/nginx.conf", blocks[0].File)
	assert.Equal(t, []string{"crlf.example.com"}, blocks[0].ServerNames)
	assert.Equal(t, []ServerListen{{Port: "8443", SSL: true}}, blocks[0].Listens)
}

func TestParseServerBlocksIncludeEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		nginxT    string
		wantNames [][]string
		wantLsn   [][]ServerListen
	}{
		{
			name: "no listen defaults to port 80",
			nginxT: `# configuration file /etc/nginx/nginx.conf:
http { server { server_name nolisten.example.com; } }
`,
			wantNames: [][]string{{"nolisten.example.com"}},
			wantLsn:   [][]ServerListen{{{Port: "80"}}},
		},
		{
			name: "listen from an included snippet and exact include path",
			nginxT: `# configuration file /etc/nginx/nginx.conf:
http { include /etc/nginx/site.conf; }
# configuration file /etc/nginx/site.conf:
server { include snippets/listen.conf; server_name snip.example.com; }
# configuration file /etc/nginx/snippets/listen.conf:
listen 8080 default_server;
`,
			wantNames: [][]string{{"snip.example.com"}},
			wantLsn:   [][]ServerListen{{{Port: "8080", DefaultServer: true}}},
		},
		{
			name: "cyclic includes terminate",
			nginxT: `# configuration file /etc/nginx/nginx.conf:
http { include a.conf; }
# configuration file /etc/nginx/a.conf:
include b.conf;
server { server_name a.example.com; }
# configuration file /etc/nginx/b.conf:
include a.conf;
server { server_name b.example.com; }
`,
			wantNames: [][]string{{"b.example.com"}, {"a.example.com"}},
			wantLsn:   [][]ServerListen{{{Port: "80"}}, {{Port: "80"}}},
		},
		{
			name: "glob does not cross directories",
			nginxT: `# configuration file /etc/nginx/nginx.conf:
http { include sites/*; }
stream { include sites/stream/*; }
# configuration file /etc/nginx/sites/one.conf:
server { server_name one.example.com; }
# configuration file /etc/nginx/sites/stream/tcp.conf:
server { listen 5000; proxy_pass 127.0.0.1:5001; }
`,
			wantNames: [][]string{{"one.example.com"}},
			wantLsn:   [][]ServerListen{{{Port: "80"}}},
		},
		{
			name: "mail servers are excluded",
			nginxT: `# configuration file /etc/nginx/nginx.conf:
mail { server { listen 25; server_name mail.example.com; protocol smtp; } }
http { server { listen 80; server_name web.example.com; } }
`,
			wantNames: [][]string{{"web.example.com"}},
			wantLsn:   [][]ServerListen{{{Port: "80"}}},
		},
		{
			name:   "no section header",
			nginxT: "nginx: [emerg] open() \"/etc/nginx/nginx.conf\" failed\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := ParseServerBlocks(tt.nginxT)
			var names [][]string
			var listens [][]ServerListen
			for _, b := range blocks {
				names = append(names, b.ServerNames)
				listens = append(listens, b.Listens)
			}
			assert.Equal(t, tt.wantNames, names)
			assert.Equal(t, tt.wantLsn, listens)
		})
	}
}

func TestResolveServerSocketsLinux(t *testing.T) {
	blocks := ParseServerBlocks(readServerBlocksFixture(t, "linux_nginx_t.txt"))

	wildcardV4 := ServerListen{Port: "80", DefaultServer: true}
	wildcardV6 := ServerListen{Port: "80", IPv6: true, DefaultServer: true}
	specific := ServerListen{Addr: "10.0.0.5", Port: "80"}

	type pick struct {
		Listen ServerListen
		// Return, or the first server_name when the block has no return.
		Block  string
		ByName bool
	}
	describe := func(s ServerSocket) pick {
		label := s.Block.Return
		if label == "" && len(s.Block.ServerNames) > 0 {
			label = s.Block.ServerNames[0]
		}
		return pick{Listen: s.Listen, Block: label, ByName: s.ByName}
	}

	tests := []struct {
		host string
		port string
		want []pick
	}{
		{"a.example.com", "80", []pick{
			{wildcardV4, "a.example.com", true},
			{wildcardV6, "a.example.com", true},
			{specific, "c.example.com", false},
		}},
		{"A.Example.COM.", "80", []pick{
			{wildcardV4, "a.example.com", true},
			{wildcardV6, "a.example.com", true},
			{specific, "c.example.com", false},
		}},
		{"c.example.com", "80", []pick{
			{wildcardV4, "444", false},
			{wildcardV6, "444", false},
			{specific, "c.example.com", true},
		}},
		{"localhost", "80", []pick{
			{wildcardV4, "localhost", true},
			{wildcardV6, "444", false},
			{specific, "c.example.com", false},
		}},
		{"x.example.org", "80", []pick{{wildcardV4, "200 leading-short", true}, {wildcardV6, "444", false}, {specific, "c.example.com", false}}},
		{"x.api.example.org", "80", []pick{{wildcardV4, "200 leading-long", true}, {wildcardV6, "444", false}, {specific, "c.example.com", false}}},
		// Leading wildcards win over trailing ones.
		{"www.example.org", "80", []pick{{wildcardV4, "200 leading-short", true}, {wildcardV6, "444", false}, {specific, "c.example.com", false}}},
		{"www.example.com", "80", []pick{{wildcardV4, "200 trailing", true}, {wildcardV6, "444", false}, {specific, "c.example.com", false}}},
		// The first matching regex in config order wins; invalid ones are skipped.
		{"foo.regex.example.net", "80", []pick{{wildcardV4, "200 regex-first", true}, {wildcardV6, "444", false}, {specific, "c.example.com", false}}},
		{"second.regex.example.net", "80", []pick{{wildcardV4, "200 regex-first", true}, {wildcardV6, "444", false}, {specific, "c.example.com", false}}},
		{"a.b.regex.example.net", "80", []pick{{wildcardV4, "200 regex-second", true}, {wildcardV6, "444", false}, {specific, "c.example.com", false}}},
		// ".dot.example.com" matches the apex and every subdomain.
		{"dot.example.com", "80", []pick{{wildcardV4, "200 dot", true}, {wildcardV6, "444", false}, {specific, "c.example.com", false}}},
		{"deep.sub.dot.example.com", "80", []pick{{wildcardV4, "200 dot", true}, {wildcardV6, "444", false}, {specific, "c.example.com", false}}},
		{"unknown.test", "80", []pick{{wildcardV4, "444", false}, {wildcardV6, "444", false}, {specific, "c.example.com", false}}},
		{"b.example.com", "443", []pick{{ServerListen{Port: "443", SSL: true}, "a.example.com", true}}},
		{"b.example.com", "9999", []pick{}},
	}

	for _, tt := range tests {
		t.Run(tt.host+":"+tt.port, func(t *testing.T) {
			got := []pick{}
			for _, s := range ResolveServerSockets(blocks, tt.host, tt.port) {
				got = append(got, describe(s))
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveServerSocketsDefaultSelection(t *testing.T) {
	first := ServerBlock{File: "first", ServerNames: []string{"first.example.com"}, Listens: []ServerListen{{Port: "80"}}}
	second := ServerBlock{File: "second", ServerNames: []string{"second.example.com"}, Listens: []ServerListen{{Port: "80", DefaultServer: true}}}
	third := ServerBlock{File: "third", ServerNames: []string{"third.example.com"}, Listens: []ServerListen{{Port: "80"}}}

	tests := []struct {
		name     string
		blocks   []ServerBlock
		wantFile string
	}{
		{"first block without default_server", []ServerBlock{first, third}, "first"},
		{"default_server wins over order", []ServerBlock{first, second, third}, "second"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sockets := ResolveServerSockets(tt.blocks, "nothing.example.com", "80")
			require.Len(t, sockets, 1)
			assert.Equal(t, tt.wantFile, sockets[0].Block.File)
			assert.False(t, sockets[0].ByName)
		})
	}

	// A block listening twice on the same socket is only a candidate once.
	dup := ServerBlock{File: "dup", ServerNames: []string{"dup.example.com"}, Listens: []ServerListen{{Port: "80"}, {Addr: "", Port: "80", SSL: true}}}
	sockets := ResolveServerSockets([]ServerBlock{dup}, "dup.example.com", "80")
	require.Len(t, sockets, 1)
	assert.True(t, sockets[0].ByName)
}

// serverBlocksRunner answers nginx -T with a fixed result.
type serverBlocksRunner struct {
	out   string
	err   error
	calls int
}

func (r *serverBlocksRunner) Exec(_ context.Context, _ string, args ...string) (string, error) {
	r.calls++
	if len(args) == 1 && args[0] == "-T" {
		return r.out, r.err
	}
	return "", errors.New("unexpected command")
}

func (r *serverBlocksRunner) Stat(string) bool { return true }

func (r *serverBlocksRunner) GOOS() string { return "linux" }

func TestGetServerBlocks(t *testing.T) {
	fixture := readServerBlocksFixture(t, "linux_nginx_t.txt")

	tests := []struct {
		name       string
		runner     *serverBlocksRunner
		wantErr    error
		wantBlocks int
	}{
		{"success", &serverBlocksRunner{out: fixture}, nil, 13},
		{"nginx -T fails", &serverBlocksRunner{out: "nginx: [emerg] bad\n", err: errors.New("exit status 1")}, ErrNginxTOutputEmpty, 0},
		{"no configuration printed", &serverBlocksRunner{out: "nginx: the configuration file /etc/nginx/nginx.conf syntax is ok\n"}, ErrNginxTOutputEmpty, 0},
		{"empty output", &serverBlocksRunner{}, ErrNginxTOutputEmpty, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubNginxSettings(t, settings.Nginx{SbinPath: "/usr/sbin/nginx"})
			originalResolve := resolveRunner
			t.Cleanup(func() { resolveRunner = originalResolve })
			resolveRunner = func() Runner { return tt.runner }

			blocks, err := GetServerBlocks()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, blocks)
				return
			}
			require.NoError(t, err)
			assert.Len(t, blocks, tt.wantBlocks)
		})
	}

	// nginx -T is run on every call so configuration changes are visible.
	stubNginxSettings(t, settings.Nginx{SbinPath: "/usr/sbin/nginx"})
	originalResolve := resolveRunner
	t.Cleanup(func() { resolveRunner = originalResolve })
	runner := &serverBlocksRunner{out: fixture}
	resolveRunner = func() Runner { return runner }
	_, _ = GetServerBlocks()
	_, _ = GetServerBlocks()
	assert.Equal(t, 2, runner.calls)
}
