package huaweicloud

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

type recordFields struct {
	id       string
	name     string
	zoneName string
	typeName string
	ttl      int32
	values   []string
	line     string
	weight   *int32
}

func recordFromFields(fields recordFields) dns.Record {
	recordType := strings.ToUpper(strings.TrimSpace(fields.typeName))
	content, priority, srvWeight := recordContent(recordType, fields.values)
	record := dns.Record{
		ID:       fields.id,
		Name:     relativeRecordName(fields.name, fields.zoneName),
		Type:     recordType,
		Content:  content,
		TTL:      int(fields.ttl),
		Line:     firstNonEmpty(fields.line, defaultRecordLine),
		Priority: priority,
	}
	if recordType == "SRV" {
		record.Weight = srvWeight
	} else if fields.weight != nil {
		record.Weight = pointer(int(*fields.weight))
	}
	return record
}

func recordContent(recordType string, values []string) (string, *int, *int) {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			trimmed = append(trimmed, value)
		}
	}

	switch recordType {
	case "MX":
		content, prefixes, ok := stripCommonNumericPrefixes(trimmed, 1)
		if ok {
			return strings.Join(content, "\n"), pointer(prefixes[0]), nil
		}
	case "SRV":
		content, prefixes, ok := stripCommonNumericPrefixes(trimmed, 2)
		if ok {
			return strings.Join(content, "\n"), pointer(prefixes[0]), pointer(prefixes[1])
		}
	}

	return strings.Join(trimmed, "\n"), nil, nil
}

func stripCommonNumericPrefixes(values []string, count int) ([]string, []int, bool) {
	if len(values) == 0 {
		return nil, nil, false
	}

	prefixes := make([]int, count)
	content := make([]string, 0, len(values))
	for index, value := range values {
		fields := strings.Fields(value)
		if len(fields) <= count {
			return nil, nil, false
		}
		for prefixIndex := 0; prefixIndex < count; prefixIndex++ {
			parsed, err := strconv.Atoi(fields[prefixIndex])
			if err != nil {
				return nil, nil, false
			}
			if index == 0 {
				prefixes[prefixIndex] = parsed
			} else if prefixes[prefixIndex] != parsed {
				return nil, nil, false
			}
		}
		content = append(content, strings.Join(fields[count:], " "))
	}

	return content, prefixes, true
}

func recordValuesFromInput(recordType string, input dns.RecordInput) ([]string, error) {
	if input.TTL <= 0 || int64(input.TTL) > int64(^uint32(0)>>1) {
		return nil, fmt.Errorf("huaweicloud: TTL must be between 1 and 2147483647")
	}

	values := splitRecordValues(input.Content)
	if len(values) == 0 {
		return nil, fmt.Errorf("huaweicloud: record value is required")
	}

	switch recordType {
	case "TXT":
		for i, value := range values {
			if !isQuotedTXTValue(value) {
				values[i] = strconv.Quote(value)
			}
		}
	case "MX":
		for i, value := range values {
			if hasNumericPrefixes(value, 1) {
				continue
			}
			if input.Priority == nil {
				return nil, fmt.Errorf("huaweicloud: MX records require a priority")
			}
			values[i] = fmt.Sprintf("%d %s", *input.Priority, value)
		}
	case "SRV":
		for i, value := range values {
			if hasNumericPrefixes(value, 3) {
				continue
			}
			if input.Priority == nil || input.Weight == nil {
				return nil, fmt.Errorf("huaweicloud: SRV records require priority and weight")
			}
			fields := strings.Fields(value)
			if len(fields) < 2 {
				return nil, fmt.Errorf("huaweicloud: SRV record value must contain a port and target")
			}
			if _, err := strconv.Atoi(fields[0]); err != nil {
				return nil, fmt.Errorf("huaweicloud: SRV record value must start with a numeric port")
			}
			values[i] = fmt.Sprintf("%d %d %s", *input.Priority, *input.Weight, value)
		}
	}

	return values, nil
}

func splitRecordValues(content string) []string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	values := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			values = append(values, line)
		}
	}
	return values
}

func hasNumericPrefixes(value string, count int) bool {
	fields := strings.Fields(value)
	if len(fields) <= count {
		return false
	}
	for i := 0; i < count; i++ {
		if _, err := strconv.Atoi(fields[i]); err != nil {
			return false
		}
	}
	return true
}

func isQuotedTXTValue(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"'
}

func recordFQDN(domain, name string) string {
	zone := normalizeZoneName(domain)
	relative := strings.Trim(strings.ToLower(strings.TrimSpace(name)), ".")
	if relative == "" || relative == "@" || relative == zone {
		return zone + "."
	}
	if strings.HasSuffix(relative, "."+zone) {
		return relative + "."
	}
	return relative + "." + zone + "."
}

func relativeRecordName(name, domain string) string {
	recordName := normalizeZoneName(name)
	zoneName := normalizeZoneName(domain)
	if recordName == "" || recordName == zoneName {
		return "@"
	}
	if zoneName != "" && strings.HasSuffix(recordName, "."+zoneName) {
		return strings.TrimSuffix(recordName, "."+zoneName)
	}
	return recordName
}

func normalizeZoneName(value string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(value)), ".")
}

func recordLine(line *string, fallback string) string {
	if line == nil {
		return fallback
	}
	return firstNonEmpty(strings.TrimSpace(*line), fallback)
}

func lookupCredential(cred *dns.Credential, key string) string {
	if cred == nil {
		return ""
	}
	return firstNonEmpty(cred.Values[key], cred.Additional[key])
}

func defaultTimeout() time.Duration {
	return 10 * time.Second
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringSliceValue(value *[]string) []string {
	if value == nil {
		return nil
	}
	return *value
}

func int32Value(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

func int32Pointer(value int) *int32 {
	converted := int32(value)
	return &converted
}

func pointer[T any](value T) *T {
	return &value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
