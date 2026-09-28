package cert

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const http01TestSiteFile = "/etc/nginx/sites-enabled/example.com"

func http01Listen80() []nginx.ServerListen {
	return []nginx.ServerListen{{Port: "80"}}
}

func http01Listen443() []nginx.ServerListen {
	return []nginx.ServerListen{{Port: "443", SSL: true}}
}

func http01ChallengeLocation(proxyPass string) nginx.ServerLocation {
	return nginx.ServerLocation{Modifier: "^~", Path: "/.well-known/acme-challenge", ProxyPass: proxyPass}
}

func http01Block(names []string, listens []nginx.ServerListen, serverReturn string, locations ...nginx.ServerLocation) nginx.ServerBlock {
	return nginx.ServerBlock{
		File:        http01TestSiteFile,
		ServerNames: names,
		Listens:     listens,
		Return:      serverReturn,
		Locations:   locations,
	}
}

func TestValidateHTTP01ChallengeConfig(t *testing.T) {
	name := []string{"example.com"}
	redirect := "301 https://$host$request_uri"
	tests := []struct {
		name    string
		blocks  []nginx.ServerBlock
		wantErr string
	}{
		{
			name:   "valid HTTP listener and proxy",
			blocks: []nginx.ServerBlock{http01Block(name, http01Listen80(), "", http01ChallengeLocation("http://127.0.0.1:9180"))},
		},
		{
			name:   "localhost proxy with path",
			blocks: []nginx.ServerBlock{http01Block(name, http01Listen80(), "", http01ChallengeLocation("http://localhost:9180/"))},
		},
		{
			// ACME validators follow the HTTPS redirect, so it only fails
			// because no HTTPS server routes the challenge.
			name:    "server level redirect without HTTPS challenge route",
			blocks:  []nginx.ServerBlock{http01Block(name, http01Listen80(), redirect)},
			wantErr: "the server block for example.com on *:80 in " + http01TestSiteFile + " redirects to HTTPS at server level (return " + redirect + "), and no HTTPS server block for example.com routes /.well-known/acme-challenge/ to 127.0.0.1:9180",
		},
		{
			name:    "server level redirect overrides challenge proxy",
			blocks:  []nginx.ServerBlock{http01Block(name, http01Listen80(), redirect, http01ChallengeLocation("http://127.0.0.1:9180"))},
			wantErr: "no HTTPS server block for example.com",
		},
		{
			name: "server level redirect to HTTPS server routing the challenge",
			blocks: []nginx.ServerBlock{
				http01Block(name, http01Listen80(), redirect),
				http01Block(name, http01Listen443(), "", http01ChallengeLocation("http://127.0.0.1:9180")),
			},
		},
		{
			name: "location level redirect to HTTPS server routing the challenge",
			blocks: []nginx.ServerBlock{
				http01Block(name, http01Listen80(), "", nginx.ServerLocation{Path: "/", Return: redirect}),
				http01Block(name, http01Listen443(), "", nginx.ServerLocation{Modifier: "~", Path: "/.well-known/acme-challenge", ProxyPass: "http://127.0.0.1:9180"}),
			},
		},
		{
			name: "redirect to HTTPS server without challenge route",
			blocks: []nginx.ServerBlock{
				http01Block(name, http01Listen80(), "", nginx.ServerLocation{Path: "/", Return: redirect}),
				http01Block(name, http01Listen443(), "", nginx.ServerLocation{Path: "/", ProxyPass: "http://127.0.0.1:3000"}),
			},
			wantErr: "redirects to HTTPS in location /, and no HTTPS server block for example.com",
		},
		{
			name:    "server level non-redirect return",
			blocks:  []nginx.ServerBlock{http01Block(name, http01Listen80(), "444", http01ChallengeLocation("http://127.0.0.1:9180"))},
			wantErr: "has a server-level return (return 444), which answers before any location is reached",
		},
		{
			name:    "challenge only on HTTPS",
			blocks:  []nginx.ServerBlock{http01Block(name, http01Listen443(), "", http01ChallengeLocation("http://127.0.0.1:9180"))},
			wantErr: "no server block listens on port 80 for example.com",
		},
		{
			name:    "wrong challenge port",
			blocks:  []nginx.ServerBlock{http01Block(name, http01Listen80(), "", http01ChallengeLocation("http://127.0.0.1:9181"))},
			wantErr: "has a /.well-known/acme-challenge location that does not proxy to 127.0.0.1:9180 (proxy_pass http://127.0.0.1:9181)",
		},
		{
			name:    "no challenge location",
			blocks:  []nginx.ServerBlock{http01Block(name, http01Listen80(), "", nginx.ServerLocation{Path: "/", ProxyPass: "http://127.0.0.1:3000"})},
			wantErr: "the server block for example.com on *:80 in " + http01TestSiteFile + " has no /.well-known/acme-challenge location proxying to 127.0.0.1:9180",
		},
		{
			// Only the default server of the socket answers the name.
			name: "name served by another block's default server",
			blocks: []nginx.ServerBlock{{
				File:        "/etc/nginx/conf.d/default.conf",
				ServerNames: []string{"_"},
				Listens:     []nginx.ServerListen{{Port: "80", DefaultServer: true}},
			}},
			wantErr: "no server_name matches example.com on *:80, so Nginx answers with the default server block in /etc/nginx/conf.d/default.conf, which has no /.well-known/acme-challenge location",
		},
		{
			// An unrelated IPv6 default server that routes the challenge does
			// not decide for a name its IPv4 block serves.
			name: "unrelated IPv6 default server does not hide the name-matched block",
			blocks: []nginx.ServerBlock{
				http01Block(name, http01Listen80(), ""),
				{
					File:        "/etc/nginx/sites-enabled/other",
					ServerNames: []string{"other.example.com"},
					Listens:     []nginx.ServerListen{{Port: "80", IPv6: true, DefaultServer: true}},
					Locations:   []nginx.ServerLocation{http01ChallengeLocation("http://127.0.0.1:9180")},
				},
			},
			wantErr: "the server block for example.com on *:80 in " + http01TestSiteFile + " has no /.well-known/acme-challenge location",
		},
		{
			// The routing block lives on the wildcard socket, but the
			// specific-address socket answers the name with a block that
			// does not route the challenge.
			name: "specific address socket does not route",
			blocks: []nginx.ServerBlock{
				http01Block(name, http01Listen80(), "", http01ChallengeLocation("http://127.0.0.1:9180")),
				{
					File:        "/etc/nginx/sites-enabled/public",
					ServerNames: name,
					Listens:     []nginx.ServerListen{{Addr: "192.0.2.10", Port: "80"}},
				},
			},
			wantErr: "the server block for example.com on 192.0.2.10:80 in /etc/nginx/sites-enabled/public has no /.well-known/acme-challenge location",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHTTP01ChallengeConfig(tt.blocks, "9180", []string{"example.com"})
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "HTTP-01 route is unavailable")
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateHTTP01ChallengeConfigRequiresEachIdentifier(t *testing.T) {
	blocks := []nginx.ServerBlock{
		http01Block([]string{"example.com"}, http01Listen80(), "", http01ChallengeLocation("http://127.0.0.1:9180")),
		http01Block([]string{"www.example.com"}, http01Listen80(), ""),
	}

	assert.NoError(t, validateHTTP01ChallengeConfig(blocks, "9180", []string{"example.com"}))
	err := validateHTTP01ChallengeConfig(blocks, "9180", []string{"example.com", "www.example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "www.example.com")
}

func TestValidateHTTP01ChallengeConfigReadsEffectiveConfig(t *testing.T) {
	previousBlocks := http01ProbeServerBlocks
	previousChallengePort := settings.CertSettings.HTTPChallengePort
	settings.CertSettings.HTTPChallengePort = "9180"
	t.Cleanup(func() {
		http01ProbeServerBlocks = previousBlocks
		settings.CertSettings.HTTPChallengePort = previousChallengePort
	})

	http01ProbeServerBlocks = func() ([]nginx.ServerBlock, error) { return nil, nginx.ErrNginxTOutputEmpty }
	err := ValidateHTTP01ChallengeConfig([]string{"example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the local Nginx configuration could not be read")

	// The analyzer matches server blocks by name, whatever file or site
	// name they come from.
	http01ProbeServerBlocks = func() ([]nginx.ServerBlock, error) {
		return []nginx.ServerBlock{{
			File:        "/etc/nginx/conf.d/anything.conf",
			ServerNames: []string{"example.com"},
			Listens:     http01Listen80(),
			Locations:   []nginx.ServerLocation{http01ChallengeLocation("http://127.0.0.1:9180")},
		}}, nil
	}
	assert.NoError(t, ValidateHTTP01ChallengeConfig([]string{"example.com"}))
}
