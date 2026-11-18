package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

// secretReferenceFixture points the configuration root at a temp dir and
// returns helpers to write files below it.
func secretReferenceFixture(t *testing.T) (root string, write func(name, content string) string) {
	t.Helper()
	root = t.TempDir()
	useSecretReferenceRoot(t, root)

	write = func(name, content string) string {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}
	return root, write
}

func useSecretReferenceRoot(t *testing.T, root string) {
	t.Helper()
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = root
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = originalConfigDir })
}

// serverConfig wraps statements in the blocks a real configuration has.
func serverConfig(statements string) string {
	return "events {}\nhttp {\n\tserver {\n\t\t" + statements + "\n\t}\n}\n"
}

// The reference scan must read configuration the way ngx_conf_read_token
// does: '#' starts a comment only where a token may start, and is an ordinary
// character inside a quoted string or in the middle of a word. Every case
// names a file nginx really loads as a secret, so each must be reported.
// (GHSA-wxp5-6x82-7qf4)
func TestSecretFileReferencesReadConfigurationLikeNginx(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config string
		// secret is the file name nginx opens, relative to the root.
		secret string
	}{
		// Controls that already work.
		{"control: plain unquoted path", "ssl_session_ticket_key ssl/ticket.bin;", "ssl/ticket.bin"},
		{"control: directive split across lines", "ssl_session_ticket_key\n\t\tssl/ticket.bin\n\t;", "ssl/ticket.bin"},
		{"control: comment between name and value", "ssl_session_ticket_key # rotated weekly\n ssl/ticket.bin;", "ssl/ticket.bin"},
		{"control: straight after open brace", "location / {auth_basic_user_file auth/users; }", "auth/users"},
		{"control: straight after close brace", "location / { }auth_basic_user_file auth/users;", "auth/users"},
		{"control: straight after semicolon", "listen 443 ssl;ssl_password_file ssl/site.pass;", "ssl/site.pass"},
		{"control: quoted path with space and semicolon", `ssl_password_file "ssl/my pass;1.txt";`, "ssl/my pass;1.txt"},
		{"control: tab and CRLF separators", "ssl_password_file\tssl/site.pass\r\n;", "ssl/site.pass"},

		// '#' inside the path itself.
		{"hash inside double-quoted path", `ssl_session_ticket_key "ssl/ticket#1.bin";`, "ssl/ticket#1.bin"},
		{"hash inside single-quoted path", `ssl_password_file 'ssl/site#1.pass';`, "ssl/site#1.pass"},
		{"hash mid-token in unquoted path", "ssl_session_ticket_key ssl/ticket#1.bin;", "ssl/ticket#1.bin"},
		{"hash mid-token in certificate key path", "ssl_certificate_key ssl/site#1.der;", "ssl/site#1.der"},
		{"hash mid-token in basic auth file", "auth_basic_user_file auth/users#staging;", "auth/users#staging"},

		// '#' earlier on the same line, in an unrelated statement.
		{"hash in earlier quoted string on the line", `add_header X-Color "#fff"; ssl_session_ticket_key ssl/ticket.bin;`, "ssl/ticket.bin"},
		{"hash in earlier redirect fragment on the line", "error_page 404 /index.html#/404; auth_basic_user_file auth/users;", "auth/users"},
		{"hash mid-token in earlier word on the line", "set $v a#b; ssl_password_file ssl/site.pass;", "ssl/site.pass"},
		{"hash in earlier single-quoted string on the line", "add_header X 'a # b'; ssl_certificate_key ssl/site.der;", "ssl/site.der"},
		{"hash right after a variable on the line", "set $v $host#; ssl_password_file ssl/site.pass;", "ssl/site.pass"},
		{"hash in an earlier regex location on the line", "location ~ ^/a#b$ { return 204; } ssl_password_file ssl/site.pass;", "ssl/site.pass"},
		{"hash inside a Lua block earlier on the line", `location /l { content_by_lua_block { ngx.say("#") } } ssl_password_file ssl/site.pass;`, "ssl/site.pass"},

		// Escapes nginx resolves before it opens the file.
		{"escaped quote inside quoted path", `ssl_password_file "ssl/pa\"ss.txt";`, `ssl/pa"ss.txt`},
		{"escaped backslash in unquoted path", `ssl_password_file ssl/pa\\ss.txt;`, `ssl/pa\ss.txt`},
		{"escaped space keeps its backslash", `ssl_password_file ssl/my\ pass.txt;`, `ssl/my\ pass.txt`},
		{"quote characters at the end of an unquoted path", `ssl_password_file ssl/site.pass';`, `ssl/site.pass'`},
		{"closing brace inside an unquoted path", "ssl_session_ticket_key ssl/ticket}1.bin;", "ssl/ticket}1.bin"},

		// Tokens nginx accepts that a line pattern does not.
		{"quoted directive name", `"ssl_session_ticket_key" ssl/ticket.bin;`, "ssl/ticket.bin"},

		// Literal characters that are not a variable or a key source.
		{"colon in file name", "ssl_session_ticket_key ssl/ticket:1.bin;", "ssl/ticket:1.bin"},
		{"colon in certificate key file name", "ssl_certificate_key ssl/site:1.der;", "ssl/site:1.der"},
		{"dollar in a directive without variable support", "ssl_session_ticket_key ssl/ticket$1.bin;", "ssl/ticket$1.bin"},
		{"dollar brace in a directive without variable support", "ssl_password_file ssl/site${a}.pass;", "ssl/site${a}.pass"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, write := secretReferenceFixture(t)
			secret := write(tt.secret, "0123456789abcdef0123456789abcdef0123456789abcdef")
			write("nginx.conf", serverConfig(tt.config))

			require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()),
				"nginx reads %q as a secret file but the scan did not report it", tt.secret)
		})
	}
}

// Shapes a fix must keep readable: real comments, certificates, CA bundles,
// values that do not name a file and text that is not a statement at all.
func TestSecretFileReferencesKeepNonSecretsReadable(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config string
		public string
	}{
		{"full-line comment", "# ssl_password_file ssl/old.pass;", "ssl/old.pass"},
		{"comment after whitespace", "listen 443 ssl; # ssl_password_file ssl/old.pass;", "ssl/old.pass"},
		{"comment straight after semicolon", "listen 443 ssl;# ssl_password_file ssl/old.pass;", "ssl/old.pass"},
		{"comment straight after open brace", "location / {# ssl_password_file ssl/old.pass;\n}", "ssl/old.pass"},
		{"comment straight after close brace", "location / { }# ssl_password_file ssl/old.pass;", "ssl/old.pass"},
		{"comment after a quoted string holding a hash", `add_header X-Color "#fff"; # ssl_password_file ssl/old.pass;`, "ssl/old.pass"},
		{"comment after a regex location holding a hash", "location ~ ^/a#b$ { return 204; } # ssl_password_file ssl/old.pass;", "ssl/old.pass"},
		{"comment with CRLF line ending", "# ssl_password_file ssl/old.pass;\r", "ssl/old.pass"},
		{"certificate", "ssl_certificate ssl/site.crt;", "ssl/site.crt"},
		{"certificate next to its key", "ssl_certificate ssl/site.crt; ssl_certificate_key ssl/site.der;", "ssl/site.crt"},
		{"client CA bundle", "ssl_client_certificate ssl/ca.bundle;", "ssl/ca.bundle"},
		{"trusted CA bundle", "ssl_trusted_certificate ssl/ca.bundle;", "ssl/ca.bundle"},
		{"upstream CA bundle", "proxy_ssl_trusted_certificate ssl/ca.bundle;", "ssl/ca.bundle"},
		{"DH parameters", "ssl_dhparam ssl/dhparam.txt;", "ssl/dhparam.txt"},
		{"include", "include conf.d/extra.conf;", "conf.d/extra.conf"},
		{"document root", "root html/site;", "html/site"},
		{"variable key path", "ssl_certificate_key $ssl_server_name.der;", "$ssl_server_name.der"},
		{"variable upstream key path", "proxy_ssl_certificate_key ssl/$upstream.der;", "ssl/$upstream.der"},
		{"variable basic auth file", "auth_basic_user_file auth/$host;", "auth/$host"},
		{"data scheme", "ssl_certificate_key data:$key;", "data:$key"},
		{"engine key source", "ssl_certificate_key engine:pkcs11:slot_0-id_1;", "engine:pkcs11:slot_0-id_1"},
		{"provider key source", "ssl_certificate_key store:pkcs11:object=site;", "store:pkcs11:object=site"},
		{"directive name used as an argument", "add_header ssl_password_file ssl/note.txt;", "ssl/note.txt"},
		{"directive name inside a quoted string", `add_header X-Note "ssl_password_file ssl/note.txt;";`, "ssl/note.txt"},
		{"directive name inside a Lua block", "location /l { content_by_lua_block {\n-- ssl_password_file ssl/note.txt;\n} }", "ssl/note.txt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, write := secretReferenceFixture(t)
			public := write(tt.public, "public")
			write("ssl/site.der", "key")
			write("nginx.conf", serverConfig(tt.config))

			require.False(t, IsSecretFile(public, []byte("not pem"), SecretFileReferences()),
				"%q must stay readable", tt.public)
		})
	}
}

// A certificate or another data file is not configuration: text inside it
// names no secret unless an include makes nginx read it as directives.
func TestSecretFileReferencesDoNotParseDataFilesAsDirectives(t *testing.T) {
	_, write := secretReferenceFixture(t)
	public := write("ssl/public.txt", "public")
	write("ssl/notes.pem", "ssl_password_file ssl/public.txt;\n")
	write("nginx.conf", serverConfig("ssl_certificate ssl/notes.pem;"))

	require.False(t, IsSecretFile(public, []byte("not pem"), SecretFileReferences()))
}

// Every directive nginx reads secret material from a file with.
func TestSecretFileReferencesKnowEverySecretFileDirective(t *testing.T) {
	for _, directive := range []string{
		"ssl_certificate_key",
		"ssl_password_file",
		"ssl_session_ticket_key",
		"ssl_ech_file",
		"ssl_key_log",
		"quic_host_key",
		"auth_basic_user_file",
		"auth_jwt_key_file",
		"proxy_ssl_certificate_key",
		"proxy_ssl_password_file",
		"proxy_ssl_key_log",
		"grpc_ssl_certificate_key",
		"grpc_ssl_password_file",
		"uwsgi_ssl_certificate_key",
		"uwsgi_ssl_password_file",
		"zone_sync_ssl_certificate_key",
		"zone_sync_ssl_password_file",
	} {
		t.Run(directive, func(t *testing.T) {
			_, write := secretReferenceFixture(t)
			secret := write("secrets/material.bin", "secret material")
			write("nginx.conf", serverConfig(directive+" secrets/material.bin;"))

			require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
		})
	}
}

// Places nginx loads configuration from, or keeps it in, that a walk over the
// regular files of the configuration root does not reach.
func TestSecretFileReferencesCoverEveryConfigurationNginxLoads(t *testing.T) {
	const directive = "ssl_session_ticket_key ssl/ticket.bin;\n"
	padding := strings.Repeat("# padding padding padding padding padding padding padding\n", maxSecretReferenceScanSize/58+1)

	t.Run("include of a file outside the configuration root", func(t *testing.T) {
		_, write := secretReferenceFixture(t)
		secret := write("ssl/ticket.bin", "ticket")
		outside := filepath.Join(t.TempDir(), "tls.conf")
		require.NoError(t, os.WriteFile(outside, []byte(directive), 0o600))
		write("nginx.conf", "events {}\nhttp { include "+outside+"; }\n")

		require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
	})

	t.Run("nested include outside the configuration root", func(t *testing.T) {
		_, write := secretReferenceFixture(t)
		secret := write("ssl/ticket.bin", "ticket")
		outside := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(outside, "tls.conf"), []byte(directive), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(outside, "http.conf"),
			[]byte("include "+filepath.Join(outside, "tls.conf")+";\n"), 0o600))
		write("nginx.conf", "events {}\nhttp { include "+filepath.Join(outside, "*.conf")+"; }\n")

		require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
	})

	t.Run("enabled site symlinked from outside the configuration root", func(t *testing.T) {
		root, write := secretReferenceFixture(t)
		secret := write("ssl/ticket.bin", "ticket")
		outside := filepath.Join(t.TempDir(), "app.conf")
		require.NoError(t, os.WriteFile(outside, []byte("server { "+directive+"}\n"), 0o600))
		require.NoError(t, os.MkdirAll(filepath.Join(root, "sites-enabled"), 0o755))
		require.NoError(t, os.Symlink(outside, filepath.Join(root, "sites-enabled", "app.conf")))
		write("nginx.conf", "events {}\nhttp { include sites-enabled/*; }\n")

		require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
	})

	t.Run("symlinked file that no include reaches", func(t *testing.T) {
		root, write := secretReferenceFixture(t)
		secret := write("ssl/ticket.bin", "ticket")
		outside := filepath.Join(t.TempDir(), "app.conf")
		require.NoError(t, os.WriteFile(outside, []byte("server { "+directive+"}\n"), 0o600))
		require.NoError(t, os.MkdirAll(filepath.Join(root, "sites-enabled"), 0o755))
		require.NoError(t, os.Symlink(outside, filepath.Join(root, "sites-enabled", "app.conf")))
		write("nginx.conf", "events {}\nhttp {}\n")

		require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
	})

	t.Run("configuration directory symlinked from outside the root", func(t *testing.T) {
		root, write := secretReferenceFixture(t)
		secret := write("ssl/ticket.bin", "ticket")
		outside := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(outside, "tls.conf"), []byte(directive), 0o600))
		require.NoError(t, os.Symlink(outside, filepath.Join(root, "conf.d")))
		write("nginx.conf", "events {}\nhttp { include conf.d/*.conf; }\n")

		require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
	})

	t.Run("configuration root that is itself a symlink", func(t *testing.T) {
		target := t.TempDir()
		root := filepath.Join(t.TempDir(), "nginx")
		require.NoError(t, os.Symlink(target, root))
		useSecretReferenceRoot(t, root)
		require.NoError(t, os.MkdirAll(filepath.Join(target, "ssl"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(target, "sites-available"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(target, "ssl", "ticket.bin"), []byte("ticket"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(target, "sites-available", "app"), []byte("server { "+directive+"}\n"), 0o600))

		require.True(t, IsSecretFile(filepath.Join(root, "ssl", "ticket.bin"), []byte("not pem"), SecretFileReferences()))
	})

	t.Run("main configuration larger than one mebibyte", func(t *testing.T) {
		_, write := secretReferenceFixture(t)
		secret := write("ssl/ticket.bin", "ticket")
		write("nginx.conf", "events {}\nhttp {\n"+padding+directive+"}\n")

		require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
	})

	t.Run("included file larger than one mebibyte", func(t *testing.T) {
		_, write := secretReferenceFixture(t)
		secret := write("ssl/ticket.bin", "ticket")
		write("conf.d/large.conf", padding+directive)
		write("nginx.conf", "events {}\nhttp { include conf.d/*.conf; }\n")

		require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
	})

	t.Run("included fragment stored under a data file extension", func(t *testing.T) {
		_, write := secretReferenceFixture(t)
		secret := write("ssl/ticket.bin", "ticket")
		write("conf.d/tls.pem", directive)
		write("nginx.conf", "events {}\nhttp { include conf.d/tls.pem; }\n")

		require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
	})

	t.Run("disabled site that no include reaches", func(t *testing.T) {
		_, write := secretReferenceFixture(t)
		secret := write("ssl/ticket.bin", "ticket")
		write("sites-available/app", "server { "+directive+"}\n")
		write("nginx.conf", "events {}\nhttp { include sites-enabled/*; }\n")

		require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
	})

	t.Run("include cycle", func(t *testing.T) {
		_, write := secretReferenceFixture(t)
		secret := write("ssl/ticket.bin", "ticket")
		write("conf.d/a.inc", "include conf.d/b.inc;\n")
		write("conf.d/b.inc", "include conf.d/a.inc;\n"+directive)
		write("nginx.conf", "events {}\nhttp { include conf.d/a.inc; }\n")

		require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
	})
}

// nginx does not clean a path before it opens it, so ".." after a symlinked
// directory leaves through the link target, not through the link's parent.
func TestSecretFileReferencesResolveDotDotLikeTheKernel(t *testing.T) {
	root, write := secretReferenceFixture(t)
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "keys", "current"), 0o755))
	passphrase := filepath.Join(outside, "keys", "site.pass")
	require.NoError(t, os.WriteFile(passphrase, []byte("passphrase"), 0o600))
	require.NoError(t, os.Symlink(filepath.Join(outside, "keys", "current"), filepath.Join(root, "live")))
	alias := filepath.Join(root, "alias.pass")
	require.NoError(t, os.Symlink(passphrase, alias))
	write("nginx.conf", serverConfig("ssl_password_file live/../site.pass;"))

	require.True(t, IsSecretFile(alias, []byte("not pem"), SecretFileReferences()))
}

// A large file is summarized once and then served from memory, which must not
// outlive a change to the file.
func TestSecretFileReferencesFollowChangesToLargeConfiguration(t *testing.T) {
	_, write := secretReferenceFixture(t)
	first := write("ssl/first.bin", "ticket")
	second := write("ssl/second.bin", "ticket")
	padding := strings.Repeat("# padding padding padding padding padding padding padding\n", maxSecretReferenceScanSize/58+1)

	write("nginx.conf", "events {}\nhttp {\n"+padding+"ssl_session_ticket_key ssl/first.bin;\n}\n")
	for range 2 {
		references := SecretFileReferences()
		require.True(t, IsSecretFile(first, []byte("not pem"), references))
		require.False(t, IsSecretFile(second, []byte("not pem"), references))
	}

	write("nginx.conf", "events {}\nhttp {\n"+padding+"ssl_session_ticket_key ssl/second.bin;\n}\n")
	references := SecretFileReferences()
	require.False(t, IsSecretFile(first, []byte("not pem"), references))
	require.True(t, IsSecretFile(second, []byte("not pem"), references))
}

// nginx reads bytes. A file name in a legacy encoding is not valid UTF-8 and
// must come out of the scan byte for byte.
func TestSummarizeConfigKeepsBytesThatAreNotUTF8(t *testing.T) {
	summary := summarizeConfig([]byte("ssl_password_file \"ssl/caf\xe9 #1.pass\"; # caf\xe9\ninclude conf.d/\xff*.conf;\n"))

	require.Equal(t, []string{"ssl/caf\xe9 #1.pass"}, summary.secrets)
	require.Equal(t, []string{"conf.d/\xff*.conf"}, summary.includes)
}

func TestSecretFileReferencesKeepFileNamesThatAreNotUTF8(t *testing.T) {
	root, write := secretReferenceFixture(t)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "ssl"), 0o755))
	secret := filepath.Join(root, "ssl", "caf\xe9.pass")
	if err := os.WriteFile(secret, []byte("passphrase"), 0o600); err != nil {
		t.Skipf("file system rejects names that are not UTF-8: %v", err)
	}
	write("nginx.conf", serverConfig("ssl_password_file ssl/caf\xe9.pass;"))

	require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
}

// On a file system that ignores letter case or Unicode normalization another
// spelling opens the same file, so the lookup must not depend on the spelling
// the caller chose.
func TestIsSecretFileIgnoresSpellingOnInsensitiveFileSystems(t *testing.T) {
	for _, tt := range []struct {
		name      string
		secret    string
		respelled string
	}{
		{"letter case of the file", "ssl/ticket.bin", "ssl/Ticket.BIN"},
		{"letter case of the directory", "ssl/ticket.bin", "SSL/ticket.bin"},
		{"letter case of both", "ssl/ticket.bin", "SSL/Ticket.BIN"},
		{"composed and decomposed characters", "ssl/café.bin", "ssl/café.bin"},
		// Full case folding maps one character to several, so the names
		// differ in length and no simple fold makes them equal.
		{"sharp s folding to two letters", "ssl/password.bin", "ssl/paßword.bin"},
		{"ligature folding to two letters", "ssl/profile.bin", "ssl/proﬁle.bin"},
		{"sharp s in a directory", "ssl/ticket.bin", "ßl/ticket.bin"},
		{"sharp s in a secret extension", "auth/.htpasswd", "auth/.htpaßwd"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, write := secretReferenceFixture(t)
			secret := write(tt.secret, "ticket")
			write("nginx.conf", serverConfig("ssl_session_ticket_key "+tt.secret+";"))

			respelled := filepath.Join(root, tt.respelled)
			original, err := os.Stat(secret)
			require.NoError(t, err)
			variant, err := os.Stat(respelled)
			if err != nil || !os.SameFile(original, variant) {
				t.Skip("file system keeps these spellings apart")
			}

			references := SecretFileReferences()
			require.True(t, IsSecretFile(secret, []byte("not pem"), references))
			require.True(t, IsSecretFile(respelled, []byte("not pem"), references),
				"%q opens the referenced secret file", respelled)
		})
	}
}

// A hard link opens the referenced file under an unrelated name on every file
// system, so it shows that the lookup compares files, not names.
// (GHSA-wxp5-6x82-7qf4)
func TestIsSecretFileRecognisesHardLinksToReferencedFiles(t *testing.T) {
	root, write := secretReferenceFixture(t)
	secret := write("ssl/ticket.bin", "ticket")
	write("nginx.conf", serverConfig("ssl_session_ticket_key ssl/ticket.bin;"))
	link := filepath.Join(root, "html", "notes.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
	if err := os.Link(secret, link); err != nil {
		t.Skipf("file system does not support hard links: %v", err)
	}
	copied := write("html/copy.txt", "ticket")

	references := SecretFileReferences()
	require.True(t, IsSecretFile(link, []byte("not pem"), references), "a hard link opens the secret file")
	require.False(t, IsSecretFile(copied, []byte("not pem"), references), "a separate file with the same content stays readable")
}

// The OpenSSL store loader opens a store: key from a file as well, given as a
// plain path or as a file: URI. (GHSA-wxp5-6x82-7qf4)
func TestSecretFileReferencesFollowStoreKeySources(t *testing.T) {
	for _, tt := range []struct{ name, prefix string }{
		{"file URI with an absolute path", "store:file:"},
		{"file URI with an empty authority", "store:file://"},
		{"file URI naming localhost", "store:file://localhost"},
		{"file URI with an upper-case scheme", "store:FILE:"},
		{"plain absolute path", "store:"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, write := secretReferenceFixture(t)
			secret := write("ssl/site.der", "binary key")
			public := write("ssl/site.crt", "public")
			write("nginx.conf", serverConfig("ssl_certificate ssl/site.crt; ssl_certificate_key "+tt.prefix+secret+";"))

			references := SecretFileReferences()
			require.True(t, IsSecretFile(secret, []byte("not pem"), references))
			require.False(t, IsSecretFile(public, []byte("not pem"), references))
		})
	}

	t.Run("relative path", func(t *testing.T) {
		_, write := secretReferenceFixture(t)
		secret := write("ssl/site.der", "binary key")
		write("nginx.conf", serverConfig("ssl_certificate_key store:ssl/site.der;"))

		require.True(t, IsSecretFile(secret, []byte("not pem"), SecretFileReferences()))
	})

	t.Run("variable path", func(t *testing.T) {
		_, write := secretReferenceFixture(t)
		public := write("ssl/$host.der", "public")
		write("nginx.conf", serverConfig("ssl_certificate_key store:file:ssl/$host.der;"))

		require.False(t, IsSecretFile(public, []byte("not pem"), SecretFileReferences()))
	})
}

// The walk over the configuration root reads every small file, as it did
// before includes were followed. Ordinary files under the root, however many,
// must not use up the include budget and leave a disabled site unread.
// (GHSA-wxp5-6x82-7qf4)
func TestSecretFileReferencesWalkIsNotLimitedByTheIncludeBudget(t *testing.T) {
	_, write := secretReferenceFixture(t)
	secret := write("ssl/ticket.bin", "ticket")
	write("nginx.conf", "events {}\nhttp { include sites-enabled/*; }\n")
	write("sites-available/app", "server {\n"+strings.Repeat("    location /a { proxy_pass http://b; }\n", 64)+
		"    ssl_session_ticket_key ssl/ticket.bin;\n}\n")

	// More than the whole include budget, in files the walk reaches before
	// sites-available: large pages first, then small ones that leave less
	// than the size of the site.
	page := strings.Repeat("#", maxSecretReferenceScanSize-1024)
	var pages []string
	for i := range maxSecretReferenceScanBytes/len(page) + 1 {
		pages = append(pages, write(filepath.Join("html", "page"+strconv.Itoa(i)+".txt"), page))
	}
	for i := range 80 {
		write(filepath.Join("html", "small"+strconv.Itoa(i)+".txt"), strings.Repeat("#", 1024))
	}

	references := SecretFileReferences()
	require.True(t, IsSecretFile(secret, []byte("not pem"), references), "the disabled site must still be read")
	require.False(t, IsSecretFile(pages[0], []byte("not pem"), references), "ordinary files stay readable")
}

// Where spellings are distinct files, a file that only shares its name with a
// referenced secret stays readable.
func TestIsSecretFileKeepsFilesThatShareANameReadable(t *testing.T) {
	root, write := secretReferenceFixture(t)
	write("auth/users", "admin:hash")
	write("nginx.conf", serverConfig("auth_basic_user_file auth/users;"))

	t.Run("same name in another directory", func(t *testing.T) {
		sameName := write("html/users", "public")
		require.False(t, IsSecretFile(sameName, []byte("not pem"), SecretFileReferences()))
	})

	t.Run("other letter case on a case-sensitive file system", func(t *testing.T) {
		if _, err := os.Stat(filepath.Join(root, "auth", "USERS")); err == nil {
			t.Skip("file system ignores letter case")
		}
		otherCase := write("auth/USERS", "public")
		require.False(t, IsSecretFile(otherCase, []byte("not pem"), SecretFileReferences()))
	})
}

// An include the budget no longer covers must stay unread rather than count as
// scanned, so the walk over the configuration root still reads it when it is
// small and lies under the root.
func TestSecretFileReferencesWalkReadsIncludesTheBudgetSkipped(t *testing.T) {
	_, write := secretReferenceFixture(t)
	secret := write("ssl/ticket.bin", "ticket")
	main := "events {}\nhttp { include sites-enabled/*; include block/*.conf; }\n"
	write("nginx.conf", main)
	write("sites-enabled/app", "server { ssl_session_ticket_key ssl/ticket.bin; }\n"+strings.Repeat("#", 4096)+"\n")
	for i := range 63 {
		write(filepath.Join("block", strconv.Itoa(10+i)+".conf"), strings.Repeat("#", maxSecretReferenceScanSize))
	}
	write("block/99.conf", strings.Repeat("#", maxSecretReferenceScanSize-len(main)-100))

	for scan := range 2 {
		require.True(t, IsSecretFile(secret, nil, SecretFileReferences()), "scan %d", scan)
	}
}
