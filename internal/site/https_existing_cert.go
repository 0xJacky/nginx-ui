package site

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/acmehint"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-acme/lego/v5/certcrypto"
	"gorm.io/gorm"
)

// Hint codes of an unusable existing certificate.
const (
	HTTPSHintCertificateNotFound            = "certificate_not_found"
	HTTPSHintCertificateFilesMissing        = "certificate_files_missing"
	HTTPSHintCertificateExpired             = "certificate_expired"
	HTTPSHintCertificateDoesNotCoverDomains = "certificate_does_not_cover_domains"
)

var (
	httpsMessageCertificateNotFound = translation.C(
		"The selected certificate does not exist any more; choose another certificate")
	httpsMessageCertificateFilesMissing = translation.C(
		"The certificate or key file of %{name} is missing or cannot be read; fix it in the certificate manager or choose another certificate")
	httpsMessageCertificateExpired = translation.C(
		"The certificate %{name} expired on %{not_after}; renew it or choose another certificate")
	httpsMessageCertificateDoesNotCoverDomains = translation.C(
		"The certificate %{name} does not cover any of the domains: %{uncovered}")
	httpsMessageDomainsNotInExistingCertificate = translation.C(
		"The certificate %{name} does not cover some of the domains; they are left out of HTTPS: %{uncovered}")
	httpsMessageExistingCertificateProbeSkipped = translation.C("An existing certificate is used")
)

// HTTPSExistingCertificate is a certificate from the certificate manager that
// an onboarding run installs instead of issuing a new one.
type HTTPSExistingCertificate struct {
	// ID is the id of the cert record.
	ID uint64
	// Name is the display name of the certificate.
	Name              string
	SSLCertificate    string
	SSLCertificateKey string
	KeyType           certcrypto.KeyType
	NotAfter          time.Time
	// Covers reports whether the certificate is valid for a normalized
	// identifier (host name, wildcard name or IP address).
	Covers func(identifier string) bool
}

// HTTPSCertificateReview is the outcome of checking an existing certificate
// against the requested domains.
type HTTPSCertificateReview struct {
	// Certificate is nil when the certificate could not be loaded.
	Certificate *HTTPSExistingCertificate
	// Covered and Uncovered split the requested domains.
	Covered   []string
	Uncovered []string
	// Err, when set, makes the certificate unusable; it carries a hint.
	Err error
}

// Params returns the certificate facts shared by the check and the hints:
// name, not_after (RFC 3339, UTC) and uncovered (space separated).
func (r *HTTPSCertificateReview) Params() map[string]string {
	params := map[string]string{}
	if r.Certificate != nil {
		params["name"] = r.Certificate.Name
		if !r.Certificate.NotAfter.IsZero() {
			params["not_after"] = r.Certificate.NotAfter.UTC().Format(time.RFC3339)
		}
	}
	params["uncovered"] = strings.Join(r.Uncovered, " ")
	return params
}

// Diagnostics returns the warning about requested domains the certificate
// does not cover, if any.
func (r *HTTPSCertificateReview) Diagnostics() []HTTPSDiagnostic {
	if r.Err != nil || len(r.Uncovered) == 0 {
		return nil
	}
	return []HTTPSDiagnostic{{
		Level:   acmehint.LevelWarning,
		Code:    HTTPSDiagnosticNamesNotInCertificate,
		Message: httpsMessageDomainsNotInExistingCertificate.Message,
		Params:  r.Params(),
	}}
}

// UsesExistingCertificate reports whether the run installs a certificate from
// the certificate manager instead of issuing one.
func (r HTTPSRequest) UsesExistingCertificate() bool {
	return r.CertificateID > 0
}

// reviewCertificate loads the selected certificate and checks it against the
// normalized domains.
func (o *HTTPSOnboarding) reviewCertificate(domains []string) *HTTPSCertificateReview {
	load := o.LoadCertificate
	if load == nil {
		load = LoadHTTPSExistingCertificate
	}

	review := &HTTPSCertificateReview{}
	existing, err := load(o.Request.CertificateID)
	if err != nil {
		review.Err = err
		return review
	}
	review.Certificate = existing

	for _, domain := range domains {
		if existing.Covers != nil && existing.Covers(domain) {
			review.Covered = append(review.Covered, domain)
		} else {
			review.Uncovered = append(review.Uncovered, domain)
		}
	}

	switch {
	case !existing.NotAfter.IsZero() && !time.Now().Before(existing.NotAfter):
		review.Err = &HTTPSHintError{
			Hint: HTTPSHint{
				Code:    HTTPSHintCertificateExpired,
				Message: httpsMessageCertificateExpired.Message,
				Params:  review.Params(),
			},
			Err: fmt.Errorf("certificate %q expired at %s", existing.Name,
				existing.NotAfter.UTC().Format(time.RFC3339)),
		}
	case len(review.Covered) == 0:
		review.Err = &HTTPSHintError{
			Hint: HTTPSHint{
				Code:    HTTPSHintCertificateDoesNotCoverDomains,
				Message: httpsMessageCertificateDoesNotCoverDomains.Message,
				Params:  review.Params(),
			},
			Err: fmt.Errorf("certificate %q does not cover %s", existing.Name, strings.Join(domains, ", ")),
		}
	}
	return review
}

// LoadHTTPSExistingCertificate loads a cert record and its certificate files.
// Errors carry the certificate_not_found or certificate_files_missing hint.
// It only reads the record; the auto-renew settings stay untouched.
func LoadHTTPSExistingCertificate(id uint64) (*HTTPSExistingCertificate, error) {
	c := query.Cert
	record, err := c.Where(c.ID.Eq(id)).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &HTTPSHintError{
			Hint: HTTPSHint{
				Code:    HTTPSHintCertificateNotFound,
				Message: httpsMessageCertificateNotFound.Message,
				Params:  map[string]string{"id": fmt.Sprint(id)},
			},
			Err: fmt.Errorf("certificate %d not found", id),
		}
	}
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(record.Name)
	if name == "" {
		name = cert.CertificateName(fmt.Sprintf("#%d", id), record.Domains)
	}
	existing := &HTTPSExistingCertificate{
		ID:                id,
		Name:              name,
		SSLCertificate:    strings.TrimSpace(record.SSLCertificatePath),
		SSLCertificateKey: strings.TrimSpace(record.SSLCertificateKeyPath),
		KeyType:           record.KeyType,
	}
	filesMissing := func(err error) error {
		return &HTTPSHintError{
			Hint: HTTPSHint{
				Code:    HTTPSHintCertificateFilesMissing,
				Message: httpsMessageCertificateFilesMissing.Message,
				Params:  map[string]string{"name": name},
			},
			Err: fmt.Errorf("certificate %q: %w", name, err),
		}
	}
	if existing.SSLCertificate == "" || existing.SSLCertificateKey == "" {
		return nil, filesMissing(errors.New("the certificate record has no certificate or key path"))
	}

	pair, err := cert.LoadCertificatePair(existing.SSLCertificate, existing.SSLCertificateKey)
	if err != nil {
		return nil, filesMissing(err)
	}
	if existing.KeyType == "" {
		existing.KeyType = certcrypto.KeyType(pair.KeyType)
	}
	existing.NotAfter = pair.Leaf.NotAfter
	leaf := pair.Leaf
	existing.Covers = func(identifier string) bool {
		return cert.CertificateCoversIdentifier(leaf, identifier)
	}
	return existing, nil
}

// normalizeDomains runs Normalize on the requested identifiers. The ACME
// rules it enforces for a challenge method (IP identifiers need HTTP-01, no
// wildcard next to an IP) do not apply to an existing certificate, so in that
// mode every identifier is validated on its own.
func (o *HTTPSOnboarding) normalizeDomains() ([]string, error) {
	if o.Normalize == nil {
		return o.Request.Domains, nil
	}
	if !o.Request.UsesExistingCertificate() {
		return o.Normalize(o.Request.Domains, o.Request.ChallengeMethod)
	}
	var domains []string
	for _, domain := range o.Request.Domains {
		if strings.TrimSpace(domain) == "" {
			continue
		}
		normalized, err := o.Normalize([]string{domain}, HTTPSChallengeHTTP01)
		if err != nil {
			return nil, err
		}
		domains = append(domains, normalized...)
	}
	return domains, nil
}

// existingCertificateResult is the done payload of a run that installs an
// existing certificate.
func existingCertificateResult(existing *HTTPSExistingCertificate) HTTPSIssueResult {
	return HTTPSIssueResult{
		SSLCertificate:    existing.SSLCertificate,
		SSLCertificateKey: existing.SSLCertificateKey,
		KeyType:           existing.KeyType,
		CertID:            existing.ID,
	}
}
