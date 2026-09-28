package access_list

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loc(i int) *int { return &i }

const siteConfig = `# Main site
server {
    listen 443 ssl;
    server_name nas.example.com; # primary

    # Upstream app
    location / {
        proxy_pass http://127.0.0.1:5000;
    }

    location /share {
        proxy_pass http://127.0.0.1:5000/share;
    }

    location ~ /.well-known/acme-challenge {
        proxy_set_header Host $host;
        proxy_pass http://127.0.0.1:9180;
    }
}
`

func TestStatePublic(t *testing.T) {
	states, err := State(siteConfig)
	require.NoError(t, err)
	require.Len(t, states, 1)
	assert.Equal(t, ModePublic, states[0].Mode)
	assert.Equal(t, "nas.example.com", states[0].ServerName)
	require.Len(t, states[0].Locations, 3)
	for _, l := range states[0].Locations {
		assert.Equal(t, ModeInherit, l.Mode, l.Path)
	}
	assert.True(t, states[0].Locations[2].ACME)
}

func TestApplyServerListAddsACMEException(t *testing.T) {
	out, err := Apply(siteConfig, []Change{{Server: 0, Mode: ModeList, Slug: "lan"}})
	require.NoError(t, err)

	want := `# Main site
server {
    listen 443 ssl;
    server_name nas.example.com; # primary
    include nginx-ui/access/lan.conf;

    # Upstream app
    location / {
        proxy_pass http://127.0.0.1:5000;
    }

    location /share {
        proxy_pass http://127.0.0.1:5000/share;
    }

    location ~ /.well-known/acme-challenge {
        # nginx-ui: keep the ACME challenge reachable
        allow all;
        proxy_set_header Host $host;
        proxy_pass http://127.0.0.1:9180;
    }
}
`
	assert.Equal(t, want, out)

	states, err := State(out)
	require.NoError(t, err)
	assert.Equal(t, ModeList, states[0].Mode)
	assert.Equal(t, "lan", states[0].Slug)
	assert.Equal(t, ModeACME, states[0].Locations[2].Mode)

	// Going back to public restores the original file byte for byte.
	back, err := Apply(out, []Change{{Server: 0, Mode: ModePublic}})
	require.NoError(t, err)
	assert.Equal(t, siteConfig, back)
}

func TestApplyLocationModes(t *testing.T) {
	restricted, err := Apply(siteConfig, []Change{{Server: 0, Mode: ModeList, Slug: "lan"}})
	require.NoError(t, err)

	out, err := Apply(restricted, []Change{
		{Server: 0, Location: loc(1), Mode: ModePublic},
		{Server: 0, Location: loc(0), Mode: ModeList, Slug: "lan-office"},
	})
	require.NoError(t, err)

	states, err := State(out)
	require.NoError(t, err)
	assert.Equal(t, ModeList, states[0].Locations[0].Mode)
	assert.Equal(t, "lan-office", states[0].Locations[0].Slug)
	assert.Equal(t, ModePublic, states[0].Locations[1].Mode)
	assert.Contains(t, out, "    location /share {\n        allow all;\n        proxy_pass")
	assert.Contains(t, out, "    location / {\n        include nginx-ui/access/lan-office.conf;\n        proxy_pass")

	// Switching a list location back to inherit leaves only the original body.
	out, err = Apply(out, []Change{{Server: 0, Location: loc(0), Mode: ModeInherit}, {Server: 0, Location: loc(1), Mode: ModeInherit}})
	require.NoError(t, err)
	assert.Equal(t, restricted, out)
}

func TestApplyACMEExplicitChoiceWins(t *testing.T) {
	restricted, err := Apply(siteConfig, []Change{{Server: 0, Mode: ModeList, Slug: "lan"}})
	require.NoError(t, err)

	// An explicit list on the challenge location replaces the managed exception.
	out, err := Apply(restricted, []Change{{Server: 0, Location: loc(2), Mode: ModeList, Slug: "lan"}})
	require.NoError(t, err)
	assert.NotContains(t, out, ACMEMarker)
	states, err := State(out)
	require.NoError(t, err)
	assert.Equal(t, ModeList, states[0].Locations[2].Mode)

	// Asking for inherit again brings the managed exception back.
	out, err = Apply(out, []Change{{Server: 0, Location: loc(2), Mode: ModeInherit}})
	require.NoError(t, err)
	assert.Equal(t, restricted, out)
}

func TestApplySwitchesList(t *testing.T) {
	out, err := Apply(siteConfig, []Change{{Server: 0, Mode: ModeList, Slug: "lan"}})
	require.NoError(t, err)
	out, err = Apply(out, []Change{{Server: 0, Mode: ModeList, Slug: "office"}})
	require.NoError(t, err)
	assert.Contains(t, out, "include nginx-ui/access/office.conf;")
	assert.NotContains(t, out, "lan.conf")
}

func TestApplyRefusesManualRules(t *testing.T) {
	content := `server {
    listen 80;
    allow 10.0.0.0/8;
    deny all;
    location / {
        deny 1.2.3.4;
        allow all;
    }
}
`
	states, err := State(content)
	require.NoError(t, err)
	assert.Equal(t, ModeManual, states[0].Mode)
	assert.Equal(t, ModeManual, states[0].Locations[0].Mode)

	_, err = Apply(content, []Change{{Server: 0, Mode: ModeList, Slug: "lan"}})
	assertErrCode(t, err, ErrManualRules)
	_, err = Apply(content, []Change{{Server: 0, Location: loc(0), Mode: ModeInherit}})
	assertErrCode(t, err, ErrManualRules)
}

func TestApplyValidatesChange(t *testing.T) {
	_, err := Apply(siteConfig, []Change{{Server: 0, Mode: ModeList, Slug: "../x"}})
	assertErrCode(t, err, ErrInvalidSlug)
	_, err = Apply(siteConfig, []Change{{Server: 0, Mode: ModeInherit}})
	assertErrCode(t, err, ErrInvalidMode)
	_, err = Apply(siteConfig, []Change{{Server: 3, Mode: ModePublic}})
	assertErrCode(t, err, ErrServerNotFound)
	_, err = Apply(siteConfig, []Change{{Server: 0, Location: loc(9), Mode: ModePublic}})
	assertErrCode(t, err, ErrLocationNotFound)
}

func TestStateAcceptsAbsoluteInclude(t *testing.T) {
	content := "server {\n\tlisten 80;\n\tinclude /etc/nginx/nginx-ui/access/lan.conf;\n}\n"
	states, err := State(content)
	require.NoError(t, err)
	assert.Equal(t, ModeList, states[0].Mode)
	assert.Equal(t, "lan", states[0].Slug)

	out, err := Apply(content, []Change{{Server: 0, Mode: ModePublic}})
	require.NoError(t, err)
	assert.Equal(t, "server {\n\tlisten 80;\n}\n", out)
}

func TestApplyInsertsWithTabs(t *testing.T) {
	content := "server {\n\tlisten 80;\n\tlocation / {\n\t\treturn 200;\n\t}\n}\n"
	out, err := Apply(content, []Change{{Server: 0, Mode: ModeList, Slug: "lan"}, {Server: 0, Location: loc(0), Mode: ModePublic}})
	require.NoError(t, err)
	assert.Equal(t, "server {\n\tlisten 80;\n\tinclude nginx-ui/access/lan.conf;\n\tlocation / {\n\t\tallow all;\n\t\treturn 200;\n\t}\n}\n", out)
}

func TestApplyMultipleServersAndStreams(t *testing.T) {
	content := `server {
    listen 80;
    server_name a.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name a.example.com;
}
`
	out, err := Apply(content, []Change{{Server: 0, Mode: ModeList, Slug: "lan"}, {Server: 1, Mode: ModeList, Slug: "lan"}})
	require.NoError(t, err)
	states, err := State(out)
	require.NoError(t, err)
	require.Len(t, states, 2)
	assert.Equal(t, "lan", states[0].Slug)
	assert.Equal(t, "lan", states[1].Slug)

	stream := "server {\n    listen 2222;\n    proxy_pass 10.0.0.2:22;\n}\n"
	out, err = Apply(stream, []Change{{Server: 0, Mode: ModeList, Slug: "lan"}})
	require.NoError(t, err)
	assert.Equal(t, "server {\n    listen 2222;\n    include nginx-ui/access/lan.conf;\n    proxy_pass 10.0.0.2:22;\n}\n", out)
}

func TestStructuredStateAndApply(t *testing.T) {
	cfg, err := nginx.ParseNgxConfigByContent(siteConfig)
	require.NoError(t, err)

	require.NoError(t, ApplyTo(cfg, []Change{
		{Server: 0, Mode: ModeList, Slug: "lan"},
		{Server: 0, Location: loc(1), Mode: ModePublic},
	}))

	states, err := StateOf(cfg)
	require.NoError(t, err)
	assert.Equal(t, ModeList, states[0].Mode)
	assert.Equal(t, "lan", states[0].Slug)
	assert.Equal(t, ModeInherit, states[0].Locations[0].Mode)
	assert.Equal(t, ModePublic, states[0].Locations[1].Mode)
	assert.Equal(t, ModeACME, states[0].Locations[2].Mode)

	// The include follows server_name.
	var names []string
	for _, d := range cfg.Servers[0].Directives {
		names = append(names, d.Directive)
	}
	assert.Equal(t, []string{"listen", "server_name", "include"}, names)

	// A build and re-parse round trip keeps the state, including the marker.
	built, err := cfg.BuildConfig()
	require.NoError(t, err)
	textStates, err := State(built)
	require.NoError(t, err)
	assert.Equal(t, states, textStates)

	require.NoError(t, ApplyTo(cfg, []Change{{Server: 0, Mode: ModePublic}, {Server: 0, Location: loc(1), Mode: ModeInherit}}))
	states, err = StateOf(cfg)
	require.NoError(t, err)
	assert.Equal(t, ModePublic, states[0].Mode)
	for _, l := range states[0].Locations {
		assert.Equal(t, ModeInherit, l.Mode, l.Path)
	}
	assert.NotContains(t, cfg.Servers[0].Locations[2].Content, "allow")
}

func TestStructuredManualServer(t *testing.T) {
	cfg, err := nginx.ParseNgxConfigByContent("server {\n    listen 80;\n    deny 1.2.3.4;\n}\n")
	require.NoError(t, err)
	states, err := StateOf(cfg)
	require.NoError(t, err)
	assert.Equal(t, ModeManual, states[0].Mode)
	assertErrCode(t, ApplyTo(cfg, []Change{{Server: 0, Mode: ModeList, Slug: "lan"}}), ErrManualRules)
}

func TestSummarize(t *testing.T) {
	one := "server {\n    include nginx-ui/access/lan.conf;\n}\n"
	mode, slug := Summarize(one + one)
	assert.Equal(t, ModeList, mode)
	assert.Equal(t, "lan", slug)

	mode, _ = Summarize(one + "server {\n    listen 80;\n}\n")
	assert.Equal(t, ModeMixed, mode)

	mode, _ = Summarize("server {\n    listen 80;\n}\n")
	assert.Equal(t, ModePublic, mode)

	mode, _ = Summarize(one + "server {\n    deny all;\n}\n")
	assert.Equal(t, ModeManual, mode)

	mode, _ = Summarize("server {")
	assert.Empty(t, mode)
}
