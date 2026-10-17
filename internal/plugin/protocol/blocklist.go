package protocol

// BlocklistFetchParams is the payload of blocklist.fetch.
type BlocklistFetchParams struct {
	// Source is the source kind code declared in the manifest, without any host prefix.
	Source string `json:"source"`
	// Config holds the field values the user filled in, keyed by field key.
	Config map[string]string `json:"config"`
}

// BlocklistFetchResult is the reply to blocklist.fetch.
type BlocklistFetchResult struct {
	// Entries is the complete current list of the source. Empty denies nothing.
	Entries []BlocklistEntry `json:"entries"`
	// TTLSeconds is how long the list stays fresh. 0 leaves it to the host.
	TTLSeconds int `json:"ttl_seconds,omitempty"`
}

// BlocklistEntry is one address or network to deny.
type BlocklistEntry struct {
	// CIDR is an IPv4 or IPv6 address, or a network in CIDR notation.
	CIDR string `json:"cidr"`
	// Reason says why the source lists the entry, for display.
	Reason string `json:"reason,omitempty"`
}
