package cert

import "time"

const shortLivedCertificateMaxValidity = 10 * 24 * time.Hour

func isShortLivedCertificate(info *Info) bool {
	if info == nil {
		return false
	}

	validity := info.NotAfter.Sub(info.NotBefore)
	return validity > 0 && validity <= shortLivedCertificateMaxValidity
}

func certificateRenewalTime(info *Info, renewalThresholdDays int) time.Time {
	if info == nil {
		return time.Time{}
	}

	validity := info.NotAfter.Sub(info.NotBefore)
	if validity <= 0 || renewalThresholdDays <= 0 {
		return time.Time{}
	}

	renewalThreshold := time.Duration(renewalThresholdDays) * 24 * time.Hour
	if isShortLivedCertificate(info) || validity <= renewalThreshold {
		// This matches lego's short-lived fallback and leaves half of the
		// certificate lifetime available for retries. It also prevents an
		// oversized threshold from making a new certificate immediately due.
		return info.NotBefore.Add(validity / 2)
	}

	return info.NotAfter.Add(-renewalThreshold)
}

func shouldRenewCertificate(info *Info, now time.Time, renewalThresholdDays int) bool {
	renewAt := certificateRenewalTime(info, renewalThresholdDays)
	return !renewAt.IsZero() && !now.Before(renewAt)
}
