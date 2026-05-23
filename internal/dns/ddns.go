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
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy"
)

const (
	minDDNSIntervalSeconds     = 60
	defaultDDNSIntervalSeconds = 300
	ipDetectTimeout            = 8 * time.Second

	// DDNS IP version selectors persisted in DDNSConfig.IPVersion and accepted by the API.
	DDNSIPVersionIPv4     = "ipv4"
	DDNSIPVersionIPv6     = "ipv6"
	DDNSIPVersionIPv4IPv6 = "ipv4_ipv6"
	DDNSIPVersionIPv6IPv4 = "ipv6_ipv4"
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

type ipFamily int

const (
	ipFamilyAny ipFamily = iota
	ipFamilyV4
	ipFamilyV6
)

// DefaultDDNSInterval returns the default polling interval in seconds.
func DefaultDDNSInterval() int {
	return defaultDDNSIntervalSeconds
}

// DDNSUpdateInput carries payload for updating a DDNS configuration.
type DDNSUpdateInput struct {
	Enabled                   bool
	IntervalSeconds           int
	IPVersion                 string
	CleanupConflictingRecords bool
	RecordIDs                 []string
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

	if domain.DDNSConfig == nil {
		return &model.DDNSConfig{
			Enabled:         false,
			IntervalSeconds: defaultDDNSIntervalSeconds,
			IPVersion:       DDNSIPVersionIPv4IPv6,
			Targets:         []model.DDNSRecordTarget{},
		}, nil
	}

	cfg := *domain.DDNSConfig
	cfg.IntervalSeconds = sanitizeInterval(cfg.IntervalSeconds)
	cfg.IPVersion = NormalizeDDNSIPVersion(cfg.IPVersion)
	cfg.Targets = append([]model.DDNSRecordTarget(nil), cfg.Targets...)
	return &cfg, nil
}

// UpdateDDNSConfig validates and persists DDNS configuration for the given domain.
func (s *Service) UpdateDDNSConfig(ctx context.Context, domainID uint64, input DDNSUpdateInput) (*model.DDNSConfig, error) {
	interval := sanitizeInterval(input.IntervalSeconds)
	if interval < minDDNSIntervalSeconds {
		return nil, ErrInvalidDDNSInterval
	}
	version, err := sanitizeDDNSIPVersion(input.IPVersion)
	if err != nil {
		return nil, err
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
		recordsByName := indexRecordsByName(records)

		seen := map[string]struct{}{}
		seenTargetIDs := map[string]struct{}{}
		for _, id := range input.RecordIDs {
			trimmed := strings.TrimSpace(id)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			if record, ok := recordMap[trimmed]; ok {
				recordType := strings.ToUpper(record.Type)
				if recordType != "A" && recordType != "AAAA" {
					return nil, cosy.WrapErrorWithParams(ErrInvalidDDNSTargetType, recordType)
				}
				if !ddnsIPVersionMatchesRecordType(version, recordType) {
					return nil, cosy.WrapErrorWithParams(ErrDDNSIPVersionRecordMismatch, recordType, version)
				}
				if _, ok := seenTargetIDs[record.ID]; !ok {
					targets = append(targets, model.DDNSRecordTarget{
						ID:   record.ID,
						Name: record.Name,
						Type: recordType,
					})
					seenTargetIDs[record.ID] = struct{}{}
				}
				seen[trimmed] = struct{}{}
				continue
			}

			if namedRecords := recordsByName[strings.ToLower(trimmed)]; len(namedRecords) > 0 {
				createdTargets, err := collectDDNSTargetsForNamedRecords(namedRecords, version, seenTargetIDs)
				if err != nil {
					return nil, err
				}
				targets = append(targets, createdTargets...)
				seen[trimmed] = struct{}{}
				continue
			}

			// If record does not exist, create new A/AAAA records using detected IPs.
			if ipSnapshot == nil {
				snapshot, ipErr := resolvePublicIPs(ctx, version)
				if ipErr != nil {
					return nil, ipErr
				}
				ipSnapshot = &snapshot
			}

			createdTargets, err := createDDNSRecordsForMissingName(ctx, provider, domain.Domain, trimmed, version, *ipSnapshot)
			if err != nil {
				return nil, err
			}

			targets = append(targets, createdTargets...)
			seen[trimmed] = struct{}{}
		}

		if len(targets) == 0 {
			return nil, ErrDDNSTargetRequired
		}
	}

	cfg := &model.DDNSConfig{
		Enabled:         input.Enabled,
		IntervalSeconds: interval,
		IPVersion:       version,
		Targets:         targets,
	}

	if existing != nil {
		cfg.LastIPv4 = existing.LastIPv4
		cfg.LastIPv6 = existing.LastIPv6
		cfg.LastRunAt = existing.LastRunAt
		cfg.LastError = existing.LastError
	}

	if err := saveDDNSConfig(ctx, domainID, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// DeleteDDNSConfig removes DDNS configuration for the given domain.
func (s *Service) DeleteDDNSConfig(ctx context.Context, domainID uint64) error {
	if _, err := loadDomain(ctx, domainID); err != nil {
		return err
	}

	return model.UseDB().WithContext(ctx).
		Model(&model.DnsDomain{}).
		Where("id = ?", domainID).
		Update("ddns_config", nil).Error
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
	version := NormalizeDDNSIPVersion(cfg.IPVersion)
	policy := getDDNSIPVersionPolicy(version)

	ipSnapshot, ipErr := resolvePublicIPs(ctx, version)

	records, err := fetchProviderRecords(ctx, provider, domain.Domain)
	if err != nil {
		return err
	}
	recordMap := indexRecordsByID(records)

	updateErrs := append([]string(nil), ipSnapshot.Warnings...)

	now := time.Now()

	for _, target := range cfg.Targets {
		record, ok := recordMap[target.ID]
		if !ok {
			updateErrs = append(updateErrs, fmt.Sprintf("record %s missing", target.ID))
			continue
		}

		recordType := strings.ToUpper(record.Type)
		if _, ok := policy.recordTypes[recordType]; !ok {
			updateErrs = append(updateErrs, fmt.Sprintf("record %s has type %s outside selected IP version %s", record.ID, recordType, version))
			continue
		}

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
			Comment:  record.Comment,
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
	IPv4     string
	IPv6     string
	Warnings []string
}

func resolvePublicIPs(ctx context.Context, ipVersion string) (ipSnapshot, error) {
	version, err := sanitizeDDNSIPVersion(ipVersion)
	if err != nil {
		return ipSnapshot{}, err
	}
	policy := getDDNSIPVersionPolicy(version)

	ipCtx, cancel := context.WithTimeout(ctx, ipDetectTimeout)
	defer cancel()

	// Probe each family concurrently so a slow IPv6 lookup does not delay IPv4.
	var (
		wg               sync.WaitGroup
		ipv4Res, ipv6Res string
		ipv4Err, ipv6Err error
	)
	for _, family := range policy.families {
		switch family {
		case ipFamilyV4:
			wg.Add(1)
			go func() {
				defer wg.Done()
				ipv4Res, ipv4Err = fetchAnyIP(ipCtx, ipv4Endpoints, ipFamilyV4)
			}()
		case ipFamilyV6:
			wg.Add(1)
			go func() {
				defer wg.Done()
				ipv6Res, ipv6Err = fetchAnyIP(ipCtx, ipv6Endpoints, ipFamilyV6)
			}()
		}
	}
	wg.Wait()

	var snapshot ipSnapshot
	var errs []string
	var successCount int
	for _, family := range policy.families {
		switch family {
		case ipFamilyV4:
			if ipv4Err == nil {
				snapshot.IPv4 = ipv4Res
				successCount++
			} else {
				errs = append(errs, fmt.Sprintf("ipv4: %v", ipv4Err))
			}
		case ipFamilyV6:
			if ipv6Err == nil {
				snapshot.IPv6 = ipv6Res
				successCount++
			} else {
				errs = append(errs, fmt.Sprintf("ipv6: %v", ipv6Err))
			}
		}
	}

	if successCount == 0 && len(errs) > 0 {
		return snapshot, fmt.Errorf("public ip resolve errors: %s", strings.Join(errs, "; "))
	}
	if len(errs) > 0 {
		snapshot.Warnings = errs
	}

	return snapshot, nil
}

func fetchIP(ctx context.Context, endpoint string, family ipFamily) (string, error) {
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
	parsed, err := parseIPString(ipStr, family)
	if err != nil {
		return "", fmt.Errorf("invalid ip from %s: %v", endpoint, err)
	}
	return parsed, nil
}

func fetchAnyIP(ctx context.Context, endpoints []string, family ipFamily) (string, error) {
	var errs []string
	for _, ep := range endpoints {
		if ip, err := fetchIP(ctx, ep, family); err == nil {
			return ip, nil
		} else {
			errs = append(errs, fmt.Sprintf("%s: %v", ep, err))
		}
	}
	return "", errors.New(strings.Join(errs, "; "))
}

func parseIPString(val string, family ipFamily) (string, error) {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return "", fmt.Errorf("empty ip string")
	}

	if ip := parseExpectedIP(trimmed, family); ip != nil {
		return ip.String(), nil
	}

	for _, token := range strings.Fields(trimmed) {
		cleaned := strings.Trim(token, " ,;[](){}<>")
		if ip := parseExpectedIP(cleaned, family); ip != nil {
			return ip.String(), nil
		}
	}

	if matches := ipRegex.FindAllString(trimmed, -1); len(matches) > 0 {
		for _, candidate := range matches {
			if ip := parseExpectedIP(candidate, family); ip != nil {
				return ip.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no valid ip found")
}

func parseExpectedIP(val string, family ipFamily) net.IP {
	ip := net.ParseIP(val)
	if ip == nil {
		return nil
	}

	switch family {
	case ipFamilyV4:
		if ip.To4() == nil {
			return nil
		}
	case ipFamilyV6:
		if ip.To4() != nil {
			return nil
		}
	case ipFamilyAny:
	default:
		return nil
	}

	return ip
}

func sanitizeInterval(value int) int {
	if value < minDDNSIntervalSeconds {
		return minDDNSIntervalSeconds
	}
	return value
}

func sanitizeDDNSIPVersion(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DDNSIPVersionIPv4:
		return DDNSIPVersionIPv4, nil
	case DDNSIPVersionIPv6:
		return DDNSIPVersionIPv6, nil
	case DDNSIPVersionIPv4IPv6:
		return DDNSIPVersionIPv4IPv6, nil
	case DDNSIPVersionIPv6IPv4:
		return DDNSIPVersionIPv6IPv4, nil
	default:
		return "", ErrInvalidDDNSIPVersion
	}
}

// NormalizeDDNSIPVersion returns a canonical IP version selector, defaulting to
// DDNSIPVersionIPv4IPv6 for empty or unrecognized values.
func NormalizeDDNSIPVersion(value string) string {
	version, err := sanitizeDDNSIPVersion(value)
	if err != nil {
		return DDNSIPVersionIPv4IPv6
	}
	return version
}

type ddnsIPVersionPolicy struct {
	families    []ipFamily
	recordTypes map[string]struct{}
}

func getDDNSIPVersionPolicy(version string) ddnsIPVersionPolicy {
	switch version {
	case DDNSIPVersionIPv4:
		return ddnsIPVersionPolicy{
			families:    []ipFamily{ipFamilyV4},
			recordTypes: map[string]struct{}{"A": {}},
		}
	case DDNSIPVersionIPv6:
		return ddnsIPVersionPolicy{
			families:    []ipFamily{ipFamilyV6},
			recordTypes: map[string]struct{}{"AAAA": {}},
		}
	case DDNSIPVersionIPv6IPv4:
		return ddnsIPVersionPolicy{
			families:    []ipFamily{ipFamilyV6, ipFamilyV4},
			recordTypes: map[string]struct{}{"A": {}, "AAAA": {}},
		}
	default:
		return ddnsIPVersionPolicy{
			families:    []ipFamily{ipFamilyV4, ipFamilyV6},
			recordTypes: map[string]struct{}{"A": {}, "AAAA": {}},
		}
	}
}

func ddnsIPVersionMatchesRecordType(ipVersion string, recordType string) bool {
	policy := getDDNSIPVersionPolicy(ipVersion)
	_, ok := policy.recordTypes[strings.ToUpper(recordType)]
	return ok
}

func collectDDNSTargetsForNamedRecords(records []Record, version string, seenTargetIDs map[string]struct{}) ([]model.DDNSRecordTarget, error) {
	targets := make([]model.DDNSRecordTarget, 0, len(records))
	for _, record := range records {
		recordType := strings.ToUpper(record.Type)
		if recordType != "A" && recordType != "AAAA" {
			continue
		}
		if !ddnsIPVersionMatchesRecordType(version, recordType) {
			continue
		}
		if _, ok := seenTargetIDs[record.ID]; ok {
			continue
		}

		targets = append(targets, model.DDNSRecordTarget{
			ID:   record.ID,
			Name: record.Name,
			Type: recordType,
		})
		seenTargetIDs[record.ID] = struct{}{}
	}

	if len(targets) == 0 {
		return nil, cosy.WrapErrorWithParams(ErrDDNSIPVersionRecordMismatch, records[0].Name, version)
	}
	return targets, nil
}

func createDDNSRecordsForMissingName(ctx context.Context, provider Provider, domain string, name string, version string, snapshot ipSnapshot) ([]model.DDNSRecordTarget, error) {
	// Best-effort: create a record for the first family in the policy whose IP is detected.
	for _, family := range getDDNSIPVersionPolicy(version).families {
		switch family {
		case ipFamilyV4:
			if snapshot.IPv4 != "" {
				return createDDNSRecordTargets(ctx, provider, domain, name, []RecordInput{
					{Type: "A", Name: name, Content: snapshot.IPv4, TTL: 600},
				})
			}
		case ipFamilyV6:
			if snapshot.IPv6 != "" {
				return createDDNSRecordTargets(ctx, provider, domain, name, []RecordInput{
					{Type: "AAAA", Name: name, Content: snapshot.IPv6, TTL: 600},
				})
			}
		}
	}

	return nil, ErrDDNSIPUnavailable
}

func createDDNSRecordTargets(ctx context.Context, provider Provider, domain string, name string, inputs []RecordInput) ([]model.DDNSRecordTarget, error) {
	targets := make([]model.DDNSRecordTarget, 0, len(inputs))
	for _, input := range inputs {
		newRecord, err := provider.CreateRecord(ctx, domain, sanitizeRecordInput(input))
		if err != nil {
			rollbackCreatedDDNSRecords(ctx, provider, domain, targets)
			return nil, cosy.WrapErrorWithParams(ErrDDNSRecordNotFound, name)
		}
		targets = append(targets, model.DDNSRecordTarget{
			ID:   newRecord.ID,
			Name: newRecord.Name,
			Type: strings.ToUpper(newRecord.Type),
		})
	}
	return targets, nil
}

func rollbackCreatedDDNSRecords(ctx context.Context, provider Provider, domain string, targets []model.DDNSRecordTarget) {
	for _, target := range targets {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, providerTimeout)
		_ = provider.DeleteRecord(ctxWithTimeout, domain, target.ID)
		cancel()
	}
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

func indexRecordsByName(records []Record) map[string][]Record {
	result := make(map[string][]Record, len(records))
	for _, record := range records {
		name := strings.ToLower(record.Name)
		result[name] = append(result[name], record)
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
