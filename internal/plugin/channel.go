package plugin

import (
	"strings"

	"github.com/0xJacky/Nginx-UI/model"
)

// Release channels, ordered from the most to the least stable.
const (
	ChannelStable = "stable"
	ChannelBeta   = "beta"
	ChannelDev    = "dev"
)

// devIdentifiers are the prerelease identifiers that mean the dev channel.
// Any other prerelease identifier means beta.
var devIdentifiers = map[string]bool{
	"alpha": true, "dev": true, "nightly": true,
	"snapshot": true, "canary": true, "preview": true,
}

// IsValidChannel reports whether c names a channel.
func IsValidChannel(c string) bool {
	return c == ChannelStable || c == ChannelBeta || c == ChannelDev
}

// NormalizeChannel maps an empty or unknown value to the stable channel.
func NormalizeChannel(c string) string {
	if IsValidChannel(c) {
		return c
	}
	return ChannelStable
}

// ChannelRank orders the channels, a higher rank is less stable.
func ChannelRank(c string) int {
	switch c {
	case ChannelBeta:
		return 1
	case ChannelDev:
		return 2
	default:
		return 0
	}
}

// lessStableChannel returns the channel with the higher rank.
func lessStableChannel(a, b string) string {
	a, b = NormalizeChannel(a), NormalizeChannel(b)
	if ChannelRank(b) > ChannelRank(a) {
		return b
	}
	return a
}

// InferChannel derives the channel of a version: no prerelease part is
// stable, a prerelease that starts with a dev identifier is dev, any other
// prerelease is beta.
func InferChannel(version string) string {
	parsed, ok := parseVersion(version)
	if !ok || parsed.prerelease == "" {
		return ChannelStable
	}
	first, _, _ := strings.Cut(parsed.prerelease, ".")
	if devIdentifiers[strings.ToLower(first)] {
		return ChannelDev
	}
	return ChannelBeta
}

// channelOfRelease is the channel of a release: the one the catalog names, or
// the one its version implies.
func channelOfRelease(r *CatalogRelease) string {
	if IsValidChannel(r.Channel) {
		return r.Channel
	}
	return InferChannel(r.Version)
}

// rowReleaseChannel is the channel of the release a row records, read from
// the version for a row that predates the column.
func rowReleaseChannel(row *model.Plugin) string {
	if IsValidChannel(row.ReleaseChannel) {
		return row.ReleaseChannel
	}
	return InferChannel(row.Version)
}
