package cert

import (
	"bytes"
	"crypto/x509"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
)

// RecommendationCandidate is a certificate from the certificate manager that
// RecommendCertificate ranks.
type RecommendationCandidate struct {
	ID   uint64
	Leaf *x509.Certificate
}

// PickRecommendedCertificate returns the index of the candidate that fits the
// normalized identifiers best, or -1 when no candidate covers every one of
// them or when there are no identifiers. Expired certificates are never
// picked. Coverage uses CertificateCoversIdentifier, the rule an HTTPS
// onboarding run applies to an existing certificate. Among the candidates:
//
//   - a CA-issued certificate beats a self-signed one, which browsers do not
//     trust;
//   - then the most identifiers matched by an exact SAN wins, so
//     `www.example.com` beats `*.example.com`;
//   - then the fewest SANs, so a lone wildcard beats a certificate bundling
//     unrelated sites;
//   - then the latest expiry, then the lowest id.
func PickRecommendedCertificate(candidates []RecommendationCandidate, identifiers []string, now time.Time) int {
	if len(identifiers) == 0 {
		return -1
	}

	type ranked struct {
		index        int
		selfSigned   bool
		exactMatches int
		nameCount    int
	}

	var fits []ranked
	for i, candidate := range candidates {
		leaf := candidate.Leaf
		if leaf == nil || !now.Before(leaf.NotAfter) {
			continue
		}

		exactMatches := 0
		coversAll := true
		for _, identifier := range identifiers {
			if !CertificateCoversIdentifier(leaf, identifier) {
				coversAll = false
				break
			}
			if certificateHasExactName(leaf, identifier) {
				exactMatches++
			}
		}
		if !coversAll {
			continue
		}

		fits = append(fits, ranked{
			index:        i,
			selfSigned:   bytes.Equal(leaf.RawIssuer, leaf.RawSubject),
			exactMatches: exactMatches,
			nameCount:    len(leaf.DNSNames) + len(leaf.IPAddresses),
		})
	}
	if len(fits) == 0 {
		return -1
	}

	sort.SliceStable(fits, func(i, j int) bool {
		a, b := fits[i], fits[j]
		if a.selfSigned != b.selfSigned {
			return !a.selfSigned
		}
		if a.exactMatches != b.exactMatches {
			return a.exactMatches > b.exactMatches
		}
		if a.nameCount != b.nameCount {
			return a.nameCount < b.nameCount
		}
		aLeaf, bLeaf := candidates[a.index].Leaf, candidates[b.index].Leaf
		if !aLeaf.NotAfter.Equal(bLeaf.NotAfter) {
			return aLeaf.NotAfter.After(bLeaf.NotAfter)
		}
		return candidates[a.index].ID < candidates[b.index].ID
	})
	return fits[0].index
}

// certificateHasExactName reports whether leaf lists identifier itself as a
// SAN, as opposed to covering it through a wildcard.
func certificateHasExactName(leaf *x509.Certificate, identifier string) bool {
	identifier = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(identifier)), ".")
	ipCandidate := strings.TrimSuffix(strings.TrimPrefix(identifier, "["), "]")
	if addr, err := netip.ParseAddr(ipCandidate); err == nil {
		addr = addr.Unmap()
		for _, ip := range leaf.IPAddresses {
			if candidate, ok := netip.AddrFromSlice(ip); ok && candidate.Unmap() == addr {
				return true
			}
		}
		return false
	}

	for _, name := range leaf.DNSNames {
		if strings.EqualFold(strings.TrimSuffix(name, "."), identifier) {
			return true
		}
	}
	return false
}

// RecommendCertificate returns the certificate record of the certificate
// manager that fits the identifiers best (see PickRecommendedCertificate), or
// nil when none covers all of them. Only records whose certificate and key
// files load as a matching pair are considered, like an HTTPS onboarding run
// requires.
func RecommendCertificate(identifiers []string, now time.Time) (*model.Cert, error) {
	if len(identifiers) == 0 {
		return nil, nil
	}

	// Only the paths are needed to rank; loading every column would decrypt
	// every stored ACME resource.
	c := query.Cert
	records, err := c.Select(c.ID, c.SSLCertificatePath, c.SSLCertificateKeyPath).Find()
	if err != nil {
		return nil, err
	}

	candidates := make([]RecommendationCandidate, 0, len(records))
	for _, record := range records {
		pair, err := LoadCertificatePair(record.SSLCertificatePath, record.SSLCertificateKeyPath)
		if err != nil {
			continue
		}
		candidates = append(candidates, RecommendationCandidate{ID: record.ID, Leaf: pair.Leaf})
	}

	index := PickRecommendedCertificate(candidates, identifiers, now)
	if index < 0 {
		return nil, nil
	}
	return c.Where(c.ID.Eq(candidates[index].ID)).First()
}
