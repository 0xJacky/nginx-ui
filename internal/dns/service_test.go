package dns_test

import (
	"context"
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

	q := setupTestQuery(t)
	ctx := context.Background()
	service := dnsSvc.NewService(q)

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
	return []dnsSvc.Record{{
		ID:      "1",
		Type:    "mock",
		Name:    "@",
		Content: "127.0.0.1",
		TTL:     60,
	}}, nil
}

func (m *mockProvider) CreateRecord(ctx context.Context, domain string, input dnsSvc.RecordInput) (dnsSvc.Record, error) {
	return dnsSvc.Record{
		ID:      "1",
		Type:    input.Type,
		Name:    input.Name,
		Content: input.Content,
		TTL:     input.TTL,
	}, nil
}

func (m *mockProvider) UpdateRecord(ctx context.Context, domain string, recordID string, input dnsSvc.RecordInput) (dnsSvc.Record, error) {
	return dnsSvc.Record{
		ID:      recordID,
		Type:    input.Type,
		Name:    input.Name,
		Content: input.Content,
		TTL:     input.TTL,
	}, nil
}

func (m *mockProvider) DeleteRecord(ctx context.Context, domain string, recordID string) error {
	return nil
}



