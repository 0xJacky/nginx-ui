package azuredns

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
)

const (
	// zoneCacheTTL bounds how long a resolved zone location is reused. Zones move
	// between resource groups rarely, so a short cache removes a lookup round trip
	// from every record operation without stranding stale placements for long.
	zoneCacheTTL = 10 * time.Minute

	// zoneCacheLimit triggers a sweep of expired entries to bound memory growth.
	zoneCacheLimit = 256
)

// zoneRef locates an Azure DNS zone inside a subscription.
type zoneRef struct {
	ResourceGroup string
	Name          string
}

type zoneCacheEntry struct {
	zone      zoneRef
	expiresAt time.Time
}

// zoneCache is process wide because providers are constructed per API call.
var zoneCache sync.Map

// resolveZone locates the Azure DNS zone hosting the given registered domain.
func (p *provider) resolveZone(ctx context.Context, domain string) (zoneRef, error) {
	name := normalizeZoneName(firstNonEmpty(p.zoneName, domain))
	if name == "" {
		return zoneRef{}, fmt.Errorf("azuredns: resolve zone: empty domain")
	}

	key := zoneCacheKey(p.fingerprint, p.subscriptionID, p.resourceGroup, name)
	if cached, ok := zoneCache.Load(key); ok {
		if entry, valid := cached.(zoneCacheEntry); valid && time.Now().Before(entry.expiresAt) {
			return entry.zone, nil
		}
		zoneCache.Delete(key)
	}

	zone, err := p.discoverZone(ctx, name)
	if err != nil {
		return zoneRef{}, err
	}

	storeZone(key, zone)

	return zone, nil
}

// discoverZone finds the zone that hosts the domain, allowing the domain to be a
// subdomain of a zone that Azure actually holds.
func (p *provider) discoverZone(ctx context.Context, domain string) (zoneRef, error) {
	client, err := p.zonesClient()
	if err != nil {
		return zoneRef{}, err
	}

	if p.resourceGroup != "" {
		if _, err := client.Get(ctx, p.resourceGroup, domain, nil); err == nil {
			return zoneRef{ResourceGroup: p.resourceGroup, Name: domain}, nil
		} else if !isNotFound(err) {
			return zoneRef{}, fmt.Errorf("azuredns: resolve zone: %w", err)
		}

		// An explicit zone name is taken at face value, so a miss is fatal.
		if p.zoneName != "" {
			return zoneRef{}, fmt.Errorf("azuredns: resolve zone: zone %s not found in resource group %s", domain, p.resourceGroup)
		}

		zones := make([]zoneRef, 0)
		pager := client.NewListByResourceGroupPager(p.resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return zoneRef{}, fmt.Errorf("azuredns: resolve zone: %w", err)
			}
			for _, zone := range page.Value {
				if zone == nil || zone.Name == nil {
					continue
				}
				zones = append(zones, zoneRef{ResourceGroup: p.resourceGroup, Name: normalizeZoneName(*zone.Name)})
			}
		}

		if match, ok := selectZone(domain, zones); ok {
			return match, nil
		}

		return zoneRef{}, fmt.Errorf("azuredns: resolve zone: no Azure DNS zone found for %s in resource group %s", domain, p.resourceGroup)
	}

	zones := make([]zoneRef, 0)
	pager := client.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return zoneRef{}, fmt.Errorf("azuredns: resolve zone: %w", err)
		}
		for _, zone := range page.Value {
			if zone == nil || zone.Name == nil || zone.ID == nil {
				continue
			}
			resourceGroup := resourceGroupFromResourceID(*zone.ID)
			if resourceGroup == "" {
				continue
			}
			zones = append(zones, zoneRef{ResourceGroup: resourceGroup, Name: normalizeZoneName(*zone.Name)})
		}
	}

	if match, ok := selectZone(domain, zones); ok {
		return match, nil
	}

	return zoneRef{}, fmt.Errorf("azuredns: resolve zone: no Azure DNS zone found for %s", domain)
}

// zonesClient builds a zones client bound to the configured subscription.
func (p *provider) zonesClient() (*armdns.ZonesClient, error) {
	client, err := armdns.NewZonesClient(p.subscriptionID, p.credential, p.armOptions())
	if err != nil {
		return nil, fmt.Errorf("azuredns: new zones client: %w", err)
	}

	return client, nil
}

// selectZone picks the most specific zone hosting the domain, so a delegated child
// zone wins over its parent when both are present in the subscription.
func selectZone(domain string, zones []zoneRef) (zoneRef, bool) {
	target := normalizeZoneName(domain)
	if target == "" {
		return zoneRef{}, false
	}

	var (
		best  zoneRef
		found bool
	)

	for _, zone := range zones {
		if zone.Name == "" {
			continue
		}
		if target != zone.Name && !strings.HasSuffix(target, "."+zone.Name) {
			continue
		}
		if !found || len(zone.Name) > len(best.Name) {
			best = zone
			found = true
		}
	}

	return best, found
}

// resourceGroupFromResourceID extracts the resource group from an ARM resource ID.
func resourceGroupFromResourceID(id string) string {
	segments := strings.Split(id, "/")
	for i, segment := range segments {
		if strings.EqualFold(segment, "resourceGroups") && i+1 < len(segments) {
			return segments[i+1]
		}
	}

	return ""
}

// normalizeZoneName lowercases a zone name and strips the trailing root label.
func normalizeZoneName(value string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(value)), ".")
}

// zoneCacheKey scopes a cached zone to everything that can change where a zone
// resolves. The resource group belongs in the key because Azure allows the same zone
// name in several resource groups, and two credentials can share a service principal
// while pinning different ones.
func zoneCacheKey(fingerprint, subscriptionID, resourceGroup, domain string) string {
	return strings.Join([]string{fingerprint, subscriptionID, strings.ToLower(resourceGroup), domain}, "|")
}

// storeZone caches a resolved zone, sweeping expired entries once the cache grows.
func storeZone(key string, zone zoneRef) {
	now := time.Now()

	count := 0
	zoneCache.Range(func(_, _ any) bool {
		count++
		return count < zoneCacheLimit
	})

	if count >= zoneCacheLimit {
		zoneCache.Range(func(k, v any) bool {
			if entry, ok := v.(zoneCacheEntry); ok && now.After(entry.expiresAt) {
				zoneCache.Delete(k)
			}
			return true
		})
	}

	zoneCache.Store(key, zoneCacheEntry{zone: zone, expiresAt: now.Add(zoneCacheTTL)})
}
