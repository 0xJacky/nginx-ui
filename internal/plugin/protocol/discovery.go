package protocol

// DiscoveryResolveParams is the payload of discovery.resolve.
type DiscoveryResolveParams struct {
	// Provider is the provider code declared in the manifest, without any host prefix.
	Provider string `json:"provider"`
	// Config holds the field values the user filled in, keyed by field key.
	Config map[string]string `json:"config"`
	// Service names the service to resolve, in the terms of the provider.
	Service string `json:"service"`
}

// DiscoveryResolveResult is the reply to discovery.resolve.
type DiscoveryResolveResult struct {
	// Targets lists every server that backs the service right now.
	Targets []DiscoveryTarget `json:"targets"`
	// TTLSeconds is how long the answer stays fresh. 0 leaves it to the host.
	TTLSeconds int `json:"ttl_seconds,omitempty"`
}

// DiscoveryTarget is one server of a service.
type DiscoveryTarget struct {
	// Address is an IPv4 or IPv6 address or a host name, without a port.
	Address string `json:"address"`
	// Port is the TCP port, 1 to 65535.
	Port int `json:"port"`
	// Weight is the relative weight of the server. 0 means 1.
	Weight int `json:"weight,omitempty"`
	// Tags are labels the provider attaches to the server, for display.
	Tags []string `json:"tags,omitempty"`
}
