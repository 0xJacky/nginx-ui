package releasesign

// trustedPublicKeys contains the release-signing keys embedded in every
// official binary. They only verify nginx-ui release artifacts, never a
// plugin. During rotation, add the new key before switching the GitHub
// Actions signing secret, then remove the old key in a later release.
var trustedPublicKeys = []string{
	`untrusted comment: minisign public key: E099146682BA5032
RWQyULqCZhSZ4LTBwQQlPCm5HS4qjbxPv75e56lU2y3cc9kviWsNqW4v`,
}

// pluginPublicKeys contains the keys that sign the official plugins, the
// partner certificates and the partner keyring. They are kept apart from the
// release keys so that the CI of a plugin repository never holds a key that
// could sign an nginx-ui upgrade. Rotate them the same way.
var pluginPublicKeys = []string{
	`untrusted comment: minisign public key 088ACCA5E13F459F
RWSfRT/hpcyKCKvzHZ4sPfRnT7XK5voUPXys7PN8L5dFry+rx5cA6vO0`,
}

// TrustedPublicKeys returns an isolated copy of the pinned release keys.
func TrustedPublicKeys() []string {
	return append([]string(nil), trustedPublicKeys...)
}

// PluginPublicKeys returns an isolated copy of the pinned official plugin
// keys.
func PluginPublicKeys() []string {
	return append([]string(nil), pluginPublicKeys...)
}
