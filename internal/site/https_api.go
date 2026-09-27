package site

import "github.com/0xJacky/Nginx-UI/internal/nginx"

// HTTPSPreview is the side-effect free outcome of the plan step. The HTTPS
// pre-flight check endpoint uses it to report what the onboarding would do.
type HTTPSPreview struct {
	// Plan is the plan the onboarding would apply.
	Plan *HTTPSPlan
	// Status is the current status of the site (enabled or disabled; a site
	// in maintenance fails the preview like it fails the plan step).
	Status Status
	// Live is the parsed configuration currently stored in sites-available.
	Live *nginx.NgxConfig
	// Certificate is the review of the certificate selected by
	// Request.CertificateID; nil when the request issues a certificate. An
	// unusable certificate does not fail the preview: the site is then
	// planned for every requested domain so the configuration is still
	// checked, and Certificate.Err explains the certificate problem.
	Certificate *HTTPSCertificateReview
}

// Preview runs the plan step of the onboarding (site lookup, remote-deploy,
// advanced-editor and maintenance checks, parsing, Normalize, the existing
// certificate review and PlanHTTPS) without writing any file or touching
// Nginx. Errors are the same the plan step would report, including
// *HTTPSHintError values, except for the certificate problems, which are
// reported in HTTPSPreview.Certificate.
func (o *HTTPSOnboarding) Preview() (*HTTPSPreview, error) {
	siteState, err := o.inspectSite()
	if err != nil {
		return nil, err
	}

	domains := siteState.domains
	var review *HTTPSCertificateReview
	if o.Request.UsesExistingCertificate() {
		review = o.reviewCertificate(domains)
		if review.Err == nil {
			domains = review.Covered
		}
	}
	state, err := o.plan(siteState, domains, review)
	if err != nil {
		return nil, err
	}

	live, err := nginx.ParseNgxConfigByContent(string(state.originalRaw))
	if err != nil {
		return nil, advancedConfigError(err)
	}

	status := StatusDisabled
	if state.wasEnabled {
		status = StatusEnabled
	}
	return &HTTPSPreview{Plan: state.plan, Status: status, Live: live, Certificate: review}, nil
}
