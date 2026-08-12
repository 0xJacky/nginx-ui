package sitecheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"syscall"
)

// Probe failure categories reported in SiteInfo.ErrorType.
//
// The raw Go error is still returned in SiteInfo.Error, but it is a developer
// string ("tls: failed to verify certificate: x509: certificate signed by
// unknown authority"). The category gives the frontend a stable, translatable
// handle so an operator can tell "the site never answered" apart from "the
// site answered but its certificate is not trusted" without guessing (#1790).
const (
	// ErrorTypeTLS means the TCP connection succeeded but the TLS handshake
	// or certificate validation failed.
	ErrorTypeTLS = "tls"
	// ErrorTypeDNS means the hostname could not be resolved.
	ErrorTypeDNS = "dns"
	// ErrorTypeConnectionRefused means nothing is listening on the target port.
	ErrorTypeConnectionRefused = "connection_refused"
	// ErrorTypeTimeout means the target did not answer in time.
	ErrorTypeTimeout = "timeout"
	// ErrorTypeNetwork is the catch-all for other transport failures.
	ErrorTypeNetwork = "network"
	// ErrorTypeStatusCode means the site answered with an unexpected status.
	ErrorTypeStatusCode = "status_code"
	// ErrorTypeContent means the response body failed the configured
	// expected/not-expected text validation.
	ErrorTypeContent = "content"
	// ErrorTypeRequest means the probe request itself could not be built,
	// which points at an invalid health check configuration.
	ErrorTypeRequest = "request"
)

// classifyCheckError maps a transport-level probe failure to a stable category.
// It returns an empty string for a nil error.
func classifyCheckError(err error) string {
	if err == nil {
		return ""
	}

	if isTLSError(err) {
		return ErrorTypeTLS
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ErrorTypeDNS
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return ErrorTypeConnectionRefused
	}

	if isTimeoutError(err) {
		return ErrorTypeTimeout
	}

	return ErrorTypeNetwork
}

// isTLSError reports whether the failure came from the TLS layer, covering
// both the handshake itself and certificate chain/hostname validation.
func isTLSError(err error) bool {
	var verificationErr *tls.CertificateVerificationError
	if errors.As(err, &verificationErr) {
		return true
	}

	var unknownAuthorityErr x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthorityErr) {
		return true
	}

	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		return true
	}

	var invalidCertErr x509.CertificateInvalidError
	if errors.As(err, &invalidCertErr) {
		return true
	}

	// Sent when a plaintext response arrives on a port probed as HTTPS.
	var recordHeaderErr tls.RecordHeaderError
	if errors.As(err, &recordHeaderErr) {
		return true
	}

	var alertErr tls.AlertError
	return errors.As(err, &alertErr)
}

// isTimeoutError reports whether the failure is a deadline or timeout, whether
// it came from the per-request context or from the network stack.
func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
