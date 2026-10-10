package plugin

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
)

// UsageKindCertificate marks a certificate that renews through the plugin.
const UsageKindCertificate = "certificate"

// maxUsageItems bounds the listed items, Total still counts every one.
const maxUsageItems = 100

// UsageItem is one thing on this node that stops working while the plugin is off.
type UsageItem struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Usage lists what depends on a plugin on this node.
type Usage struct {
	Items []UsageItem `json:"items"`
	Total int         `json:"total"`
}

// Usage reports what depends on one installed plugin on this node.
func (m *Manager) Usage(ctx context.Context, id string) (*Usage, error) {
	item, ok := m.lookup(id)
	if !ok {
		return nil, ErrPluginNotFound
	}
	m.mu.RLock()
	manifest := item.manifest
	m.mu.RUnlock()

	usage := &Usage{Items: []UsageItem{}}
	if manifest == nil {
		return usage, nil
	}

	if slices.Contains(manifest.Capabilities, protocol.CapabilityDNS01) {
		certs, err := dns01Certificates(ctx, declaredCodes(manifest, protocol.CapabilityDNS01))
		if err != nil {
			return nil, err
		}
		usage.add(certs...)
	}
	return usage, nil
}

func (u *Usage) add(items ...UsageItem) {
	u.Total += len(items)
	room := maxUsageItems - len(u.Items)
	if room <= 0 {
		return
	}
	if len(items) > room {
		items = items[:room]
	}
	u.Items = append(u.Items, items...)
}

// dns01Certificates lists the auto renewing DNS-01 certificates whose
// credential uses one of codes. A certificate whose provider cannot be told is
// counted, and so is every one when the plugin names no provider.
func dns01Certificates(ctx context.Context, codes []string) ([]UsageItem, error) {
	c := query.Cert
	certs, err := c.WithContext(ctx).
		Preload(c.DnsCredential).
		Where(c.ChallengeMethod.Eq(model.CertChallengeMethodDNS01), c.AutoCert.Eq(model.AutoCertEnabled)).
		Order(c.ID).
		Find()
	if err != nil {
		return nil, err
	}

	items := make([]UsageItem, 0, len(certs))
	for _, cert := range certs {
		if len(codes) > 0 && cert.DnsCredential != nil && cert.DnsCredential.ProviderCode != "" &&
			!slices.Contains(codes, cert.DnsCredential.ProviderCode) {
			continue
		}
		items = append(items, UsageItem{
			Kind: UsageKindCertificate,
			ID:   strconv.FormatUint(cert.ID, 10),
			Name: certificateLabel(cert),
		})
	}
	return items, nil
}

// certificateLabel is the name of a certificate, its domains when it has none.
func certificateLabel(cert *model.Cert) string {
	if name := strings.TrimSpace(cert.Name); name != "" {
		return name
	}
	return strings.Join(cert.Domains, ", ")
}
