package model

import "time"

// TLS modes of an ingress.
const (
	// IngressTLSModeNone serves the ingress over plain HTTP only.
	IngressTLSModeNone = "none"
	// IngressTLSModeACME issues and renews a certificate for the hosts.
	IngressTLSModeACME = "acme"
	// IngressTLSModeCert uses an existing certificate referenced by CertID.
	IngressTLSModeCert = "cert"
)

// ACME challenge types of an ingress.
const (
	IngressChallengeHTTP01 = "http01"
	IngressChallengeDNS01  = "dns01"
)

// Load balancing methods of an ingress backend.
const (
	IngressLBRoundRobin = "round_robin"
	IngressLBLeastConn  = "least_conn"
	IngressLBIPHash     = "ip_hash"
)

// Phases summarize an ingress status.
const (
	// IngressPhasePending means the current generation has not been applied yet.
	IngressPhasePending = "Pending"
	// IngressPhaseApplying means the apply worker is writing the config.
	IngressPhaseApplying = "Applying"
	// IngressPhaseReady means the config is live, the certificate (if any) is
	// in place and at least one server of every backend is healthy.
	IngressPhaseReady = "Ready"
	// IngressPhaseDegraded means the config is live but the certificate is not
	// ready yet or some backends have no healthy server.
	IngressPhaseDegraded = "Degraded"
	// IngressPhaseError means the last apply failed and the previous config
	// is still live.
	IngressPhaseError = "Error"
)

// Condition types of an ingress status.
const (
	IngressConditionConfigApplied   = "ConfigApplied"
	IngressConditionCertReady       = "CertReady"
	IngressConditionBackendsHealthy = "BackendsHealthy"
)

// Ingress is a declarative reverse proxy entry owned by an external
// controller. Nginx UI renders it into a site and one managed upstream per
// backend, applies them with a single test and reload, and keeps them in line
// with the spec.
type Ingress struct {
	Model
	// Name is unique across owners and names the generated files.
	Name string `json:"name" gorm:"uniqueIndex;not null"`
	// Owner identifies the controller that manages this ingress. Requests
	// from another owner are refused unless they force the change.
	Owner string `json:"owner" gorm:"index;not null"`
	// ExternalID is the owner's own identifier, for display and lookups.
	ExternalID string            `json:"external_id"`
	Labels     map[string]string `json:"labels" gorm:"serializer:json"`
	Spec       IngressSpec       `json:"spec" gorm:"serializer:json"`
	// Generation increases every time Spec changes.
	Generation int64         `json:"generation"`
	Status     IngressStatus `json:"status" gorm:"serializer:json"`
	// SitePath is the generated site file name under sites-available.
	SitePath string `json:"site_path"`
	// NamespaceID optionally places the generated site in a namespace so the
	// existing cluster sync replicates it.
	NamespaceID uint64 `json:"namespace_id"`
}

// IngressSpec is the desired state of an ingress.
type IngressSpec struct {
	Hosts    []string         `json:"hosts"`
	Backends []IngressBackend `json:"backends"`
	Routes   []IngressRoute   `json:"routes"`
	TLS      IngressTLS       `json:"tls"`
	Policy   IngressPolicy    `json:"policy"`
}

// IngressBackend becomes one managed upstream group.
type IngressBackend struct {
	Name    string          `json:"name"`
	LB      string          `json:"lb,omitempty"`
	Servers []IngressServer `json:"servers"`
	// Keepalive is the number of idle upstream connections kept per worker.
	Keepalive int `json:"keepalive,omitempty"`
}

// IngressServer is one server of a backend.
type IngressServer struct {
	// Address is host:port. It must fall inside the allowed backend ranges.
	Address     string `json:"address"`
	Weight      int    `json:"weight,omitempty"`
	MaxFails    int    `json:"max_fails,omitempty"`
	FailTimeout string `json:"fail_timeout,omitempty"`
	Down        bool   `json:"down"`
	Backup      bool   `json:"backup"`
}

// IngressRoute maps a path prefix to a backend.
type IngressRoute struct {
	PathPrefix string `json:"path_prefix"`
	Backend    string `json:"backend"`
	WebSocket  bool   `json:"websocket"`
	// Timeouts are nginx time values such as 60s or 5m.
	ReadTimeout string `json:"read_timeout,omitempty"`
	SendTimeout string `json:"send_timeout,omitempty"`
	// ClientMaxBodySize overrides Policy.ClientMaxBodySize, e.g. 64m.
	ClientMaxBodySize string `json:"client_max_body_size,omitempty"`
	// AllowCIDRs narrows Policy.AllowCIDRs for this route.
	AllowCIDRs []string `json:"allow_cidrs,omitempty"`
}

// IngressTLS selects where the certificate comes from.
type IngressTLS struct {
	Mode            string `json:"mode"`
	CertID          uint64 `json:"cert_id,omitempty"`
	KeyType         string `json:"key_type,omitempty"`
	Challenge       string `json:"challenge,omitempty"`
	DNSCredentialID uint64 `json:"dns_credential_id,omitempty"`
	ACMEUserID      uint64 `json:"acme_user_id,omitempty"`
	RedirectHTTPS   bool   `json:"redirect_https"`
	HSTS            bool   `json:"hsts"`
}

// IngressPolicy holds request policy that applies to every route.
type IngressPolicy struct {
	AllowCIDRs        []string           `json:"allow_cidrs,omitempty"`
	RateLimit         *IngressRateLimit  `json:"rate_limit,omitempty"`
	ClientMaxBodySize string             `json:"client_max_body_size,omitempty"`
	ExtraHeaders      []IngressHeaderSet `json:"extra_headers,omitempty"`
}

// IngressRateLimit limits requests per client address.
type IngressRateLimit struct {
	RPS   int `json:"rps"`
	Burst int `json:"burst"`
}

// IngressHeaderSet adds a request header sent to the backend.
type IngressHeaderSet struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// IngressStatus is the observed state of an ingress.
type IngressStatus struct {
	ObservedGeneration int64              `json:"observed_generation"`
	Phase              string             `json:"phase"`
	Conditions         []IngressCondition `json:"conditions"`
	// ConfigHash is the sha256 of the rendered site and upstream files.
	ConfigHash    string                `json:"config_hash,omitempty"`
	CertID        uint64                `json:"cert_id,omitempty"`
	CertNotAfter  *time.Time            `json:"cert_not_after,omitempty"`
	LastAppliedAt *time.Time            `json:"last_applied_at,omitempty"`
	Servers       []IngressServerStatus `json:"servers,omitempty"`
}

// IngressCondition is one aspect of an ingress status.
type IngressCondition struct {
	Type               string    `json:"type"`
	Status             bool      `json:"status"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
	LastTransitionTime time.Time `json:"last_transition_time"`
}

// IngressServerStatus is the health of one backend server as seen by the
// upstream availability checker.
type IngressServerStatus struct {
	Backend string `json:"backend"`
	Address string `json:"address"`
	Down    bool   `json:"down"`
	Healthy bool   `json:"healthy"`
}
