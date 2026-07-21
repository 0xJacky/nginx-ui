package azuredns

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

// providerCode matches the code of the Azure DNS credential shipped with lego.
const providerCode = "azuredns"

// provider manages record sets of a public Azure DNS zone.
//
// Azure groups records by name and type into a record set that shares one TTL, so a
// single record set is surfaced as a single record whose content holds one value per
// line. That keeps record IDs free of any content, which is what lets the DDNS engine
// keep matching a persisted target after the record it points at changes.
type provider struct {
	credential     azcore.TokenCredential
	cloudConfig    cloud.Configuration
	fingerprint    string
	subscriptionID string
	resourceGroup  string
	zoneName       string
}

func init() {
	dns.RegisterProvider(providerCode, newProvider)
}

func newProvider(cred *dns.Credential) (dns.Provider, error) {
	cfg := parseAuthConfig(cred)

	if cfg.PrivateZone {
		// Private zones live under a different resource provider with an
		// incompatible client, so refuse rather than silently managing the
		// public zone of the same name.
		return nil, fmt.Errorf("azuredns: private DNS zones are not supported for DNS record management")
	}

	if cfg.SubscriptionID == "" {
		return nil, fmt.Errorf("azuredns: missing AZURE_SUBSCRIPTION_ID")
	}

	cloudConfig, err := parseCloud(cfg.Environment)
	if err != nil {
		return nil, err
	}

	method, err := resolveAuthMethod(cfg)
	if err != nil {
		return nil, err
	}

	fingerprint := credentialFingerprint(cfg, method, cfg.Environment)

	credential, err := cachedTokenCredential(cfg, method, cloudConfig, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("azuredns: build credential: %w", err)
	}

	return &provider{
		credential:     credential,
		cloudConfig:    cloudConfig,
		fingerprint:    fingerprint,
		subscriptionID: cfg.SubscriptionID,
		resourceGroup:  cfg.ResourceGroup,
		zoneName:       cfg.ZoneName,
	}, nil
}

func (p *provider) ListRecords(ctx context.Context, domain string, filter dns.RecordFilter) ([]dns.Record, error) {
	zone, err := p.resolveZone(ctx, domain)
	if err != nil {
		return nil, err
	}

	client, err := p.recordSetsClient()
	if err != nil {
		return nil, err
	}

	recordSets, err := listRecordSets(ctx, client, zone, filter.Type)
	if err != nil {
		return nil, err
	}

	records := make([]dns.Record, 0, len(recordSets))
	for _, recordSet := range recordSets {
		record, ok := flattenRecordSet(recordSet)
		if !ok || !matchesFilter(record, filter) {
			continue
		}
		records = append(records, record)
	}

	// The API layer paginates by slicing this slice, so the order has to be stable.
	sort.Slice(records, func(i, j int) bool {
		if records[i].Name != records[j].Name {
			return records[i].Name < records[j].Name
		}
		return records[i].Type < records[j].Type
	})

	return records, nil
}

func (p *provider) CreateRecord(ctx context.Context, domain string, input dns.RecordInput) (dns.Record, error) {
	zone, err := p.resolveZone(ctx, domain)
	if err != nil {
		return dns.Record{}, err
	}

	name, err := relativeName(input.Name, zone.Name)
	if err != nil {
		return dns.Record{}, err
	}

	recordType, err := toRecordType(input.Type)
	if err != nil {
		return dns.Record{}, err
	}

	props, err := recordSetProperties(input, recordType)
	if err != nil {
		return dns.Record{}, err
	}

	client, err := p.recordSetsClient()
	if err != nil {
		return dns.Record{}, err
	}

	// If-None-Match makes this a create, never an accidental overwrite of a record
	// set the user did not know existed. Adding a value to an existing set is an
	// update of that set instead.
	response, err := client.CreateOrUpdate(ctx, zone.ResourceGroup, zone.Name, name, recordType,
		armdns.RecordSet{Properties: props},
		&armdns.RecordSetsClientCreateOrUpdateOptions{IfNoneMatch: ptr("*")})
	if err != nil {
		if isConflict(err) {
			return dns.Record{}, fmt.Errorf("azuredns: a DNS record set with the same name and type already exists")
		}
		return dns.Record{}, fmt.Errorf("azuredns: create record: %w", err)
	}

	return recordFrom(name, recordType, response.Properties), nil
}

func (p *provider) UpdateRecord(ctx context.Context, domain string, recordID string, input dns.RecordInput) (dns.Record, error) {
	zone, err := p.resolveZone(ctx, domain)
	if err != nil {
		return dns.Record{}, err
	}

	name, recordType, err := parseRecordID(recordID)
	if err != nil {
		return dns.Record{}, err
	}

	if err := ensureNoRename(input, zone.Name, name, recordType); err != nil {
		return dns.Record{}, err
	}

	props, err := recordSetProperties(input, recordType)
	if err != nil {
		return dns.Record{}, err
	}

	client, err := p.recordSetsClient()
	if err != nil {
		return dns.Record{}, err
	}

	existing, serverName, err := getRecordSet(ctx, client, zone, name, recordType)
	if err != nil {
		if isNotFound(err) {
			return dns.Record{}, fmt.Errorf("azuredns: update record: record set %s not found", recordID)
		}
		return dns.Record{}, fmt.Errorf("azuredns: update record: %w", err)
	}

	if existing.Properties != nil && existing.Properties.TargetResource != nil && existing.Properties.TargetResource.ID != nil {
		return dns.Record{}, fmt.Errorf("azuredns: alias record sets are read-only")
	}

	response, err := client.CreateOrUpdate(ctx, zone.ResourceGroup, zone.Name, serverName, recordType,
		armdns.RecordSet{Properties: props},
		&armdns.RecordSetsClientCreateOrUpdateOptions{IfMatch: existing.Etag})
	if err != nil {
		if isConflict(err) {
			// The payload fully replaces the record set, so retrying would clobber
			// whatever the concurrent writer just stored.
			return dns.Record{}, fmt.Errorf("azuredns: update record: the record set was modified concurrently, reload and try again")
		}
		return dns.Record{}, fmt.Errorf("azuredns: update record: %w", err)
	}

	return recordFrom(name, recordType, response.Properties), nil
}

func (p *provider) DeleteRecord(ctx context.Context, domain string, recordID string) error {
	zone, err := p.resolveZone(ctx, domain)
	if err != nil {
		return err
	}

	name, recordType, err := parseRecordID(recordID)
	if err != nil {
		return err
	}

	if recordType == armdns.RecordTypeSOA {
		return fmt.Errorf("azuredns: SOA record sets cannot be deleted")
	}

	client, err := p.recordSetsClient()
	if err != nil {
		return err
	}

	existing, serverName, err := getRecordSet(ctx, client, zone, name, recordType)
	if err != nil {
		if isNotFound(err) {
			// Deletion is idempotent so that stale DDNS targets can be pruned.
			return nil
		}
		return fmt.Errorf("azuredns: delete record: %w", err)
	}

	if _, err := client.Delete(ctx, zone.ResourceGroup, zone.Name, serverName, recordType,
		&armdns.RecordSetsClientDeleteOptions{IfMatch: existing.Etag}); err != nil {
		if isNotFound(err) {
			return nil
		}
		if isConflict(err) {
			return fmt.Errorf("azuredns: delete record: the record set was modified concurrently, reload and try again")
		}
		return fmt.Errorf("azuredns: delete record: %w", err)
	}

	return nil
}

// recordSetsClient builds a record sets client bound to the configured subscription.
func (p *provider) recordSetsClient() (*armdns.RecordSetsClient, error) {
	client, err := armdns.NewRecordSetsClient(p.subscriptionID, p.credential, p.armOptions())
	if err != nil {
		return nil, fmt.Errorf("azuredns: new record sets client: %w", err)
	}

	return client, nil
}

// armOptions carries the selected Azure cloud into every management request.
func (p *provider) armOptions() *arm.ClientOptions {
	return &arm.ClientOptions{ClientOptions: azcore.ClientOptions{Cloud: p.cloudConfig}}
}

// listRecordSets enumerates the record sets of a zone, narrowing by type when asked.
func listRecordSets(ctx context.Context, client *armdns.RecordSetsClient, zone zoneRef, filterType string) ([]*armdns.RecordSet, error) {
	recordSets := make([]*armdns.RecordSet, 0)

	if strings.TrimSpace(filterType) != "" {
		recordType, err := toRecordType(filterType)
		if err != nil {
			return nil, err
		}

		pager := client.NewListByTypePager(zone.ResourceGroup, zone.Name, recordType, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("azuredns: list records: %w", err)
			}
			recordSets = append(recordSets, page.Value...)
		}

		return recordSets, nil
	}

	pager := client.NewListAllByDNSZonePager(zone.ResourceGroup, zone.Name, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("azuredns: list records: %w", err)
		}
		recordSets = append(recordSets, page.Value...)
	}

	return recordSets, nil
}

// getRecordSet fetches a record set and reports the spelling Azure stores it under,
// falling back to a case insensitive scan when the canonical lookup misses.
func getRecordSet(ctx context.Context, client *armdns.RecordSetsClient, zone zoneRef, name string, recordType armdns.RecordType) (armdns.RecordSet, string, error) {
	response, err := client.Get(ctx, zone.ResourceGroup, zone.Name, name, recordType, nil)
	if err == nil {
		return response.RecordSet, name, nil
	}

	if !isNotFound(err) {
		return armdns.RecordSet{}, "", err
	}

	notFound := err

	pager := client.NewListByTypePager(zone.ResourceGroup, zone.Name, recordType, nil)
	for pager.More() {
		page, pageErr := pager.NextPage(ctx)
		if pageErr != nil {
			return armdns.RecordSet{}, "", notFound
		}
		for _, recordSet := range page.Value {
			if recordSet == nil || recordSet.Name == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSuffix(*recordSet.Name, "."), name) {
				return *recordSet, *recordSet.Name, nil
			}
		}
	}

	return armdns.RecordSet{}, "", notFound
}

// ensureNoRename rejects an update that would move a record set, which Azure cannot
// do in place and which create plus delete cannot emulate without risking data loss.
func ensureNoRename(input dns.RecordInput, zone string, name string, recordType armdns.RecordType) error {
	if strings.TrimSpace(input.Name) != "" {
		inputName, err := relativeName(input.Name, zone)
		if err != nil {
			return err
		}
		if inputName != name {
			return renameNotSupportedError()
		}
	}

	if strings.TrimSpace(input.Type) != "" {
		inputType, err := toRecordType(input.Type)
		if err != nil {
			return err
		}
		if inputType != recordType {
			return renameNotSupportedError()
		}
	}

	return nil
}

func renameNotSupportedError() error {
	return fmt.Errorf("azuredns: renaming a record set or changing its type is not supported, delete it and create a new one instead")
}

func isNotFound(err error) bool {
	var responseErr *azcore.ResponseError
	if errors.As(err, &responseErr) {
		return responseErr.StatusCode == http.StatusNotFound
	}

	return false
}

func isConflict(err error) bool {
	var responseErr *azcore.ResponseError
	if errors.As(err, &responseErr) {
		return responseErr.StatusCode == http.StatusConflict ||
			responseErr.StatusCode == http.StatusPreconditionFailed
	}

	return false
}
