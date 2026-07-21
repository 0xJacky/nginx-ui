package azuredns

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSelectZone(t *testing.T) {
	zones := []zoneRef{
		{ResourceGroup: "rg-a", Name: "example.com"},
		{ResourceGroup: "rg-b", Name: "sub.example.com"},
		{ResourceGroup: "rg-c", Name: "example.net"},
	}

	tests := []struct {
		name   string
		domain string
		zones  []zoneRef
		want   zoneRef
		wantOK bool
	}{
		{
			name:   "exact match",
			domain: "example.com",
			zones:  zones,
			want:   zoneRef{ResourceGroup: "rg-a", Name: "example.com"},
			wantOK: true,
		},
		{
			name:   "subdomain resolves to parent zone",
			domain: "api.example.com",
			zones:  zones,
			want:   zoneRef{ResourceGroup: "rg-a", Name: "example.com"},
			wantOK: true,
		},
		{
			name:   "most specific zone wins",
			domain: "api.sub.example.com",
			zones:  zones,
			want:   zoneRef{ResourceGroup: "rg-b", Name: "sub.example.com"},
			wantOK: true,
		},
		{
			name:   "delegated zone matched exactly",
			domain: "sub.example.com",
			zones:  zones,
			want:   zoneRef{ResourceGroup: "rg-b", Name: "sub.example.com"},
			wantOK: true,
		},
		{
			name:   "trailing dot and case are ignored",
			domain: "API.Example.COM.",
			zones:  zones,
			want:   zoneRef{ResourceGroup: "rg-a", Name: "example.com"},
			wantOK: true,
		},
		{
			name:   "suffix without label boundary does not match",
			domain: "notexample.com",
			zones:  zones,
			wantOK: false,
		},
		{
			name:   "unknown domain",
			domain: "example.org",
			zones:  zones,
			wantOK: false,
		},
		{
			name:   "empty zone list",
			domain: "example.com",
			zones:  nil,
			wantOK: false,
		},
		{
			name:   "empty domain",
			domain: "",
			zones:  zones,
			wantOK: false,
		},
		{
			name:   "zones with empty names are skipped",
			domain: "example.com",
			zones:  []zoneRef{{ResourceGroup: "rg", Name: ""}},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := selectZone(tt.domain, tt.zones)
			require.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestResourceGroupFromResourceID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "canonical casing",
			input: "/subscriptions/0000/resourceGroups/rg-dns/providers/Microsoft.Network/dnszones/example.com",
			want:  "rg-dns",
		},
		{
			name:  "lowercase segment",
			input: "/subscriptions/0000/resourcegroups/rg-dns/providers/Microsoft.Network/dnszones/example.com",
			want:  "rg-dns",
		},
		{
			name:  "missing resource group",
			input: "/subscriptions/0000/providers/Microsoft.Network/dnszones/example.com",
			want:  "",
		},
		{
			name:  "trailing resource group segment",
			input: "/subscriptions/0000/resourceGroups",
			want:  "",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, resourceGroupFromResourceID(tt.input))
		})
	}
}

func TestNormalizeZoneName(t *testing.T) {
	require.Equal(t, "example.com", normalizeZoneName("Example.COM"))
	require.Equal(t, "example.com", normalizeZoneName("example.com."))
	require.Equal(t, "example.com", normalizeZoneName("  example.com  "))
	require.Equal(t, "", normalizeZoneName(""))
	require.Equal(t, "", normalizeZoneName("."))
}

func TestZoneCacheKeyIsScopedToCredentialSubscriptionAndResourceGroup(t *testing.T) {
	base := zoneCacheKey("fingerprint", "sub", "rg", "example.com")

	require.Equal(t, base, zoneCacheKey("fingerprint", "sub", "rg", "example.com"))
	require.Equal(t, base, zoneCacheKey("fingerprint", "sub", "RG", "example.com"))
	require.NotEqual(t, base, zoneCacheKey("other", "sub", "rg", "example.com"))
	require.NotEqual(t, base, zoneCacheKey("fingerprint", "other", "rg", "example.com"))
	require.NotEqual(t, base, zoneCacheKey("fingerprint", "sub", "rg", "example.net"))

	// Azure allows the same zone name in several resource groups, and two credentials
	// can share a service principal while pinning different ones, so the resource
	// group has to be part of the key or one credential reads the other's zone.
	require.NotEqual(t, base, zoneCacheKey("fingerprint", "sub", "rg-other", "example.com"))
	require.NotEqual(t, base, zoneCacheKey("fingerprint", "sub", "", "example.com"))
}

func TestStoreZoneSetsExpiry(t *testing.T) {
	key := zoneCacheKey("test-fingerprint", "test-sub", "rg", "example.com")
	t.Cleanup(func() { zoneCache.Delete(key) })

	zone := zoneRef{ResourceGroup: "rg", Name: "example.com"}
	storeZone(key, zone)

	cached, ok := zoneCache.Load(key)
	require.True(t, ok)

	entry, ok := cached.(zoneCacheEntry)
	require.True(t, ok)
	require.Equal(t, zone, entry.zone)
	require.True(t, entry.expiresAt.After(time.Now()))
	require.True(t, entry.expiresAt.Before(time.Now().Add(zoneCacheTTL+time.Minute)))
}

func TestResolveZoneDoesNotLeakAcrossResourceGroups(t *testing.T) {
	const (
		fingerprint  = "shared-service-principal"
		subscription = "shared-subscription"
		domain       = "example.com"
	)

	warm := zoneCacheKey(fingerprint, subscription, "rg-prod", domain)
	cold := zoneCacheKey(fingerprint, subscription, "rg-staging", domain)
	t.Cleanup(func() {
		zoneCache.Delete(warm)
		zoneCache.Delete(cold)
	})

	storeZone(warm, zoneRef{ResourceGroup: "rg-prod", Name: domain})

	// A provider pinned to a different resource group must miss the cache. It has a
	// nil credential, so any real Azure lookup fails rather than returning rg-prod.
	staging := &provider{
		fingerprint:    fingerprint,
		subscriptionID: subscription,
		resourceGroup:  "rg-staging",
	}

	zone, err := staging.resolveZone(context.Background(), domain)
	require.Error(t, err)
	require.NotEqual(t, "rg-prod", zone.ResourceGroup)

	prod := &provider{
		fingerprint:    fingerprint,
		subscriptionID: subscription,
		resourceGroup:  "rg-prod",
	}

	cached, err := prod.resolveZone(context.Background(), domain)
	require.NoError(t, err)
	require.Equal(t, zoneRef{ResourceGroup: "rg-prod", Name: domain}, cached)
}
