// Package ingress implements declarative reverse proxy entries for external
// controllers.
//
// An ingress (model.Ingress) is rendered into one site file
// (sites-available/ingress-<name>) and one managed upstream per backend
// (conf.d/upstream-ig-<name>-<backend>.conf). Changes go through a queue: the
// API stores the spec and bumps the generation, and a single worker renders,
// writes and applies every pending ingress with one nginx test and reload,
// rolling all files back when the test fails. Certificates are handled by a
// CertManager, and a reconcile loop rewrites generated files that drifted.
//
// File ownership while the feature is being built in parallel:
//   - ingress.go, errors.go, start.go: contract, change only by agreement
//   - render.go, apply.go, queue.go, store.go: core (render, apply, API logic)
//   - cert.go: certificate manager
//   - reconcile.go: drift reconcile loop
package ingress

import (
	"context"
	"regexp"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
)

var (
	reName    = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)
	reOwner   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	reBackend = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,14}[a-z0-9])?$`)
)

// ValidateName reports whether name can be used as an ingress name.
func ValidateName(name string) error {
	if !reName.MatchString(name) {
		return ErrInvalidName
	}
	return nil
}

// ValidateOwner reports whether owner can be used as an ingress owner.
func ValidateOwner(owner string) error {
	if !reOwner.MatchString(owner) {
		return ErrInvalidOwner
	}
	return nil
}

// ValidateBackendName reports whether name can be used as a backend name.
func ValidateBackendName(name string) bool {
	return reBackend.MatchString(name)
}

// SiteName returns the site file name generated for an ingress.
func SiteName(name string) string {
	return "ingress-" + name
}

// ManagedBy returns the model.Site.ManagedBy value of an ingress site.
func ManagedBy(name string) string {
	return "ingress:" + name
}

// UpstreamName returns the managed upstream name generated for a backend.
func UpstreamName(name, backend string) string {
	return "ig-" + name + "-" + backend
}

// RenderOptions carries the state the renderer cannot derive from the spec.
type RenderOptions struct {
	// Cert is the certificate to serve. When Cert.Ready is false only the
	// HTTP listener (with the ACME challenge location) is rendered.
	Cert CertState
}

// Rendered is the output of rendering one ingress.
type Rendered struct {
	// SiteName is the file name under sites-available.
	SiteName    string `json:"site_name"`
	SiteContent string `json:"site_content"`
	// Upstreams maps managed upstream names to their file content.
	Upstreams map[string]string `json:"upstreams"`
	// Hash is the sha256 over the site and upstream contents in a fixed
	// order, stored as IngressStatus.ConfigHash.
	Hash string `json:"hash"`
}

// CertState is the certificate an ingress can serve right now.
type CertState struct {
	Ready    bool       `json:"ready"`
	CertID   uint64     `json:"cert_id,omitempty"`
	CertPath string     `json:"cert_path,omitempty"`
	KeyPath  string     `json:"key_path,omitempty"`
	NotAfter *time.Time `json:"not_after,omitempty"`
	// Reason and Message explain why the certificate is not ready, e.g.
	// DNSNotReady, Issuing, IssueFailed, CertNotFound.
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// CertManager provides certificates for ingresses.
type CertManager interface {
	// Resolve returns the certificate the ingress can serve now, without
	// side effects. It is called before every render.
	Resolve(ctx context.Context, ing *model.Ingress) (CertState, error)
	// Ensure starts issuing a certificate when the ingress needs one and
	// none is ready. It must not block on issuance: issuance runs in the
	// background and calls Enqueue(ing.Name) once the state changed, so the
	// worker renders the HTTPS listener. It is called after an apply that
	// rendered the HTTP listener for an ACME ingress.
	Ensure(ctx context.Context, ing *model.Ingress)
	// Forget drops background work and backoff for a deleted ingress.
	Forget(name string)
}

var (
	certManagerMu sync.RWMutex
	certManager   CertManager = noCerts{}
)

// SetCertManager installs the certificate manager.
func SetCertManager(m CertManager) {
	certManagerMu.Lock()
	defer certManagerMu.Unlock()
	certManager = m
}

func certs() CertManager {
	certManagerMu.RLock()
	defer certManagerMu.RUnlock()
	return certManager
}

// noCerts never has a certificate; it is used until a manager is installed.
type noCerts struct{}

func (noCerts) Resolve(context.Context, *model.Ingress) (CertState, error) {
	return CertState{Reason: "NoCertManager"}, nil
}

func (noCerts) Ensure(context.Context, *model.Ingress) {}

func (noCerts) Forget(string) {}

// ApplyOptions controls Apply.
type ApplyOptions struct {
	// DryRun renders and tests against a scratch copy of the config without
	// touching the live files or reloading.
	DryRun bool
}

// ApplyResult is the outcome of applying one ingress.
type ApplyResult struct {
	Name     string    `json:"name"`
	Rendered *Rendered `json:"rendered,omitempty"`
	// Changed is false when the rendered files already matched.
	Changed bool `json:"changed"`
	// TestOutput is the nginx -t output when the test failed.
	TestOutput string `json:"test_output,omitempty"`
	Error      string `json:"error,omitempty"`
}

// SetCondition sets a condition on status, keeping LastTransitionTime when
// the condition status did not change.
func SetCondition(status *model.IngressStatus, c model.IngressCondition) {
	for i := range status.Conditions {
		if status.Conditions[i].Type != c.Type {
			continue
		}
		if status.Conditions[i].Status == c.Status {
			c.LastTransitionTime = status.Conditions[i].LastTransitionTime
		}
		if c.LastTransitionTime.IsZero() {
			c.LastTransitionTime = time.Now()
		}
		status.Conditions[i] = c
		return
	}
	if c.LastTransitionTime.IsZero() {
		c.LastTransitionTime = time.Now()
	}
	status.Conditions = append(status.Conditions, c)
}

// FindCondition returns the condition of the given type, if any.
func FindCondition(status *model.IngressStatus, conditionType string) (model.IngressCondition, bool) {
	for _, c := range status.Conditions {
		if c.Type == conditionType {
			return c, true
		}
	}
	return model.IngressCondition{}, false
}
