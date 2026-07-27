package cert

import (
	"net/netip"
	"strings"
)

const shortLivedCertificateProfile = "shortlived"

type certificateIdentifierInfo struct {
	Values      []string
	HasIP       bool
	HasWildcard bool
}

// CertificateName returns the first certificate identifier for display and
// falls back to the associated configuration name when none is available.
func CertificateName(fallback string, identifiers []string) string {
	for _, identifier := range identifiers {
		if name := strings.TrimSpace(identifier); name != "" {
			return name
		}
	}
	return fallback
}

// NormalizeAndValidateIdentifiers canonicalizes certificate identifiers and
// enforces challenge combinations supported by the ACME protocol.
func NormalizeAndValidateIdentifiers(payload *ConfigPayload) error {
	info, err := normalizeCertificateIdentifiers(payload.ServerName)
	if err != nil {
		return err
	}

	if info.HasIP && info.HasWildcard {
		return ErrWildcardIPCertificateConflict
	}

	if payload.ChallengeMethod == "" {
		payload.ChallengeMethod = HTTP01
	}
	if info.HasIP && payload.ChallengeMethod != HTTP01 {
		return ErrIPCertificateRequiresHTTP01
	}

	payload.ServerName = info.Values
	return nil
}

func normalizeCertificateIdentifiers(values []string) (certificateIdentifierInfo, error) {
	info := certificateIdentifierInfo{
		Values: make([]string, 0, len(values)),
	}
	seen := make(map[string]struct{}, len(values))

	for _, raw := range values {
		identifier := strings.TrimSpace(raw)
		if identifier == "" {
			continue
		}
		if identifier == "_" {
			return certificateIdentifierInfo{}, NewInvalidCertificateIdentifierError(identifier)
		}

		canonical := identifier
		ipCandidate := identifier
		if strings.HasPrefix(ipCandidate, "[") && strings.HasSuffix(ipCandidate, "]") {
			ipCandidate = strings.TrimSuffix(strings.TrimPrefix(ipCandidate, "["), "]")
		}
		if addr, err := netip.ParseAddr(ipCandidate); err == nil {
			if addr.Zone() != "" {
				return certificateIdentifierInfo{}, NewInvalidCertificateIdentifierError(identifier)
			}
			canonical = addr.String()
			info.HasIP = true
		} else if strings.HasPrefix(identifier, "*.") {
			info.HasWildcard = true
		}

		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		info.Values = append(info.Values, canonical)
	}

	if len(info.Values) == 0 {
		return certificateIdentifierInfo{}, NewInvalidCertificateIdentifierError("")
	}

	return info, nil
}

func resolveCertificateProfile(payload *ConfigPayload, advertisedProfiles map[string]string) error {
	payload.Profile = strings.TrimSpace(payload.Profile)
	if payload.Profile == "" && payload.Resource != nil && payload.Resource.Resource != nil {
		payload.Profile = strings.TrimSpace(payload.Resource.Profile)
	}

	if payload.Profile != "" {
		if len(advertisedProfiles) > 0 {
			if _, ok := advertisedProfiles[payload.Profile]; !ok {
				return NewCertificateProfileUnavailableError(payload.Profile)
			}
		}
		return nil
	}

	info, err := normalizeCertificateIdentifiers(payload.ServerName)
	if err != nil {
		return err
	}
	if !info.HasIP {
		return nil
	}

	if _, ok := advertisedProfiles[shortLivedCertificateProfile]; ok {
		payload.Profile = shortLivedCertificateProfile
		return nil
	}
	if len(advertisedProfiles) > 0 {
		return NewCertificateProfileUnavailableError(shortLivedCertificateProfile)
	}

	// Some private ACME servers support IP identifiers without implementing the
	// profiles extension. Keep the profile empty and let the server decide.
	return nil
}
