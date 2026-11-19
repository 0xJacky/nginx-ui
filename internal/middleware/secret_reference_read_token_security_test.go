package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

const (
	hashTicketKey    = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	controlTicketKey = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	retiredNote      = "retired-file-that-no-directive-names"
)

// referencedSecretFiles are secret only because a directive names them: none
// has a secret extension or PEM content. Each is written to the configuration
// the way nginx accepts it, and each stands for one way a scan can read the
// configuration differently from nginx.
var referencedSecretFiles = []struct{ name, path, secret string }{
	{"control key", "ssl/ticket-control.bin", controlTicketKey},
	{"quoted path holding a hash", "ssl/ticket#1.bin", hashTicketKey},
	{"unquoted path holding a hash", "ssl/site#2.pass", "passphrase-in-unquoted-hash-file"},
	{"statement after a quoted hash on the line", "ssl/after-hash.pass", "passphrase-after-quoted-hash"},
	{"quoted directive name and a colon in the path", "ssl/ticket:3.bin", "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"},
	{"escaped quote in the path", `ssl/pa"ss.txt`, "passphrase-in-escaped-quote-file"},
	{"QUIC host key", "ssl/quic-host.bin", "quic-host-key-material"},
	{"directive in a file included from outside the root", "ssl/included.pass", "passphrase-named-by-outside-include"},
	{"directive in an included data file", "ssl/fragment.pass", "passphrase-named-by-data-file-fragment"},
	{"key read through the OpenSSL store loader", "ssl/store-key.der", "key-named-by-store-file-uri"},
}

type secretReferenceReadState struct {
	storedSecretReadState
	confDir     string
	hashPath    string
	controlPath string
}

// setupSecretReferenceReadEndpoints adds the referenced secret files to the
// stored secret fixture, next to files that must stay readable.
func setupSecretReferenceReadEndpoints(t *testing.T) secretReferenceReadState {
	t.Helper()
	state := setupStoredSecretReadEndpoints(t)

	confDir := nginx.GetConfPath()
	for _, file := range referencedSecretFiles {
		require.NoError(t, os.WriteFile(filepath.Join(confDir, file.path), []byte(file.secret), 0o600))
	}
	hashPath := filepath.Join(confDir, "ssl", "ticket#1.bin")
	controlPath := filepath.Join(confDir, "ssl", "ticket-control.bin")

	outside := filepath.Join(t.TempDir(), "tls.conf")
	require.NoError(t, os.WriteFile(outside, []byte("ssl_password_file ssl/included.pass;\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(confDir, "conf.d"), 0o700))
	for path, content := range map[string]string{
		"conf.d/fragment.pem": "ssl_password_file ssl/fragment.pass;\n",
		"conf.d/notes#1.conf": "# notes\n",
		"ssl/retired.txt":     retiredNote,
		"nginx.conf": `events {}
http {
    ssl_session_ticket_key ssl/ticket-control.bin;
    ssl_session_ticket_key "ssl/ticket#1.bin";
    ssl_password_file ssl/site#2.pass;
    add_header X-Color "#fff"; ssl_password_file ssl/after-hash.pass;
    "ssl_session_ticket_key" ssl/ticket:3.bin;
    ssl_password_file "ssl/pa\"ss.txt";
    quic_host_key ssl/quic-host.bin;
    include ` + outside + `;
    include conf.d/fragment.pem;
    ssl_certificate_key store:file:` + filepath.Join(confDir, "ssl", "store-key.der") + `;
    ssl_certificate ssl/example.crt;
    # ssl_password_file ssl/retired.txt;
}
`,
	} {
		require.NoError(t, os.WriteFile(filepath.Join(confDir, path), []byte(content), 0o600))
	}

	// Earlier revisions of both ticket keys, as the configuration editor
	// records them.
	for _, backup := range []model.ConfigBackup{
		{Name: "ticket#1.bin", FilePath: hashPath, Content: hashTicketKey},
		{Name: "ticket-control.bin", FilePath: controlPath, Content: controlTicketKey},
	} {
		require.NoError(t, model.UseDB().Create(&backup).Error)
	}

	return secretReferenceReadState{
		storedSecretReadState: state,
		confDir:               confDir,
		hashPath:              hashPath,
		controlPath:           controlPath,
	}
}

// A file nginx loads a secret from must be refused to an api:read token
// however its directive is written and wherever that directive lives. The
// control key shows the rule working; every other file must behave the same.
// (GHSA-wxp5-6x82-7qf4)
func TestAPIReadServiceTokenCannotReadSecretFileNamedWithHash(t *testing.T) {
	state := setupSecretReferenceReadEndpoints(t)

	for _, file := range referencedSecretFiles {
		for _, spelling := range []struct{ name, path string }{
			{"relative path", file.path},
			{"absolute path", filepath.Join(state.confDir, file.path)},
		} {
			t.Run(file.name+"/"+spelling.name, func(t *testing.T) {
				recorder := storedSecretRequest(state.local, configPath(spelling.path), state.readToken, 0)
				require.NotContains(t, recorder.Body.String(), file.secret)
				require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
				body := decodeJSONBody(t, recorder)
				require.Equal(t, "config", body["scope"])
				require.EqualValues(t, 40019, body["code"])
			})
		}
	}

	t.Run("configuration history", func(t *testing.T) {
		recorder := storedSecretRequest(state.local, "/api/config_histories", state.readToken, 0)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		body := decodeJSONBody(t, recorder)
		require.Equal(t, appsettings.RedactedSensitiveValue, configBackupContent(t, body, "ticket-control.bin"))
		require.Equal(t, appsettings.RedactedSensitiveValue, configBackupContent(t, body, "ticket#1.bin"))
	})
}

// An api:read token keeps reading everything that is not a secret: the
// configuration itself, a certificate a directive names, a file only a comment
// mentions, and a file whose own name holds a '#'.
func TestAPIReadServiceTokenKeepsReadingFilesNoSecretDirectiveNames(t *testing.T) {
	state := setupSecretReferenceReadEndpoints(t)

	for _, file := range []struct{ path, content string }{
		{"nginx.conf", "ssl_session_ticket_key"},
		{"ssl/example.crt", state.certPEM},
		{"ssl/retired.txt", retiredNote},
		{"conf.d/notes#1.conf", "# notes\n"},
		{"conf.d/fragment.pem", "ssl_password_file ssl/fragment.pass;\n"},
	} {
		t.Run(file.path, func(t *testing.T) {
			recorder := storedSecretRequest(state.local, configPath(file.path), state.readToken, 0)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Contains(t, decodeJSONBody(t, recorder)["content"], file.content)
		})
	}

	t.Run("configuration history", func(t *testing.T) {
		recorder := storedSecretRequest(state.local, "/api/config_histories", state.readToken, 0)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		body := decodeJSONBody(t, recorder)
		require.Equal(t, "events {}\n# previous revision\n", configBackupContent(t, body, "nginx.conf"))
	})
}

// Callers that are allowed, or not allowed at all, are unchanged: api:write
// tokens and administrators read every referenced file and its history, a
// request without credentials reads none.
func TestSecretFileNamedWithHashKeepsOtherCallers(t *testing.T) {
	state := setupSecretReferenceReadEndpoints(t)

	for _, file := range referencedSecretFiles {
		for _, caller := range []struct{ name, token string }{
			{"api:write token", state.writeToken},
			{"interactive administrator", state.adminToken},
		} {
			t.Run(caller.name+"/"+file.name, func(t *testing.T) {
				recorder := storedSecretRequest(state.local, configPath(file.path), caller.token, 0)
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.Equal(t, file.secret, decodeJSONBody(t, recorder)["content"])
			})
		}

		t.Run("no credentials/"+file.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			state.local.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, configPath(file.path), nil))
			require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
			require.NotContains(t, recorder.Body.String(), file.secret)
		})
	}

	for _, caller := range []struct{ name, token string }{
		{"api:write token", state.writeToken},
		{"interactive administrator", state.adminToken},
	} {
		t.Run(caller.name+"/configuration history", func(t *testing.T) {
			recorder := storedSecretRequest(state.local, "/api/config_histories", caller.token, 0)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			body := decodeJSONBody(t, recorder)
			require.Equal(t, hashTicketKey, configBackupContent(t, body, "ticket#1.bin"))
			require.Equal(t, controlTicketKey, configBackupContent(t, body, "ticket-control.bin"))
		})
	}
}

// On a case-insensitive file system another spelling of the path opens the
// same file and must be refused as well, and so must a hard link to it on any
// file system. (GHSA-wxp5-6x82-7qf4)
func TestAPIReadServiceTokenCannotReadReferencedSecretFileByRespelledPath(t *testing.T) {
	state := setupSecretReferenceReadEndpoints(t)

	link := filepath.Join(state.confDir, "conf.d", "notes.txt")
	require.NoError(t, os.Link(state.controlPath, link))

	for _, spelling := range []struct{ name, path, original, secret string }{
		{"file name", "ssl/TICKET-CONTROL.BIN", "ssl/ticket-control.bin", controlTicketKey},
		{"directory", "SSL/ticket-control.bin", "ssl/ticket-control.bin", controlTicketKey},
		{"absolute path", filepath.Join(state.confDir, "Ssl", "Ticket-Control.bin"), "ssl/ticket-control.bin", controlTicketKey},
		{"file name under full case folding", "ssl/site#2.paß", "ssl/site#2.pass", "passphrase-in-unquoted-hash-file"},
		{"directory under full case folding", "ßl/ticket-control.bin", "ssl/ticket-control.bin", controlTicketKey},
		{"hard link", "conf.d/notes.txt", "ssl/ticket-control.bin", controlTicketKey},
	} {
		t.Run(spelling.name, func(t *testing.T) {
			path := spelling.path
			if !filepath.IsAbs(path) {
				path = filepath.Join(state.confDir, path)
			}
			original, err := os.Stat(filepath.Join(state.confDir, spelling.original))
			require.NoError(t, err)
			if variant, err := os.Stat(path); err != nil || !os.SameFile(original, variant) {
				t.Skip("file system keeps this spelling apart")
			}
			recorder := storedSecretRequest(state.local, configPath(spelling.path), state.readToken, 0)
			require.NotContains(t, recorder.Body.String(), spelling.secret)
			require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
			require.EqualValues(t, 40019, decodeJSONBody(t, recorder)["code"])
		})
	}
}
