package settings

import "time"

const (
	defaultIngressReconcileIntervalMinutes = 5
	minIngressReconcileIntervalMinutes     = 1
)

// Ingress configures the declarative ingress controller.
type Ingress struct {
	// AllowedBackends lists the CIDRs ingress backend servers may point to.
	// An ingress is a remote-controlled proxy_pass, so this keeps a bad or
	// compromised controller from aiming nginx at arbitrary hosts.
	AllowedBackends []string `json:"allowed_backends" binding:"omitempty,dive,cidr" ini:",,allowshadow" protected:"true"`
	// ReconcileIntervalMinutes is how often generated files are compared with
	// the spec and rewritten when they drifted.
	ReconcileIntervalMinutes int `json:"reconcile_interval_minutes" binding:"omitempty,min=1"`
}

var IngressSettings = &Ingress{
	AllowedBackends:          []string{"127.0.0.1/32", "::1/128"},
	ReconcileIntervalMinutes: defaultIngressReconcileIntervalMinutes,
}

// GetReconcileInterval returns the drift reconcile interval.
func (s *Ingress) GetReconcileInterval() time.Duration {
	minutes := s.ReconcileIntervalMinutes
	if minutes < minIngressReconcileIntervalMinutes {
		minutes = defaultIngressReconcileIntervalMinutes
	}
	return time.Duration(minutes) * time.Minute
}
