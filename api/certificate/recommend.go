package certificate

import (
	"net/http"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// RecommendCertRequest is the body of POST cert_recommendation.
type RecommendCertRequest struct {
	Domains []string `json:"domains"`
}

// RecommendCertResponse carries the recommended certificate, or null when no
// certificate of the manager covers every domain.
type RecommendCertResponse struct {
	Certificate *APICertificate `json:"certificate"`
}

// RecommendCert returns the certificate of the certificate manager that fits
// the requested domains best, so the HTTPS onboarding can preselect it.
func RecommendCert(c *gin.Context) {
	var req RecommendCertRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	identifiers, ok := recommendationIdentifiers(req.Domains)
	if !ok {
		// An invalid domain is reported by the HTTPS pre-flight check; no
		// certificate can cover it.
		c.JSON(http.StatusOK, RecommendCertResponse{})
		return
	}

	record, err := cert.RecommendCertificate(identifiers, time.Now())
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if record == nil {
		c.JSON(http.StatusOK, RecommendCertResponse{})
		return
	}

	info, _ := cert.GetCertInfo(record.SSLCertificatePath)
	c.JSON(http.StatusOK, RecommendCertResponse{Certificate: &APICertificate{
		Cert:             record,
		CertificateInfo:  info,
		DeploymentStatus: site.InspectCertificateDeployment(record),
	}})
}

// recommendationIdentifiers canonicalizes each domain on its own, the way an
// HTTPS onboarding run with an existing certificate does. It reports false
// when a domain is invalid or none is given.
func recommendationIdentifiers(domains []string) ([]string, bool) {
	var identifiers []string
	for _, domain := range domains {
		if strings.TrimSpace(domain) == "" {
			continue
		}
		payload := &cert.ConfigPayload{ServerName: []string{domain}, ChallengeMethod: cert.HTTP01}
		if err := cert.NormalizeAndValidateIdentifiers(payload); err != nil {
			return nil, false
		}
		identifiers = append(identifiers, payload.ServerName...)
	}
	return identifiers, len(identifiers) > 0
}
