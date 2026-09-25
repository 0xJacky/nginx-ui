package releasesign

// trustedPublicKeys contains the release-signing keys embedded in every
// official binary. During rotation, add the new key before switching the
// GitHub Actions signing secret, then remove the old key in a later release.
var trustedPublicKeys = []string{
	`untrusted comment: minisign public key: E099146682BA5032
RWQyULqCZhSZ4LTBwQQlPCm5HS4qjbxPv75e56lU2y3cc9kviWsNqW4v`,
}

// TrustedPublicKeys returns an isolated copy of the pinned release keys.
func TrustedPublicKeys() []string {
	return append([]string(nil), trustedPublicKeys...)
}

// partnerPublicKeys contains the keys of the partners whose plugin packages
// are shown as verified. Partners are added here.
var partnerPublicKeys = []string{}

// PartnerPublicKeys returns an isolated copy of the pinned partner keys.
func PartnerPublicKeys() []string {
	return append([]string(nil), partnerPublicKeys...)
}
