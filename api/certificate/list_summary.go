package certificate

import (
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/gorm"
)

// CertListCounts holds the number of certificates per list filter.
type CertListCounts struct {
	All      int `json:"all"`
	Expiring int `json:"expiring"`
	Failed   int `json:"failed"`
	Expired  int `json:"expired"`
}

type certListResponse struct {
	cosyModel.DataList
	Counts CertListCounts `json:"counts"`
}

type certListSummary struct {
	counts CertListCounts
	// matched lists the ids of the requested filter.
	matched []uint64
	// infos caches the parsed certificate files by record id.
	infos map[uint64]*cert.Info
}

// overviewNow is a test seam.
var overviewNow = time.Now

func buildOverview(m *model.Cert, info *cert.Info) cert.Overview {
	return cert.BuildOverview(m, info, overviewNow(), settings.CertSettings.GetCertRenewalInterval())
}

// applyCertKeyword matches the keyword against the name and the domains.
func applyCertKeyword(tx *gorm.DB, keyword string) *gorm.DB {
	if keyword == "" {
		return tx
	}
	like := "%" + strings.ToLower(keyword) + "%"
	return tx.Where("(LOWER(certs.name) LIKE ? OR LOWER(certs.domains) LIKE ?)", like, like)
}

// summarizeCertList reads every certificate matching the keyword once, so the
// counts cover the whole list and not only the current page. The state lives
// partly in the certificate files, which rules out a pure SQL filter.
func summarizeCertList(keyword string, filter cert.ListFilter) (*certListSummary, error) {
	var models []*model.Cert
	if err := applyCertKeyword(model.UseDB().Model(&model.Cert{}), keyword).Find(&models).Error; err != nil {
		return nil, err
	}

	now := overviewNow()
	summary := &certListSummary{
		matched: make([]uint64, 0),
		infos:   make(map[uint64]*cert.Info, len(models)),
	}
	for _, m := range models {
		info, _ := cert.GetCertInfo(m.SSLCertificatePath)
		summary.infos[m.ID] = info
		overview := buildOverview(m, info)

		summary.counts.All++
		if overview.MatchesFilter(cert.FilterExpiring, info, now) {
			summary.counts.Expiring++
		}
		if overview.MatchesFilter(cert.FilterFailed, info, now) {
			summary.counts.Failed++
		}
		if overview.MatchesFilter(cert.FilterExpired, info, now) {
			summary.counts.Expired++
		}
		if overview.MatchesFilter(filter, info, now) {
			summary.matched = append(summary.matched, m.ID)
		}
	}
	return summary, nil
}

// dnsProviderNames maps DNS credential ids to their provider names.
func dnsProviderNames(models []*model.Cert) map[uint64]string {
	ids := make([]uint64, 0, len(models))
	for _, m := range models {
		if m != nil && m.DnsCredentialID != 0 && m.ChallengeMethod == cert.DNS01 {
			ids = append(ids, m.DnsCredentialID)
		}
	}

	names := make(map[uint64]string, len(ids))
	db := model.UseDB()
	if len(ids) == 0 || db == nil {
		return names
	}

	var credentials []model.DnsCredential
	if err := db.Select("id", "provider", "provider_code").Where("id IN ?", ids).Find(&credentials).Error; err != nil {
		return names
	}
	for _, credential := range credentials {
		name := credential.Provider
		if name == "" {
			name = credential.ProviderCode
		}
		names[credential.ID] = name
	}
	return names
}
