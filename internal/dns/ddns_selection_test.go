package dns_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	dnsSvc "github.com/0xJacky/Nginx-UI/internal/dns"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/require"
)

// newDualStackIPServers serves a fixed public IPv4 and IPv6 address and points
// the DDNS detectors at them for the duration of the test.
func newDualStackIPServers(t *testing.T, ipv4, ipv6 string) {
	t.Helper()

	ipv4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(ipv4))
	}))
	t.Cleanup(ipv4Server.Close)
	ipv6Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(ipv6))
	}))
	t.Cleanup(ipv6Server.Close)

	restore := dnsSvc.OverrideIPEndpointsForTest([]string{ipv4Server.URL}, []string{ipv6Server.URL})
	t.Cleanup(restore)
}

func createDDNSTestDomain(t *testing.T) (*dnsSvc.Service, uint64) {
	t.Helper()

	q := setupTestQuery(t)
	// The in-memory database is shared across the package's tests, and
	// TestDomainLifecycle counts every domain row, so leave nothing behind.
	t.Cleanup(func() {
		db := model.UseDB().Unscoped()
		require.NoError(t, db.Where("1 = 1").Delete(&model.DnsDomain{}).Error)
		require.NoError(t, db.Where("1 = 1").Delete(&model.DnsCredential{}).Error)
	})
	service := dnsSvc.NewService()
	cred := createCredential(t, q)
	domain, err := service.CreateDomain(context.Background(), dnsSvc.DomainInput{
		Domain:          "example.com",
		DnsCredentialID: cred.ID,
	})
	require.NoError(t, err)
	return service, domain.ID
}

func targetIDs(targets []model.DDNSRecordTarget) []string {
	ids := make([]string, 0, len(targets))
	for _, target := range targets {
		ids = append(ids, target.ID)
	}
	return ids
}

// Reproduces issue #1968: two names that each carry an A and an AAAA record,
// with only dns4 (A) and dns6 (AAAA) selected in dual-stack mode with cleanup
// on. Saving must keep exactly those two targets, and the scheduled update must
// write only them, each with the address of its own family.
func TestDDNSOnlyUpdatesSelectedRecordsIssue1968(t *testing.T) {
	registerMockProvider()
	setMockRecords([]dnsSvc.Record{
		{ID: "dns4-a", Type: "A", Name: "dns4", Content: "198.51.100.4", TTL: 300},
		{ID: "dns4-aaaa", Type: "AAAA", Name: "dns4", Content: "2001:db8:4::1", TTL: 300},
		{ID: "dns6-a", Type: "A", Name: "dns6", Content: "198.51.100.6", TTL: 300},
		{ID: "dns6-aaaa", Type: "AAAA", Name: "dns6", Content: "2001:db8:6::1", TTL: 300},
	})
	newDualStackIPServers(t, "203.0.113.4", "2001:db8::6")

	ctx := context.Background()
	service, domainID := createDDNSTestDomain(t)

	for _, version := range []string{dnsSvc.DDNSIPVersionIPv4IPv6, dnsSvc.DDNSIPVersionIPv6IPv4} {
		t.Run(version, func(t *testing.T) {
			setMockRecords(mockRecords)

			result, err := service.UpdateDDNSConfigWithDetails(ctx, domainID, dnsSvc.DDNSUpdateInput{
				Enabled:                   true,
				IntervalSeconds:           dnsSvc.DefaultDDNSInterval(),
				IPVersion:                 version,
				CleanupConflictingRecords: true,
				RecordIDs:                 []string{"dns4-a", "dns6-aaaa"},
			})
			require.NoError(t, err)
			require.ElementsMatch(t, []string{"dns4-a", "dns6-aaaa"}, targetIDs(result.Config.Targets))
			require.Empty(t, result.DeletedRecords)
			require.Empty(t, getMockCreatedRecords())
			require.Empty(t, getMockDeletedRecordIDs())

			// The persisted config is what the scheduler and the UI read back.
			stored, err := service.GetDDNSConfig(ctx, domainID)
			require.NoError(t, err)
			require.ElementsMatch(t, []string{"dns4-a", "dns6-aaaa"}, targetIDs(stored.Targets))

			require.NoError(t, dnsSvc.RunDDNSUpdate(ctx, domainID))

			updated := getMockUpdatedRecords()
			require.Len(t, updated, 2)
			contentByID := map[string]string{}
			typeByID := map[string]string{}
			for _, record := range updated {
				contentByID[record.ID] = record.Content
				typeByID[record.ID] = record.Type
			}
			require.Equal(t, map[string]string{
				"dns4-a":    "203.0.113.4",
				"dns6-aaaa": "2001:db8::6",
			}, contentByID)
			require.Equal(t, map[string]string{"dns4-a": "A", "dns6-aaaa": "AAAA"}, typeByID)
		})
	}
}

// An existing sibling the user left unselected is not deleted either, even when
// its IP family cannot be detected at save time.
func TestUpdateDDNSConfigKeepsUnselectedSiblingWhenFamilyUnavailable(t *testing.T) {
	registerMockProvider()
	setMockRecords([]dnsSvc.Record{
		{ID: "a-record", Type: "A", Name: "home", Content: "198.51.100.10", TTL: 600},
		{ID: "aaaa-record", Type: "AAAA", Name: "home", Content: "2001:db8::1", TTL: 600},
	})

	ipv4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("198.51.100.12"))
	}))
	defer ipv4Server.Close()
	restore := dnsSvc.OverrideIPEndpointsForTest([]string{ipv4Server.URL}, []string{"http://127.0.0.1:1"})
	defer restore()

	service, domainID := createDDNSTestDomain(t)

	result, err := service.UpdateDDNSConfigWithDetails(context.Background(), domainID, dnsSvc.DDNSUpdateInput{
		Enabled:                   true,
		IntervalSeconds:           dnsSvc.DefaultDDNSInterval(),
		IPVersion:                 dnsSvc.DDNSIPVersionIPv4IPv6,
		CleanupConflictingRecords: true,
		RecordIDs:                 []string{"a-record"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"a-record"}, targetIDs(result.Config.Targets))
	require.Empty(t, result.DeletedRecords)
	require.Empty(t, getMockDeletedRecordIDs())
}

// Selecting a record name (the API accepts names as well as IDs) selects every
// address record at that name, so the name stays fully managed.
func TestUpdateDDNSConfigNameSelectionManagesAllRecordsAtName(t *testing.T) {
	registerMockProvider()
	setMockRecords([]dnsSvc.Record{
		{ID: "a-record", Type: "A", Name: "home", Content: "198.51.100.10", TTL: 600},
		{ID: "aaaa-record", Type: "AAAA", Name: "home", Content: "2001:db8::1", TTL: 600},
		{ID: "other-a", Type: "A", Name: "other", Content: "198.51.100.11", TTL: 600},
	})
	newDualStackIPServers(t, "198.51.100.12", "2001:db8::20")

	service, domainID := createDDNSTestDomain(t)

	cfg, err := service.UpdateDDNSConfig(context.Background(), domainID, dnsSvc.DDNSUpdateInput{
		Enabled:                   true,
		IntervalSeconds:           dnsSvc.DefaultDDNSInterval(),
		IPVersion:                 dnsSvc.DDNSIPVersionIPv4IPv6,
		CleanupConflictingRecords: true,
		RecordIDs:                 []string{"home"},
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"a-record", "aaaa-record"}, targetIDs(cfg.Targets))
	require.Empty(t, getMockCreatedRecords())
	require.Empty(t, getMockDeletedRecordIDs())
}
