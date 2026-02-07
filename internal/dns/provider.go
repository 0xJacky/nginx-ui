package dns

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Record represents a DNS record managed through a provider.
type Record struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority,omitempty"`
	Weight   *int   `json:"weight,omitempty"`
	Proxied  *bool  `json:"proxied,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

// RecordFilter allows narrowing down provider queries.
type RecordFilter struct {
	Type string
	Name string
}

// RecordInput represents the payload required to create or update a DNS record.
type RecordInput struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority,omitempty"`
	Weight   *int   `json:"weight,omitempty"`
	Proxied  *bool  `json:"proxied,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

// Credential holds the secrets and metadata bound to a DNS provider.
type Credential struct {
	ID         uint64
	Name       string
	Provider   string
	Code       string
	Values     map[string]string
	Additional map[string]string
}

// Provider describes the CRUD contract every DNS vendor must implement.
type Provider interface {
	ListRecords(ctx context.Context, domain string, filter RecordFilter) ([]Record, error)
	CreateRecord(ctx context.Context, domain string, input RecordInput) (Record, error)
	UpdateRecord(ctx context.Context, domain string, recordID string, input RecordInput) (Record, error)
	DeleteRecord(ctx context.Context, domain string, recordID string) error
}

// Factory constructs a provider using the given credential.
type Factory func(*Credential) (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}

	// ErrProviderNotRegistered indicates that the requested provider is not available.
	ErrProviderNotRegistered = errors.New("dns provider not registered")
)

// RegisterProvider registers a provider factory for a given provider code.
func RegisterProvider(code string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()

	key := strings.ToLower(code)
	registry[key] = factory
}

// NewProvider returns an initialized provider for the given credential code.
func NewProvider(code string, credential *Credential) (Provider, error) {
	registryMu.RLock()
	factory, ok := registry[strings.ToLower(code)]
	registryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotRegistered, code)
	}

	return factory(credential)
}

