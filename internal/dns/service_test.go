package dns_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	certdns "github.com/0xJacky/Nginx-UI/internal/cert/dns"
	dnsSvc "github.com/0xJacky/Nginx-UI/internal/dns"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDomainLifecycle(t *testing.T) {
	registerMockProvider()
	setMockRecords([]dnsSvc.Record{{
		ID:      "1",
		Type:    "mock",
		Name:    "@",
		Content: "127.0.0.1",
		TTL:     60,
	}})

	q := setupTestQuery(t)
	ctx := context.Background()
	service := dnsSvc.NewService()

	cred := createCredential(t, q)

	domain, err := service.CreateDomain(ctx, dnsSvc.DomainInput{
		Domain:          "Example.com.",
		Description:     "Test domain",
		DnsCredentialID: cred.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "example.com", domain.Domain)

	_, err = service.CreateDomain(ctx, dnsSvc.DomainInput{
		Domain:          "example.com",
		Description:     "duplicate",
		DnsCredentialID: cred.ID,
	})
	require.ErrorIs(t, err, dnsSvc.ErrDuplicateDomain)

	list, total, err := service.ListDomains(ctx, dnsSvc.DomainListOptions{
		Page:    1,
		PerPage: 10,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, list, 1)

	records, err := service.ListRecords(ctx, domain.ID, dnsSvc.RecordListOptions{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "mock", records[0].Type)
}

func TestUpdateDDNSConfigRejectsInvalidIPVersionValues(t *testing.T) {
	registerMockProvider()
	setMockRecords(nil)

	q := setupTestQuery(t)
	ctx := context.Background()
	service := dnsSvc.NewService()

	cred := createCredential(t, q)
	domain, err := service.CreateDomain(ctx, dnsSvc.DomainInput{
		Domain:          "example.com",
		DnsCredentialID: cred.ID,
	})
	require.NoError(t, err)

	for _, version := range []string{"", "both"} {
		_, err = service.UpdateDDNSConfig(ctx, domain.ID, dnsSvc.DDNSUpdateInput{
			Enabled:         false,
			IntervalSeconds: dnsSvc.DefaultDDNSInterval(),
			IPVersion:       version,
		})
		require.ErrorIs(t, err, dnsSvc.ErrInvalidDDNSIPVersion)
	}
}

func TestUpdateDDNSConfigPreservesLastErrorWhenDisabled(t *testing.T) {
	registerMockProvider()
	setMockRecords(nil)

	q := setupTestQuery(t)
	ctx := context.Background()
	service := dnsSvc.NewService()

	cred := createCredential(t, q)
	domain, err := service.CreateDomain(ctx, dnsSvc.DomainInput{
		Domain:          "example.com",
		DnsCredentialID: cred.ID,
	})
	require.NoError(t, err)

	domain.DDNSConfig = &model.DDNSConfig{
		Enabled:         true,
		IntervalSeconds: dnsSvc.DefaultDDNSInterval(),
		IPVersion:       "ipv4_ipv6",
		LastError:       "previous error",
	}
	require.NoError(t, model.UseDB().WithContext(ctx).Save(domain).Error)

	cfg, err := service.UpdateDDNSConfig(ctx, domain.ID, dnsSvc.DDNSUpdateInput{
		Enabled:         false,
		IntervalSeconds: dnsSvc.DefaultDDNSInterval(),
		IPVersion:       "ipv4_ipv6",
	})
	require.NoError(t, err)
	require.Equal(t, "previous error", cfg.LastError)
}

func TestGetDDNSConfigNormalizesInvalidIPVersionWithoutPersisting(t *testing.T) {
	registerMockProvider()
	setMockRecords(nil)

	q := setupTestQuery(t)
	ctx := context.Background()
	service := dnsSvc.NewService()

	cred := createCredential(t, q)
	domain, err := service.CreateDomain(ctx, dnsSvc.DomainInput{
		Domain:          "example.com",
		DnsCredentialID: cred.ID,
	})
	require.NoError(t, err)

	domain.DDNSConfig = &model.DDNSConfig{
		Enabled:         true,
		IntervalSeconds: dnsSvc.DefaultDDNSInterval(),
		IPVersion:       "invalid",
	}
	require.NoError(t, model.UseDB().WithContext(ctx).Save(domain).Error)

	cfg, err := service.GetDDNSConfig(ctx, domain.ID)
	require.NoError(t, err)
	require.Equal(t, "ipv4_ipv6", cfg.IPVersion)

	var persisted model.DnsDomain
	require.NoError(t, model.UseDB().WithContext(ctx).First(&persisted, domain.ID).Error)
	require.Equal(t, "invalid", persisted.DDNSConfig.IPVersion)
}

func TestGetDDNSConfigNormalizesEmptyIPVersionWithoutPersisting(t *testing.T) {
	registerMockProvider()
	setMockRecords(nil)

	q := setupTestQuery(t)
	ctx := context.Background()
	service := dnsSvc.NewService()

	cred := createCredential(t, q)
	domain, err := service.CreateDomain(ctx, dnsSvc.DomainInput{
		Domain:          "example.com",
		DnsCredentialID: cred.ID,
	})
	require.NoError(t, err)

	domain.DDNSConfig = &model.DDNSConfig{
		Enabled:         true,
		IntervalSeconds: dnsSvc.DefaultDDNSInterval(),
		IPVersion:       "",
	}
	require.NoError(t, model.UseDB().WithContext(ctx).Save(domain).Error)

	cfg, err := service.GetDDNSConfig(ctx, domain.ID)
	require.NoError(t, err)
	require.Equal(t, "ipv4_ipv6", cfg.IPVersion)

	var persisted model.DnsDomain
	require.NoError(t, model.UseDB().WithContext(ctx).First(&persisted, domain.ID).Error)
	require.Empty(t, persisted.DDNSConfig.IPVersion)
}

func TestRunDDNSUpdateNormalizesEmptyIPVersionAndRecordsBestEffortWarnings(t *testing.T) {
	registerMockProvider()
	setMockRecords([]dnsSvc.Record{
		{
			ID:      "a-record",
			Type:    "A",
			Name:    "@",
			Content: "198.51.100.10",
			TTL:     600,
		},
		{
			ID:      "aaaa-record",
			Type:    "AAAA",
			Name:    "@",
			Content: "2001:db8::10",
			TTL:     600,
		},
	})

	ipv4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("198.51.100.12"))
	}))
	defer ipv4Server.Close()
	restore := dnsSvc.OverrideIPEndpointsForTest([]string{ipv4Server.URL}, []string{"http://127.0.0.1:1"})
	defer restore()

	q := setupTestQuery(t)
	ctx := context.Background()
	service := dnsSvc.NewService()

	cred := createCredential(t, q)
	domain, err := service.CreateDomain(ctx, dnsSvc.DomainInput{
		Domain:          "example.com",
		DnsCredentialID: cred.ID,
	})
	require.NoError(t, err)

	domain.DDNSConfig = &model.DDNSConfig{
		Enabled:         true,
		IntervalSeconds: dnsSvc.DefaultDDNSInterval(),
		IPVersion:       "",
		Targets: []model.DDNSRecordTarget{
			{ID: "a-record", Name: "@", Type: "A"},
			{ID: "aaaa-record", Name: "@", Type: "AAAA"},
		},
	}
	require.NoError(t, model.UseDB().WithContext(ctx).Save(domain).Error)

	require.NoError(t, dnsSvc.RunDDNSUpdate(ctx, domain.ID))
	require.Equal(t, []string{"a-record"}, getMockUpdatedRecordIDs())

	var persisted model.DnsDomain
	require.NoError(t, model.UseDB().WithContext(ctx).First(&persisted, domain.ID).Error)
	require.Empty(t, persisted.DDNSConfig.IPVersion)
	require.Contains(t, persisted.DDNSConfig.LastError, "ipv6:")
}

func setupTestQuery(tb testing.TB) *query.Query {
	tb.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(tb, err)

	require.NoError(tb, db.AutoMigrate(&model.DnsCredential{}, &model.DnsDomain{}))

	model.Use(db)
	q := query.Use(db)
	query.SetDefault(db)

	return q
}

func createCredential(tb testing.TB, q *query.Query) *model.DnsCredential {
	tb.Helper()

	cred := &model.DnsCredential{
		Name:     "Mock Credential",
		Provider: "Mock",
		Config: &certdns.Config{
			Code: "mock",
			Configuration: &certdns.Configuration{
				Credentials: map[string]string{
					"TOKEN": "foo",
				},
				Additional: map[string]string{},
			},
		},
	}

	err := q.DnsCredential.Create(cred)
	require.NoError(tb, err)

	return cred
}

var registerOnce sync.Once

func registerMockProvider() {
	registerOnce.Do(func() {
		dnsSvc.RegisterProvider("mock", func(*dnsSvc.Credential) (dnsSvc.Provider, error) {
			return &mockProvider{}, nil
		})
	})
}

type mockProvider struct{}

func (m *mockProvider) ListRecords(ctx context.Context, domain string, filter dnsSvc.RecordFilter) ([]dnsSvc.Record, error) {
	return append([]dnsSvc.Record(nil), mockRecords...), nil
}

func (m *mockProvider) CreateRecord(ctx context.Context, domain string, input dnsSvc.RecordInput) (dnsSvc.Record, error) {
	if mockCreateFailureType == input.Type && mockCreateFailureErr != nil {
		return dnsSvc.Record{}, mockCreateFailureErr
	}

	record := dnsSvc.Record{
		ID:      fmt.Sprintf("created-%d", len(mockCreatedRecords)+1),
		Type:    input.Type,
		Name:    input.Name,
		Content: input.Content,
		TTL:     input.TTL,
	}
	mockCreatedRecords = append(mockCreatedRecords, record)
	return record, nil
}

func (m *mockProvider) UpdateRecord(ctx context.Context, domain string, recordID string, input dnsSvc.RecordInput) (dnsSvc.Record, error) {
	record := dnsSvc.Record{
		ID:      recordID,
		Type:    input.Type,
		Name:    input.Name,
		Content: input.Content,
		TTL:     input.TTL,
	}
	mockUpdatedRecords = append(mockUpdatedRecords, record)
	return record, nil
}

func (m *mockProvider) DeleteRecord(ctx context.Context, domain string, recordID string) error {
	mockDeletedRecordIDs = append(mockDeletedRecordIDs, recordID)
	return nil
}

var mockRecords []dnsSvc.Record
var mockCreatedRecords []dnsSvc.Record
var mockUpdatedRecords []dnsSvc.Record
var mockDeletedRecordIDs []string
var mockCreateFailureType string
var mockCreateFailureErr error

func setMockRecords(records []dnsSvc.Record) {
	mockRecords = append([]dnsSvc.Record(nil), records...)
	mockCreatedRecords = nil
	mockUpdatedRecords = nil
	mockDeletedRecordIDs = nil
	mockCreateFailureType = ""
	mockCreateFailureErr = nil
}

func getMockCreatedRecords() []dnsSvc.Record {
	return append([]dnsSvc.Record(nil), mockCreatedRecords...)
}

func getMockUpdatedRecordIDs() []string {
	ids := make([]string, 0, len(mockUpdatedRecords))
	for _, record := range mockUpdatedRecords {
		ids = append(ids, record.ID)
	}
	return ids
}

func getMockDeletedRecordIDs() []string {
	return append([]string(nil), mockDeletedRecordIDs...)
}

func setMockCreateFailure(recordType string, err error) {
	mockCreateFailureType = recordType
	mockCreateFailureErr = err
}

func TestUpdateDDNSConfigSilentlySkipsRecordsOutsideIPVersionPolicy(t *testing.T) {
	registerMockProvider()
	setMockRecords([]dnsSvc.Record{
		{ID: "aaaa-record", Type: "AAAA", Name: "home", Content: "2001:db8::1", TTL: 600},
		{ID: "a-record", Type: "A", Name: "home", Content: "198.51.100.10", TTL: 600},
	})

	q := setupTestQuery(t)
	ctx := context.Background()
	service := dnsSvc.NewService()

	cred := createCredential(t, q)
	domain, err := service.CreateDomain(ctx, dnsSvc.DomainInput{
		Domain:          "example.com",
		DnsCredentialID: cred.ID,
	})
	require.NoError(t, err)

	cfg, err := service.UpdateDDNSConfig(ctx, domain.ID, dnsSvc.DDNSUpdateInput{
		Enabled:                   true,
		IntervalSeconds:           dnsSvc.DefaultDDNSInterval(),
		IPVersion:                 "ipv4",
		CleanupConflictingRecords: false,
		RecordIDs:                 []string{"aaaa-record", "a-record"},
	})
	require.NoError(t, err)
	require.Len(t, cfg.Targets, 1)
	require.Equal(t, "a-record", cfg.Targets[0].ID)
	require.Equal(t, "A", cfg.Targets[0].Type)
}

func TestUpdateDDNSConfigAutoPairsSiblingRecordWhenIPv6Available(t *testing.T) {
	registerMockProvider()
	setMockRecords([]dnsSvc.Record{
		{ID: "a-record", Type: "A", Name: "home", Content: "198.51.100.10", TTL: 600},
		{ID: "aaaa-record", Type: "AAAA", Name: "home", Content: "2001:db8::1", TTL: 600},
	})

	ipv4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("198.51.100.12"))
	}))
	defer ipv4Server.Close()
	ipv6Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("2001:db8::20"))
	}))
	defer ipv6Server.Close()
	restore := dnsSvc.OverrideIPEndpointsForTest([]string{ipv4Server.URL}, []string{ipv6Server.URL})
	defer restore()

	q := setupTestQuery(t)
	ctx := context.Background()
	service := dnsSvc.NewService()

	cred := createCredential(t, q)
	domain, err := service.CreateDomain(ctx, dnsSvc.DomainInput{
		Domain:          "example.com",
		DnsCredentialID: cred.ID,
	})
	require.NoError(t, err)

	cfg, err := service.UpdateDDNSConfig(ctx, domain.ID, dnsSvc.DDNSUpdateInput{
		Enabled:                   true,
		IntervalSeconds:           dnsSvc.DefaultDDNSInterval(),
		IPVersion:                 "ipv4_ipv6",
		CleanupConflictingRecords: true,
		RecordIDs:                 []string{"a-record"},
	})
	require.NoError(t, err)
	require.Len(t, cfg.Targets, 2)

	ids := []string{cfg.Targets[0].ID, cfg.Targets[1].ID}
	require.ElementsMatch(t, []string{"a-record", "aaaa-record"}, ids)
	require.Empty(t, getMockCreatedRecords(), "no records should have been created (auto-pair only)")
}
