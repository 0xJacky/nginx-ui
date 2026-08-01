package demo

import (
	"fmt"

	"github.com/0xJacky/Nginx-UI/internal/version"
)

// releaseInfo reports the running version as the newest one.
//
// A demo instance refuses to upgrade itself anyway, so asking GitHub on every
// visit to the About page buys nothing and adds a network dependency to a page
// that should always render. Claiming to be current keeps the UI on its
// "you are up to date" path instead of dangling an upgrade the user cannot run.
type releaseInfo struct{}

var _ version.ReleaseProvider = (*releaseInfo)(nil)

func (releaseInfo) Release(channel string) (version.TRelease, bool) {
	current := version.GetVersionInfo().Version
	tag := "v" + current

	release := version.TRelease{
		TagName:     tag,
		Name:        tag,
		PublishedAt: demoBuildTime,
		Body:        "This is a demo instance. Release notes are not fetched here.",
		Type:        version.ReleaseType(channel),
		HTMLURL:     fmt.Sprintf("https://github.com/0xJacky/nginx-ui/releases/tag/%s", tag),
	}

	switch version.ReleaseType(channel) {
	case version.ReleaseTypePrerelease, version.ReleaseTypeDev:
		release.Prerelease = true
	default:
		release.Type = version.ReleaseTypeStable
	}

	return release, true
}
