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
