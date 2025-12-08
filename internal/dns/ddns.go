package dns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy"
)

const (
	minDDNSIntervalSeconds     = 60
	defaultDDNSIntervalSeconds = 300
	ipDetectTimeout            = 8 * time.Second
)

var (
	ipv4Endpoints = []string{
		"https://api.ipify.org",
		"https://ipv4.icanhazip.com",
		"https://v4.ident.me",
		"https://ipv4.ifconfig.co",
		"https://myip.ipip.net",
	}
	ipv6Endpoints = []string{
		"https://api64.ipify.org",
		"https://ipv6.icanhazip.com",
		"https://v6.ident.me",
		"https://ipv6.ifconfig.co",
		"https://myip.ipip.net",
	}
	ipRegex = regexp.MustCompile(`(?i)(?:[0-9a-f]{0,4}:){2,7}[0-9a-f]{0,4}|(?:\d{1,3}\.){3}\d{1,3}`)
)

// DefaultDDNSInterval returns the default polling interval in seconds.
func DefaultDDNSInterval() int {
	return defaultDDNSIntervalSeconds
}

// DDNSUpdateInput carries payload for updating a DDNS configuration.
type DDNSUpdateInput struct {
	Enabled         bool
	IntervalSeconds int
	RecordIDs       []string
}

// DDNSSchedule describes an enabled DDNS task.
type DDNSSchedule struct {
	DomainID        uint64
	IntervalSeconds int
}

// GetDDNSConfig returns the current DDNS configuration for a domain with defaults applied.
func (s *Service) GetDDNSConfig(ctx context.Context, domainID uint64) (*model.DDNSConfig, error) {
	domain, err := loadDomain(ctx, domainID)
	if err != nil {
		return nil, err
	}

	cfg := domain.DDNSConfig
	if cfg == nil {
		return &model.DDNSConfig{
			Enabled:         false,
			IntervalSeconds: defaultDDNSIntervalSeconds,
			Targets:         []model.DDNSRecordTarget{},
		}, nil
	}

	cfg.IntervalSeconds = sanitizeInterval(cfg.IntervalSeconds)
	return cfg, nil
}

// UpdateDDNSConfig validates and persists DDNS configuration for the given domain.
func (s *Service) UpdateDDNSConfig(ctx context.Context, domainID uint64, input DDNSUpdateInput) (*model.DDNSConfig, error) {
	interval := sanitizeInterval(input.IntervalSeconds)
	if interval < minDDNSIntervalSeconds {
		return nil, ErrInvalidDDNSInterval
	}

	domain, provider, err := s.prepareProvider(ctx, domainID)
	if err != nil {
		return nil, err
	}

	existing := domain.DDNSConfig

	targets := []model.DDNSRecordTarget{}
	var ipSnapshot *ipSnapshot
	if input.Enabled {
		if len(input.RecordIDs) == 0 {
			return nil, ErrDDNSTargetRequired
		}

		records, err := fetchProviderRecords(ctx, provider, domain.Domain)
		if err != nil {
			return nil, err
		}
		recordMap := indexRecordsByID(records)
		recordByName := indexRecordsByName(records)

		seen := map[string]struct{}{}
		for _, id := range input.RecordIDs {
			trimmed := strings.TrimSpace(id)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			record, ok := recordMap[trimmed]
			if !ok {
				record, ok = recordByName[strings.ToLower(trimmed)]
			}

			if ok {
				recordType := strings.ToUpper(record.Type)
				if recordType != "A" && recordType != "AAAA" {
					return nil, cosy.WrapErrorWithParams(ErrInvalidDDNSTargetType, recordType)
				}
				targets = append(targets, model.DDNSRecordTarget{
					ID:   record.ID,
					Name: record.Name,
					Type: recordType,
				})
				seen[trimmed] = struct{}{}
				continue
			}

			// If record does not exist, create a new A/AAAA record using detected IPs.
			if ipSnapshot == nil {
				snapshot, ipErr := resolvePublicIPs(ctx)
				if ipErr != nil {
					return nil, ipErr
				}
				ipSnapshot = &snapshot
			}

			recordType := "A"
			content := ipSnapshot.IPv4
			if content == "" && ipSnapshot.IPv6 != "" {
				recordType = "AAAA"
				content = ipSnapshot.IPv6
			}
			if content == "" {
				return nil, ErrDDNSIPUnavailable
			}

			newRecord, err := provider.CreateRecord(ctx, domain.Domain, sanitizeRecordInput(RecordInput{
				Type:    recordType,
				Name:    trimmed,
				Content: content,
				TTL:     600,
			}))
			if err != nil {
				return nil, cosy.WrapErrorWithParams(ErrDDNSRecordNotFound, trimmed)
			}

			targets = append(targets, model.DDNSRecordTarget{
				ID:   newRecord.ID,
				Name: newRecord.Name,
				Type: strings.ToUpper(newRecord.Type),
			})
			seen[trimmed] = struct{}{}
		}

		if len(targets) == 0 {
			return nil, ErrDDNSTargetRequired
		}
	}

	cfg := &model.DDNSConfig{
		Enabled:         input.Enabled,
		IntervalSeconds: interval,
		Targets:         targets,
	}

	if existing != nil {
		cfg.LastIPv4 = existing.LastIPv4
		cfg.LastIPv6 = existing.LastIPv6
		cfg.LastRunAt = existing.LastRunAt
		cfg.LastError = existing.LastError

		// Reset runtime error when disabling
		if !input.Enabled {
			cfg.LastError = ""
		}
	}

	if err := saveDDNSConfig(ctx, domainID, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ListEnabledDDNSSchedules returns schedules for enabled DDNS domains.
func ListEnabledDDNSSchedules(ctx context.Context) ([]DDNSSchedule, error) {
	domains, err := query.DnsDomain.WithContext(ctx).Find()
	if err != nil {
		return nil, err
	}

	schedules := make([]DDNSSchedule, 0, len(domains))
	for _, domain := range domains {
		cfg := domain.DDNSConfig
		if cfg == nil || !cfg.Enabled || len(cfg.Targets) == 0 {
			continue
		}
		schedules = append(schedules, DDNSSchedule{
			DomainID:        domain.ID,
			IntervalSeconds: sanitizeInterval(cfg.IntervalSeconds),
		})
	}

	return schedules, nil
}

// ListDDNSDomains returns all domains with DDNS configuration for overview pages.
func ListDDNSDomains(ctx context.Context) ([]*model.DnsDomain, error) {
	dao := query.DnsDomain
	return dao.WithContext(ctx).
		Preload(dao.DnsCredential).
		Find()
}

// RunDDNSUpdate executes DDNS update for the given domain ID.
func RunDDNSUpdate(ctx context.Context, domainID uint64) error {
	return NewService().runDDNSUpdate(ctx, domainID)
}

func (s *Service) runDDNSUpdate(ctx context.Context, domainID uint64) error {
	domain, provider, err := s.prepareProvider(ctx, domainID)
	if err != nil {
		return err
	}

	cfg := domain.DDNSConfig
	if cfg == nil || !cfg.Enabled || len(cfg.Targets) == 0 {
		return nil
	}

	cfg.IntervalSeconds = sanitizeInterval(cfg.IntervalSeconds)

	ipSnapshot, ipErr := resolvePublicIPs(ctx)
	if ipErr != nil {
		// keep running; errors will be recorded
	}

	records, err := fetchProviderRecords(ctx, provider, domain.Domain)
	if err != nil {
		return err
	}
	recordMap := indexRecordsByID(records)

	var updateErrs []string

	now := time.Now()

	for _, target := range cfg.Targets {
		record, ok := recordMap[target.ID]
		if !ok {
			updateErrs = append(updateErrs, fmt.Sprintf("record %s missing", target.ID))
			continue
		}

		recordType := strings.ToUpper(record.Type)
		var nextIP string
		switch recordType {
		case "A":
			nextIP = ipSnapshot.IPv4
			if nextIP != "" {
				cfg.LastIPv4 = nextIP
			}
		case "AAAA":
			nextIP = ipSnapshot.IPv6
			if nextIP != "" {
				cfg.LastIPv6 = nextIP
			}
		default:
			updateErrs = append(updateErrs, fmt.Sprintf("record %s has unsupported type %s", target.ID, recordType))
			continue
		}

		if nextIP == "" {
			updateErrs = append(updateErrs, fmt.Sprintf("record %s missing detected IP", target.ID))
			continue
		}

		if record.Content == nextIP {
			continue
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, providerTimeout)
		_, err = provider.UpdateRecord(ctxWithTimeout, domain.Domain, record.ID, sanitizeRecordInput(RecordInput{
			Type:     recordType,
			Name:     record.Name,
			Content:  nextIP,
			TTL:      record.TTL,
			Priority: record.Priority,
			Weight:   record.Weight,
			Proxied:  record.Proxied,
		}))
		cancel()

		if err != nil {
			updateErrs = append(updateErrs, fmt.Sprintf("%s: %v", record.ID, err))
		}
	}

	cfg.LastRunAt = &now
	if ipErr != nil {
		updateErrs = append(updateErrs, ipErr.Error())
	}

	cfg.LastError = strings.Join(updateErrs, "; ")

	return saveDDNSConfig(ctx, domainID, cfg)
}

type ipSnapshot struct {
	IPv4 string
	IPv6 string
}

func resolvePublicIPs(ctx context.Context) (ipSnapshot, error) {
	var snapshot ipSnapshot
	var errs []string

	ipCtx, cancel := context.WithTimeout(ctx, ipDetectTimeout)
	defer cancel()

	if ip, err := fetchAnyIP(ipCtx, ipv4Endpoints); err == nil {
		snapshot.IPv4 = ip
	} else {
		errs = append(errs, fmt.Sprintf("ipv4: %v", err))
	}

	if ip, err := fetchAnyIP(ipCtx, ipv6Endpoints); err == nil {
		snapshot.IPv6 = ip
	} else {
		errs = append(errs, fmt.Sprintf("ipv6: %v", err))
	}

	if len(errs) > 0 {
		return snapshot, fmt.Errorf("public ip resolve errors: %s", strings.Join(errs, "; "))
	}

	return snapshot, nil
}

func fetchIP(ctx context.Context, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	q := req.URL.Query()
	q.Set("format", "text")
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return "", err
	}

	ipStr := strings.TrimSpace(string(body))
	parsed, err := parseIPString(ipStr)
	if err != nil {
		return "", fmt.Errorf("invalid ip from %s: %v", endpoint, err)
	}
	return parsed, nil
}

func fetchAnyIP(ctx context.Context, endpoints []string) (string, error) {
	var errs []string
	for _, ep := range endpoints {
		if ip, err := fetchIP(ctx, ep); err == nil {
			return ip, nil
		} else {
			errs = append(errs, fmt.Sprintf("%s: %v", ep, err))
		}
	}
	return "", fmt.Errorf(strings.Join(errs, "; "))
}

func parseIPString(val string) (string, error) {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return "", fmt.Errorf("empty ip string")
	}

	if ip := net.ParseIP(trimmed); ip != nil {
		return ip.String(), nil
	}

	for _, token := range strings.Fields(trimmed) {
		cleaned := strings.Trim(token, " ,;[](){}<>")
		if ip := net.ParseIP(cleaned); ip != nil {
			return ip.String(), nil
		}
	}

	if matches := ipRegex.FindAllString(trimmed, -1); len(matches) > 0 {
		for _, candidate := range matches {
			if ip := net.ParseIP(candidate); ip != nil {
				return ip.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no valid ip found")
}

func sanitizeInterval(value int) int {
	if value < minDDNSIntervalSeconds {
		return minDDNSIntervalSeconds
	}
	return value
}

func fetchProviderRecords(ctx context.Context, provider Provider, domain string) ([]Record, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()
	return provider.ListRecords(ctxWithTimeout, domain, RecordFilter{})
}

func indexRecordsByID(records []Record) map[string]Record {
	result := make(map[string]Record, len(records))
	for _, record := range records {
		result[record.ID] = record
	}
	return result
}

func indexRecordsByName(records []Record) map[string]Record {
	result := make(map[string]Record, len(records))
	for _, record := range records {
		result[strings.ToLower(record.Name)] = record
	}
	return result
}

func saveDDNSConfig(ctx context.Context, domainID uint64, cfg *model.DDNSConfig) error {
	return model.UseDB().WithContext(ctx).
		Model(&model.DnsDomain{}).
		Where("id = ?", domainID).
		Updates(&model.DnsDomain{
			DDNSConfig: cfg,
		}).Error
}
