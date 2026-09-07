package nginx

import (
	"strings"
	"testing"
)

// TestBuildConfig_UpstreamCommentIsNotRepeated guards against a comment on one
// upstream directive leaking onto every following directive that has none.
func TestBuildConfig_UpstreamCommentIsNotRepeated(t *testing.T) {
	content := `upstream backend {
    # primary node
    server 10.0.0.1:8080;
    server 10.0.0.2:8080;
    server 10.0.0.3:8080;
}
`

	ngxConfig, err := ParseNgxConfigByContent(content)
	if err != nil {
		t.Fatalf("ParseNgxConfigByContent() error = %v", err)
	}

	built, err := ngxConfig.BuildConfig()
	if err != nil {
		t.Fatalf("BuildConfig() error = %v", err)
	}

	if got := strings.Count(built, "# primary node"); got != 1 {
		t.Fatalf("BuildConfig() emitted the comment %d times, want 1:\n%s", got, built)
	}

	for _, server := range []string{"server 10.0.0.1:8080;", "server 10.0.0.2:8080;", "server 10.0.0.3:8080;"} {
		if !strings.Contains(built, server) {
			t.Fatalf("BuildConfig() = %q, want it to contain %q", built, server)
		}
	}
}

// TestBuildConfig_UpstreamPerDirectiveComments keeps each directive paired with
// its own comment rather than with whichever comment came last.
func TestBuildConfig_UpstreamPerDirectiveComments(t *testing.T) {
	content := `upstream backend {
    # first
    server 10.0.0.1:8080;
    # second
    server 10.0.0.2:8080;
    server 10.0.0.3:8080;
}
`

	ngxConfig, err := ParseNgxConfigByContent(content)
	if err != nil {
		t.Fatalf("ParseNgxConfigByContent() error = %v", err)
	}

	built, err := ngxConfig.BuildConfig()
	if err != nil {
		t.Fatalf("BuildConfig() error = %v", err)
	}

	firstIdx := strings.Index(built, "# first")
	secondIdx := strings.Index(built, "# second")
	thirdIdx := strings.Index(built, "server 10.0.0.3:8080;")
	if firstIdx == -1 || secondIdx == -1 || thirdIdx == -1 {
		t.Fatalf("BuildConfig() = %q, want it to contain both comments and the last server", built)
	}
	if got := strings.Count(built, "# first"); got != 1 {
		t.Fatalf("BuildConfig() emitted %q %d times, want 1:\n%s", "# first", got, built)
	}
	if got := strings.Count(built, "# second"); got != 1 {
		t.Fatalf("BuildConfig() emitted %q %d times, want 1:\n%s", "# second", got, built)
	}
	// The uncommented third server must not inherit the second comment.
	if secondIdx > thirdIdx {
		t.Fatalf("BuildConfig() placed %q after the uncommented server:\n%s", "# second", built)
	}
}

// hasCommentLine reports whether built contains comment as a whole line,
// ignoring the indentation the dumper chooses.
func hasCommentLine(built, comment string) bool {
	for _, line := range strings.Split(built, "\n") {
		if strings.TrimSpace(line) == comment {
			return true
		}
	}
	return false
}

// TestBuildConfig_CommentKeepsInnerHash guards the # characters that belong to
// the comment text, such as a URL fragment or an issue number. The four places
// a comment is read are covered: server, location, directive and upstream.
func TestBuildConfig_CommentKeepsInnerHash(t *testing.T) {
	content := `# server #0 terminates TLS
server {
    # see https://example.com/docs#tls
    listen 80;

    # route #1 of #2
    location / {
        proxy_pass http://backend;
    }
}

upstream backend {
    # shard #2 of #3
    server 10.0.0.1:8080;
}
`

	ngxConfig, err := ParseNgxConfigByContent(content)
	if err != nil {
		t.Fatalf("ParseNgxConfigByContent() error = %v", err)
	}

	built, err := ngxConfig.BuildConfig()
	if err != nil {
		t.Fatalf("BuildConfig() error = %v", err)
	}

	for _, comment := range []string{
		"# server #0 terminates TLS",
		"# see https://example.com/docs#tls",
		"# route #1 of #2",
		"# shard #2 of #3",
	} {
		if !hasCommentLine(built, comment) {
			t.Fatalf("BuildConfig() = %q, want it to contain the line %q", built, comment)
		}
	}
}

// TestBuildConfig_CommentMarkerIsNormalised pins the marker handling that was
// already in place, so only the comment text changes.
func TestBuildConfig_CommentMarkerIsNormalised(t *testing.T) {
	content := `server {
    ## double marker
    listen 80;
}

upstream backend {
    #no space
    server 10.0.0.1:8080;
}
`

	ngxConfig, err := ParseNgxConfigByContent(content)
	if err != nil {
		t.Fatalf("ParseNgxConfigByContent() error = %v", err)
	}

	built, err := ngxConfig.BuildConfig()
	if err != nil {
		t.Fatalf("BuildConfig() error = %v", err)
	}

	for _, comment := range []string{"# double marker", "# no space"} {
		if !hasCommentLine(built, comment) {
			t.Fatalf("BuildConfig() = %q, want it to contain the line %q", built, comment)
		}
	}
}
