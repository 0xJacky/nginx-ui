package model

import "time"

// Outcomes of a certificate deployment.
const (
	CertDeploymentOK     = "ok"
	CertDeploymentFailed = "failed"
)

// CertDeployTargetKindPluginPrefix marks a deploy target kind a plugin
// provides.
const CertDeployTargetKindPluginPrefix = "plugin:"

// CertDeployTarget is an external place issued certificates are pushed to,
// such as a CDN or a load balancer, served by a cert.deploy plugin.
type CertDeployTarget struct {
	Model
	Name string `json:"name" cosy:"add:required;update:omitempty" gorm:"not null"`
	// Kind is "plugin:<code>", the target kind of a cert.deploy plugin.
	Kind string `json:"kind" cosy:"add:required;update:omitempty" gorm:"index;not null"`
	// Config holds the values of the target form. They are credentials, so
	// they are encrypted at rest.
	Config map[string]string `json:"config" cosy:"add:omitempty;update:omitempty" gorm:"serializer:json[aes]"`
	// CertID binds the target to one certificate, 0 binds every certificate.
	CertID uint64 `json:"cert_id" cosy:"add:omitempty;update:omitempty" gorm:"index"`
	// Enabled has no column default, so a target created disabled stays so.
	Enabled bool `json:"enabled" cosy:"add:omitempty;update:omitempty" gorm:"index"`
}

// Matches reports whether the target is bound to a certificate.
func (t *CertDeployTarget) Matches(certID uint64) bool {
	return t.CertID == 0 || t.CertID == certID
}

// CertDeployment records the outcome of pushing one certificate to one
// target, after every attempt the run made.
type CertDeployment struct {
	ID       uint64 `gorm:"primary_key" json:"id"`
	TargetID uint64 `json:"target_id" gorm:"index:idx_cert_deployments_target_cert"`
	CertID   uint64 `json:"cert_id" gorm:"index:idx_cert_deployments_target_cert"`
	// Status is CertDeploymentOK or CertDeploymentFailed.
	Status string `json:"status"`
	// Message is what the plugin answered, or the error of the last attempt.
	Message   string    `json:"message"`
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}
