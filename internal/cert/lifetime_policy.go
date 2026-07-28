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

func certificateRenewalTime(info *Info, renewalIntervalDays int) time.Time {
	if info == nil {
		return time.Time{}
	}

	validity := info.NotAfter.Sub(info.NotBefore)
	if validity <= 0 {
		return time.Time{}
	}

	if isShortLivedCertificate(info) {
		// This matches lego's short-lived fallback and leaves half of the
		// certificate lifetime available for retries if ARI is unavailable.
		return info.NotBefore.Add(validity / 2)
	}

	return info.NotBefore.Add(time.Duration(renewalIntervalDays) * 24 * time.Hour)
}

func shouldRenewCertificate(info *Info, now time.Time, renewalIntervalDays int) bool {
	renewAt := certificateRenewalTime(info, renewalIntervalDays)
	return !renewAt.IsZero() && !now.Before(renewAt)
}
