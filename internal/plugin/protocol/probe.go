package protocol

// ProbeCheckParams is the payload of probe.check.
type ProbeCheckParams struct {
	// Kind is the probe kind code declared in the manifest, without any host prefix.
	Kind string `json:"kind"`
	// Target is what to probe, usually an absolute URL.
	Target string `json:"target"`
	// Config holds the field values the user filled in, keyed by field key.
	Config map[string]string `json:"config"`
	// TimeoutSeconds is how long the host waits for the answer.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

// ProbeCheckResult is the reply to probe.check.
type ProbeCheckResult struct {
	// Status is one of the ProbeStatus values.
	Status string `json:"status"`
	// LatencyMS is the time the check took, in milliseconds.
	LatencyMS int `json:"latency_ms,omitempty"`
	// Message is human readable detail, expected when Status is not up.
	Message string `json:"message,omitempty"`
}

// Values of ProbeCheckResult.Status.
const (
	ProbeStatusUp       = "up"
	ProbeStatusDown     = "down"
	ProbeStatusDegraded = "degraded"
)
