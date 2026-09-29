package cert

import (
	"crypto/x509"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recommendationLeaf(notAfter time.Time, caIssued bool, names ...string) *x509.Certificate {
	leaf := &x509.Certificate{NotAfter: notAfter, RawSubject: []byte("subject"), RawIssuer: []byte("subject")}
	if caIssued {
		leaf.RawIssuer = []byte("ca")
	}
	for _, name := range names {
		if ip := net.ParseIP(name); ip != nil {
			leaf.IPAddresses = append(leaf.IPAddresses, ip)
		} else {
			leaf.DNSNames = append(leaf.DNSNames, name)
		}
	}
	return leaf
}

func TestPickRecommendedCertificate(t *testing.T) {
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	later := now.AddDate(0, 3, 0)
	latest := now.AddDate(1, 0, 0)
	candidate := func(id uint64, leaf *x509.Certificate) RecommendationCandidate {
		return RecommendationCandidate{ID: id, Leaf: leaf}
	}

	t.Run("picks the wildcard of the site's domain", func(t *testing.T) {
		candidates := []RecommendationCandidate{
			candidate(1, recommendationLeaf(later, true, "*.def.cn")),
			candidate(2, recommendationLeaf(later, true, "*.abc.cn")),
		}
		assert.Equal(t, 1, PickRecommendedCertificate(candidates, []string{"www.abc.cn"}, now))
	})

	t.Run("nothing unless one certificate covers every identifier", func(t *testing.T) {
		candidates := []RecommendationCandidate{
			candidate(1, recommendationLeaf(later, true, "*.abc.cn")),
			candidate(2, recommendationLeaf(later, true, "*.def.cn")),
		}
		assert.Equal(t, -1, PickRecommendedCertificate(candidates, []string{"www.abc.cn", "www.def.cn"}, now))
		assert.Equal(t, -1, PickRecommendedCertificate(candidates, nil, now))
		// A wildcard covers one label only.
		assert.Equal(t, -1, PickRecommendedCertificate(candidates, []string{"abc.cn"}, now))
		assert.Equal(t, -1, PickRecommendedCertificate(candidates, []string{"a.b.abc.cn"}, now))
	})

	t.Run("skips expired and unloaded certificates", func(t *testing.T) {
		candidates := []RecommendationCandidate{
			candidate(1, recommendationLeaf(now, true, "*.abc.cn")),
			candidate(2, nil),
		}
		assert.Equal(t, -1, PickRecommendedCertificate(candidates, []string{"www.abc.cn"}, now))
	})

	t.Run("prefers a CA-issued certificate over a self-signed exact match", func(t *testing.T) {
		candidates := []RecommendationCandidate{
			candidate(1, recommendationLeaf(later, false, "www.abc.cn")),
			candidate(2, recommendationLeaf(later, true, "*.abc.cn")),
		}
		assert.Equal(t, 1, PickRecommendedCertificate(candidates, []string{"www.abc.cn"}, now))
	})

	t.Run("prefers an exact match, then fewer names, then the later expiry", func(t *testing.T) {
		assert.Equal(t, 1, PickRecommendedCertificate([]RecommendationCandidate{
			candidate(1, recommendationLeaf(latest, true, "*.abc.cn")),
			candidate(2, recommendationLeaf(later, true, "www.abc.cn")),
		}, []string{"www.abc.cn"}, now))
		assert.Equal(t, 1, PickRecommendedCertificate([]RecommendationCandidate{
			candidate(1, recommendationLeaf(latest, true, "*.abc.cn", "*.def.cn")),
			candidate(2, recommendationLeaf(later, true, "*.abc.cn")),
		}, []string{"www.abc.cn"}, now))
		assert.Equal(t, 1, PickRecommendedCertificate([]RecommendationCandidate{
			candidate(1, recommendationLeaf(later, true, "*.abc.cn")),
			candidate(2, recommendationLeaf(latest, true, "*.abc.cn")),
		}, []string{"www.abc.cn"}, now))
		assert.Equal(t, 0, PickRecommendedCertificate([]RecommendationCandidate{
			candidate(1, recommendationLeaf(later, true, "*.abc.cn")),
			candidate(2, recommendationLeaf(later, true, "*.abc.cn")),
		}, []string{"www.abc.cn"}, now))
	})

	t.Run("matches IP addresses and wildcard identifiers", func(t *testing.T) {
		candidates := []RecommendationCandidate{
			candidate(1, recommendationLeaf(later, true, "*.abc.cn")),
			candidate(2, recommendationLeaf(later, true, "*.abc.cn", "192.0.2.10")),
		}
		assert.Equal(t, 1, PickRecommendedCertificate(candidates, []string{"www.abc.cn", "192.0.2.10"}, now))
		assert.Equal(t, 0, PickRecommendedCertificate(candidates, []string{"*.abc.cn"}, now))
	})
}

func TestRecommendCertificate(t *testing.T) {
	confDir := useTempNginxConfDir(t)
	db := usePayloadTestDB(t)
	query.SetDefault(db)

	addCertificate := func(name string, dnsNames ...string) *model.Cert {
		t.Helper()
		certPEM, keyPEM, err := GenerateSelfSigned(SelfSignedOptions{DNSNames: dnsNames, KeyType: "P256"})
		require.NoError(t, err)
		certPath, keyPath := writePair(t, filepath.Join(confDir, "ssl", name), certPEM, keyPEM)
		record := &model.Cert{Name: name, Domains: dnsNames, SSLCertificatePath: certPath, SSLCertificateKeyPath: keyPath}
		require.NoError(t, db.Create(record).Error)
		return record
	}

	addCertificate("def", "*.def.cn")
	abc := addCertificate("abc", "*.abc.cn")
	// A record whose files are gone is never recommended.
	require.NoError(t, db.Create(&model.Cert{
		Name:                  "missing",
		Domains:               []string{"www.abc.cn"},
		SSLCertificatePath:    filepath.Join(confDir, "ssl", "missing", "fullchain.cer"),
		SSLCertificateKeyPath: filepath.Join(confDir, "ssl", "missing", "private.key"),
	}).Error)

	recommended, err := RecommendCertificate([]string{"www.abc.cn"}, time.Now())
	require.NoError(t, err)
	require.NotNil(t, recommended)
	assert.Equal(t, abc.ID, recommended.ID)
	assert.Equal(t, "abc", recommended.Name)

	recommended, err = RecommendCertificate([]string{"www.abc.cn", "www.def.cn"}, time.Now())
	require.NoError(t, err)
	assert.Nil(t, recommended)

	recommended, err = RecommendCertificate(nil, time.Now())
	require.NoError(t, err)
	assert.Nil(t, recommended)
}
