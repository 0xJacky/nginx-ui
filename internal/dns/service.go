package dns

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/samber/lo"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
)

const providerTimeout = 10 * time.Second

var domainPattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9-]{2,}$`)

// DomainInput represents the payload required to create or update a domain.
type DomainInput struct {
	Domain          string
	Description     string
	DnsCredentialID uint64
}

// DomainListOptions controls domain pagination and filtering.
type DomainListOptions struct {
	Page          int
	PerPage       int
	Keyword       string
	DnsCredential uint64
}

// RecordListOptions wraps filters for listing DNS records.
type RecordListOptions struct {
	Filter RecordFilter
}

// Service implements domain and record operations.
type Service struct{}

// NewService builds a DNS service.
func NewService() *Service {
	return &Service{}
}

// ListDomains returns paginated domains with their credentials.
func (s *Service) ListDomains(ctx context.Context, opts DomainListOptions) ([]*model.DnsDomain, int64, error) {
	page := lo.If(opts.Page < 1, 1).Else(opts.Page)
	perPage := lo.If(opts.PerPage <= 0, 50).Else(opts.PerPage)
	offset := (page - 1) * perPage

	dao := query.DnsDomain
	d := dao.WithContext(ctx).Preload(dao.DnsCredential)

	if opts.DnsCredential > 0 {
		d = d.Where(dao.DnsCredentialID.Eq(opts.DnsCredential))
	}

	if opts.Keyword != "" {
		keyword := "%" + opts.Keyword + "%"
		d = d.Where(dao.Domain.Like(keyword)).
			Or(dao.Description.Like(keyword))
	}

	return d.Order(dao.UpdatedAt.Desc()).FindByPage(offset, perPage)
}

// CreateDomain stores a new domain bound to a credential.
func (s *Service) CreateDomain(ctx context.Context, input DomainInput) (*model.DnsDomain, error) {
	validDomain, err := normalizeDomain(input.Domain)
	if err != nil {
		return nil, err
	}

	cred, err := loadCredential(ctx, input.DnsCredentialID)
	if err != nil {
		return nil, err
	}

	if err := ensureDomainUnique(ctx, validDomain, input.DnsCredentialID, 0); err != nil {
		return nil, err
	}

	domain := &model.DnsDomain{
		Domain:          validDomain,
		Description:     strings.TrimSpace(input.Description),
		DnsCredentialID: cred.ID,
	}

	if err := query.DnsDomain.WithContext(ctx).Create(domain); err != nil {
		return nil, err
	}

	domain.DnsCredential = cred
	return domain, nil
}

// GetDomain returns a single domain with credential information.
func (s *Service) GetDomain(ctx context.Context, id uint64) (*model.DnsDomain, error) {
	return loadDomain(ctx, id)
}

// UpdateDomain updates domain metadata and credential association.
func (s *Service) UpdateDomain(ctx context.Context, id uint64, input DomainInput) (*model.DnsDomain, error) {
	domain, err := loadDomain(ctx, id)
	if err != nil {
		return nil, err
	}

	newDomain, err := normalizeDomain(input.Domain)
	if err != nil {
		return nil, err
	}

	cred, err := loadCredential(ctx, input.DnsCredentialID)
	if err != nil {
		return nil, err
	}

	if err := ensureDomainUnique(ctx, newDomain, cred.ID, domain.ID); err != nil {
		return nil, err
	}

	_, err = query.DnsDomain.WithContext(ctx).
		Where(query.DnsDomain.ID.Eq(domain.ID)).
		Updates(&model.DnsDomain{
			Domain:          newDomain,
			Description:     strings.TrimSpace(input.Description),
			DnsCredentialID: cred.ID,
		})
	if err != nil {
		return nil, err
	}

	return loadDomain(ctx, id)
}

// DeleteDomain removes the domain entry.
func (s *Service) DeleteDomain(ctx context.Context, id uint64) error {
	_, err := query.DnsDomain.WithContext(ctx).Where(query.DnsDomain.ID.Eq(id)).Delete()
	return err
}

// ListRecords lists DNS records for the given domain.
func (s *Service) ListRecords(ctx context.Context, domainID uint64, opts RecordListOptions) ([]Record, error) {
	domain, provider, err := s.prepareProvider(ctx, domainID)
	if err != nil {
		return nil, err
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	return provider.ListRecords(ctxWithTimeout, domain.Domain, opts.Filter)
}

// CreateRecord creates a DNS record under the domain.
func (s *Service) CreateRecord(ctx context.Context, domainID uint64, input RecordInput) (Record, error) {
	domain, provider, err := s.prepareProvider(ctx, domainID)
	if err != nil {
		return Record{}, err
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	return provider.CreateRecord(ctxWithTimeout, domain.Domain, sanitizeRecordInput(input))
}

// UpdateRecord updates a DNS record.
func (s *Service) UpdateRecord(ctx context.Context, domainID uint64, recordID string, input RecordInput) (Record, error) {
	domain, provider, err := s.prepareProvider(ctx, domainID)
	if err != nil {
		return Record{}, err
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	return provider.UpdateRecord(ctxWithTimeout, domain.Domain, recordID, sanitizeRecordInput(input))
}

// DeleteRecord deletes a DNS record.
func (s *Service) DeleteRecord(ctx context.Context, domainID uint64, recordID string) error {
	domain, provider, err := s.prepareProvider(ctx, domainID)
	if err != nil {
		return err
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	return provider.DeleteRecord(ctxWithTimeout, domain.Domain, recordID)
}

func (s *Service) prepareProvider(ctx context.Context, domainID uint64) (*model.DnsDomain, Provider, error) {
	domain, err := loadDomain(ctx, domainID)
	if err != nil {
		return nil, nil, err
	}

	cred := domain.DnsCredential
	if cred == nil {
		return nil, nil, ErrCredentialNotFound
	}

	providerCred, err := toProviderCredential(cred)
	if err != nil {
		return nil, nil, err
	}

	provider, err := NewProvider(providerCred.Code, providerCred)
	if err != nil {
		return nil, nil, err
	}

	return domain, provider, nil
}

func loadCredential(ctx context.Context, id uint64) (*model.DnsCredential, error) {
	credential, err := query.DnsCredential.WithContext(ctx).FirstByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, err
	}

	return credential, nil
}

func loadDomain(ctx context.Context, id uint64) (*model.DnsDomain, error) {
	dao := query.DnsDomain
	domain, err := dao.WithContext(ctx).
		Preload(dao.DnsCredential).
		Where(dao.ID.Eq(id)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}

	return domain, nil
}

func ensureDomainUnique(ctx context.Context, domain string, credentialID uint64, excludeID uint64) error {
	dao := query.DnsDomain
	d := dao.WithContext(ctx).Where(
		dao.DnsCredentialID.Eq(credentialID),
		dao.Domain.Eq(domain),
	)

	if excludeID > 0 {
		d = d.Where(dao.ID.Neq(excludeID))
	}

	count, err := d.Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrDuplicateDomain
	}

	return nil
}

func normalizeDomain(value string) (string, error) {
	domain := strings.Trim(strings.ToLower(value), ".")
	if domain == "" || !domainPattern.MatchString(domain) {
		return "", cosy.WrapErrorWithParams(ErrInvalidDomain, value)
	}
	return domain, nil
}

func toProviderCredential(credential *model.DnsCredential) (*Credential, error) {
	if credential == nil || credential.Config == nil || credential.Config.Configuration == nil {
		return nil, ErrInvalidCredential
	}

	values := make(map[string]string)
	for key, val := range credential.Config.Configuration.Credentials {
		if trimmed := strings.TrimSpace(val); trimmed != "" {
			values[key] = trimmed
		}
	}
	if len(values) == 0 {
		return nil, ErrInvalidCredential
	}

	additional := make(map[string]string)
	for key, val := range credential.Config.Configuration.Additional {
		if trimmed := strings.TrimSpace(val); trimmed != "" {
			additional[key] = trimmed
		}
	}

	return &Credential{
		ID:         credential.ID,
		Name:       credential.Name,
		Provider:   credential.Provider,
		Code:       credential.Config.Code,
		Values:     values,
		Additional: additional,
	}, nil
}

func sanitizeRecordInput(input RecordInput) RecordInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Content = strings.TrimSpace(input.Content)
	input.Type = strings.ToUpper(strings.TrimSpace(input.Type))
	if input.TTL <= 0 {
		input.TTL = 600
	}
	return input
}
