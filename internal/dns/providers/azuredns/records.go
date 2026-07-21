package azuredns

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

const (
	// recordIDSeparator joins the relative name and the record type into a record ID.
	// A DNS label can never contain it, so the encoding is unambiguous.
	recordIDSeparator = "|"

	// apexName is the relative name Azure uses for the zone apex.
	apexName = "@"

	// valueSeparator joins the values of a record set into a single content field.
	valueSeparator = "\n"

	// txtChunkSize is the maximum length of a single TXT character string.
	txtChunkSize = 255
)

// newRecordID encodes the identity of an Azure record set. The ID is a pure function
// of the record set key, so it stays byte identical across content updates, which the
// DDNS engine relies on when it matches persisted targets against a fresh listing.
func newRecordID(relativeName string, recordType armdns.RecordType) string {
	return relativeName + recordIDSeparator + string(recordType)
}

// parseRecordID decodes a record ID back into the record set name and type.
func parseRecordID(id string) (string, armdns.RecordType, error) {
	trimmed := strings.TrimSpace(id)

	index := strings.LastIndex(trimmed, recordIDSeparator)
	if index < 0 {
		return "", "", fmt.Errorf("azuredns: invalid record id %q", id)
	}

	namePart := trimmed[:index]
	if strings.TrimSpace(namePart) == "" {
		return "", "", fmt.Errorf("azuredns: invalid record id %q", id)
	}

	name, err := normalizeRelativeName(namePart)
	if err != nil {
		return "", "", fmt.Errorf("azuredns: invalid record id %q", id)
	}

	recordType, err := toRecordType(trimmed[index+1:])
	if err != nil {
		return "", "", fmt.Errorf("azuredns: invalid record id %q", id)
	}

	return name, recordType, nil
}

// normalizeRelativeName canonicalizes a relative record set name and rejects any
// character that would break the record ID encoding or the API route it travels on.
func normalizeRelativeName(name string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	trimmed = strings.TrimSuffix(trimmed, ".")

	if trimmed == "" || trimmed == apexName {
		return apexName, nil
	}

	if strings.ContainsAny(trimmed, "/%#?"+recordIDSeparator) {
		return "", fmt.Errorf("azuredns: invalid record name %q", name)
	}

	for _, r := range trimmed {
		if r <= ' ' || r == 0x7f {
			return "", fmt.Errorf("azuredns: invalid record name %q", name)
		}
	}

	return trimmed, nil
}

// relativeName converts a possibly fully qualified name into a zone relative one.
func relativeName(name, zone string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	trimmed = strings.TrimSuffix(trimmed, ".")

	if trimmed == "" || trimmed == apexName {
		return apexName, nil
	}

	if zoneName := normalizeZoneName(zone); zoneName != "" {
		if trimmed == zoneName {
			return apexName, nil
		}
		trimmed = strings.TrimSuffix(trimmed, "."+zoneName)
	}

	return normalizeRelativeName(trimmed)
}

// toRecordType validates a record type against the types Azure DNS supports.
func toRecordType(value string) (armdns.RecordType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))

	for _, recordType := range armdns.PossibleRecordTypeValues() {
		if string(recordType) == normalized {
			return recordType, nil
		}
	}

	return "", fmt.Errorf("azuredns: unsupported record type %q", value)
}

// recordTypeFromResourceType maps an ARM resource type such as
// "Microsoft.Network/dnszones/A" onto a record type.
func recordTypeFromResourceType(value string) (armdns.RecordType, bool) {
	trimmed := strings.TrimSpace(value)
	if index := strings.LastIndex(trimmed, "/"); index >= 0 {
		trimmed = trimmed[index+1:]
	}

	recordType, err := toRecordType(trimmed)
	if err != nil {
		return "", false
	}

	return recordType, true
}

// flattenRecordSet converts an Azure record set into a single manageable record.
// Record sets that cannot be classified are skipped rather than half reported.
func flattenRecordSet(recordSet *armdns.RecordSet) (dns.Record, bool) {
	if recordSet == nil || recordSet.Properties == nil || recordSet.Name == nil || recordSet.Type == nil {
		return dns.Record{}, false
	}

	// Azure already reports the name relative to the zone and carries the fully
	// qualified form separately in Properties.Fqdn. Stripping the zone suffix again
	// here would collapse a record set legitimately named "www.example.com" onto the
	// unrelated "www" set, handing both the same record ID.
	name, err := normalizeRelativeName(*recordSet.Name)
	if err != nil {
		return dns.Record{}, false
	}

	recordType, ok := recordTypeFromResourceType(*recordSet.Type)
	if !ok {
		return dns.Record{}, false
	}

	return recordFrom(name, recordType, recordSet.Properties), true
}

// recordFrom builds the record representation returned to the DNS service. Every
// path funnels through here so that the emitted ID always matches the one a later
// listing produces, regardless of how Azure spells the name in its response.
func recordFrom(name string, recordType armdns.RecordType, props *armdns.RecordSetProperties) dns.Record {
	record := dns.Record{
		ID:   newRecordID(name, recordType),
		Type: string(recordType),
		Name: name,
	}

	if props == nil {
		return record
	}

	if props.TTL != nil {
		record.TTL = int(*props.TTL)
	}

	values, priority, weight := recordSetValues(recordType, props)
	record.Priority = priority
	record.Weight = weight

	if len(values) == 0 && props.TargetResource != nil && props.TargetResource.ID != nil {
		// Alias record sets carry no inline values, only a target resource.
		record.Content = *props.TargetResource.ID
		return record
	}

	record.Content = strings.Join(values, valueSeparator)

	return record
}

// recordSetValues renders the values of a record set as display strings, together
// with the priority and weight the UI exposes as dedicated fields.
func recordSetValues(recordType armdns.RecordType, props *armdns.RecordSetProperties) ([]string, *int, *int) {
	values := make([]string, 0, 4)

	switch recordType {
	case armdns.RecordTypeA:
		for _, item := range props.ARecords {
			if item != nil && item.IPv4Address != nil {
				values = append(values, *item.IPv4Address)
			}
		}

	case armdns.RecordTypeAAAA:
		for _, item := range props.AaaaRecords {
			if item != nil && item.IPv6Address != nil {
				values = append(values, *item.IPv6Address)
			}
		}

	case armdns.RecordTypeCNAME:
		if props.CnameRecord != nil && props.CnameRecord.Cname != nil {
			values = append(values, *props.CnameRecord.Cname)
		}

	case armdns.RecordTypeTXT:
		for _, item := range props.TxtRecords {
			if item != nil {
				values = append(values, joinTXT(item.Value))
			}
		}

	case armdns.RecordTypeNS:
		for _, item := range props.NsRecords {
			if item != nil && item.Nsdname != nil {
				values = append(values, *item.Nsdname)
			}
		}

	case armdns.RecordTypePTR:
		for _, item := range props.PtrRecords {
			if item != nil && item.Ptrdname != nil {
				values = append(values, *item.Ptrdname)
			}
		}

	case armdns.RecordTypeMX:
		var priority *int
		for _, item := range props.MxRecords {
			if item == nil || item.Exchange == nil {
				continue
			}
			preference := int32Value(item.Preference)
			values = append(values, fmt.Sprintf("%d %s", preference, *item.Exchange))
			if priority == nil {
				value := int(preference)
				priority = &value
			}
		}
		return values, priority, nil

	case armdns.RecordTypeSRV:
		var priority, weight *int
		for _, item := range props.SrvRecords {
			if item == nil || item.Target == nil {
				continue
			}
			values = append(values, fmt.Sprintf("%d %d %d %s",
				int32Value(item.Priority), int32Value(item.Weight), int32Value(item.Port), *item.Target))
			if priority == nil {
				value := int(int32Value(item.Priority))
				priority = &value
			}
			if weight == nil {
				value := int(int32Value(item.Weight))
				weight = &value
			}
		}
		return values, priority, weight

	case armdns.RecordTypeCAA:
		for _, item := range props.CaaRecords {
			if item == nil || item.Tag == nil {
				continue
			}
			values = append(values, fmt.Sprintf("%d %s %s",
				int32Value(item.Flags), *item.Tag, strconv.Quote(stringValue(item.Value))))
		}

	case armdns.RecordTypeSOA:
		if soa := props.SoaRecord; soa != nil {
			values = append(values, fmt.Sprintf("%s %s %d %d %d %d %d",
				stringValue(soa.Host), stringValue(soa.Email), int64Value(soa.SerialNumber),
				int64Value(soa.RefreshTime), int64Value(soa.RetryTime),
				int64Value(soa.ExpireTime), int64Value(soa.MinimumTTL)))
		}
	}

	return values, nil, nil
}

// recordSetProperties builds the Azure payload for a record set from user input.
func recordSetProperties(input dns.RecordInput, recordType armdns.RecordType) (*armdns.RecordSetProperties, error) {
	if recordType == armdns.RecordTypeSOA {
		return nil, fmt.Errorf("azuredns: SOA record sets are read-only")
	}

	values := splitValues(input.Content)
	if len(values) == 0 {
		return nil, fmt.Errorf("azuredns: record value is required")
	}

	props := &armdns.RecordSetProperties{TTL: ptr(normalizeTTL(input.TTL))}

	switch recordType {
	case armdns.RecordTypeA:
		for _, value := range values {
			props.ARecords = append(props.ARecords, &armdns.ARecord{IPv4Address: ptr(value)})
		}

	case armdns.RecordTypeAAAA:
		for _, value := range values {
			props.AaaaRecords = append(props.AaaaRecords, &armdns.AaaaRecord{IPv6Address: ptr(value)})
		}

	case armdns.RecordTypeCNAME:
		if len(values) > 1 {
			return nil, fmt.Errorf("azuredns: CNAME record sets accept a single value")
		}
		props.CnameRecord = &armdns.CnameRecord{Cname: ptr(values[0])}

	case armdns.RecordTypeTXT:
		for _, value := range values {
			// The value is stored verbatim, matching the other providers: quotes are
			// zone file syntax, not part of the record data Azure holds.
			props.TxtRecords = append(props.TxtRecords, &armdns.TxtRecord{Value: chunkTXT(value)})
		}

	case armdns.RecordTypeNS:
		for _, value := range values {
			props.NsRecords = append(props.NsRecords, &armdns.NsRecord{Nsdname: ptr(value)})
		}

	case armdns.RecordTypePTR:
		for _, value := range values {
			props.PtrRecords = append(props.PtrRecords, &armdns.PtrRecord{Ptrdname: ptr(value)})
		}

	case armdns.RecordTypeMX:
		for _, value := range values {
			record, err := parseMXValue(value, input.Priority)
			if err != nil {
				return nil, err
			}
			props.MxRecords = append(props.MxRecords, record)
		}

	case armdns.RecordTypeSRV:
		for _, value := range values {
			record, err := parseSRVValue(value, input.Priority, input.Weight)
			if err != nil {
				return nil, err
			}
			props.SrvRecords = append(props.SrvRecords, record)
		}

	case armdns.RecordTypeCAA:
		for _, value := range values {
			record, err := parseCAAValue(value)
			if err != nil {
				return nil, err
			}
			props.CaaRecords = append(props.CaaRecords, record)
		}

	default:
		return nil, fmt.Errorf("azuredns: unsupported record type %q", string(recordType))
	}

	return props, nil
}

// parseMXValue accepts "<preference> <exchange>" or a bare exchange, in which case
// the preference comes from the dedicated priority field.
func parseMXValue(value string, fallbackPriority *int) (*armdns.MxRecord, error) {
	fields := strings.Fields(value)

	switch len(fields) {
	case 1:
		preference, err := fallbackInt32(fallbackPriority)
		if err != nil {
			return nil, fmt.Errorf("azuredns: invalid MX preference %d", intValue(fallbackPriority))
		}
		return &armdns.MxRecord{
			Preference: ptr(preference),
			Exchange:   ptr(fields[0]),
		}, nil
	case 2:
		preference, err := parseInt32(fields[0], 0, math.MaxUint16)
		if err != nil {
			return nil, fmt.Errorf("azuredns: invalid MX preference %q", fields[0])
		}
		return &armdns.MxRecord{
			Preference: ptr(preference),
			Exchange:   ptr(fields[1]),
		}, nil
	default:
		return nil, fmt.Errorf("azuredns: invalid MX value %q, expected \"<preference> <exchange>\"", value)
	}
}

// parseSRVValue reads an SRV value from the right so that omitted leading fields
// fall back to the dedicated priority and weight inputs.
func parseSRVValue(value string, fallbackPriority, fallbackWeight *int) (*armdns.SrvRecord, error) {
	fields := strings.Fields(value)
	if len(fields) < 2 || len(fields) > 4 {
		return nil, fmt.Errorf("azuredns: invalid SRV value %q, expected \"<priority> <weight> <port> <target>\"", value)
	}

	last := len(fields) - 1
	target := fields[last]

	port, err := parseInt32(fields[last-1], 0, math.MaxUint16)
	if err != nil {
		return nil, fmt.Errorf("azuredns: invalid SRV port %q", fields[last-1])
	}

	weight, err := fallbackInt32(fallbackWeight)
	if err != nil {
		return nil, fmt.Errorf("azuredns: invalid SRV weight %d", intValue(fallbackWeight))
	}
	if len(fields) >= 3 {
		weight, err = parseInt32(fields[last-2], 0, math.MaxUint16)
		if err != nil {
			return nil, fmt.Errorf("azuredns: invalid SRV weight %q", fields[last-2])
		}
	}

	priority, err := fallbackInt32(fallbackPriority)
	if err != nil {
		return nil, fmt.Errorf("azuredns: invalid SRV priority %d", intValue(fallbackPriority))
	}
	if len(fields) == 4 {
		priority, err = parseInt32(fields[0], 0, math.MaxUint16)
		if err != nil {
			return nil, fmt.Errorf("azuredns: invalid SRV priority %q", fields[0])
		}
	}

	return &armdns.SrvRecord{
		Priority: ptr(priority),
		Weight:   ptr(weight),
		Port:     ptr(port),
		Target:   ptr(target),
	}, nil
}

// parseCAAValue accepts "<flags> <tag> <value>" and the flag-less "<tag> <value>".
func parseCAAValue(value string) (*armdns.CaaRecord, error) {
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return nil, fmt.Errorf("azuredns: invalid CAA value %q, expected \"<flags> <tag> <value>\"", value)
	}

	if flags, err := parseInt32(fields[0], 0, 255); err == nil {
		if len(fields) < 3 {
			return nil, fmt.Errorf("azuredns: invalid CAA value %q, expected \"<flags> <tag> <value>\"", value)
		}
		return &armdns.CaaRecord{
			Flags: ptr(flags),
			Tag:   ptr(fields[1]),
			Value: ptr(unquote(strings.Join(fields[2:], " "))),
		}, nil
	}

	return &armdns.CaaRecord{
		Flags: ptr(int32(0)),
		Tag:   ptr(fields[0]),
		Value: ptr(unquote(strings.Join(fields[1:], " "))),
	}, nil
}

// chunkTXT splits a TXT value into the character strings the protocol allows.
//
// Chunks are cut on rune boundaries. A chunk that ends mid rune is not valid UTF-8,
// and the JSON encoder used for the request body silently rewrites invalid bytes to
// U+FFFD, which would store a corrupted value in Azure.
func chunkTXT(value string) []*string {
	if value == "" {
		return []*string{ptr("")}
	}

	chunks := make([]*string, 0, len(value)/txtChunkSize+1)

	start := 0
	for start < len(value) {
		end := start + txtChunkSize
		if end >= len(value) {
			chunks = append(chunks, ptr(value[start:]))
			break
		}

		// Walk back to the start of the rune that would be split.
		for end > start && !utf8.RuneStart(value[end]) {
			end--
		}
		if end == start {
			// A single rune longer than the chunk size cannot happen for valid
			// UTF-8, so fall back to a byte cut rather than looping forever.
			end = start + txtChunkSize
		}

		chunks = append(chunks, ptr(value[start:end]))
		start = end
	}

	return chunks
}

// joinTXT reassembles the character strings of a TXT record into one value.
func joinTXT(chunks []*string) string {
	var builder strings.Builder
	for _, chunk := range chunks {
		if chunk != nil {
			builder.WriteString(*chunk)
		}
	}

	return builder.String()
}

// splitValues turns the multi line content field into individual record values.
func splitValues(content string) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	parts := strings.Split(normalized, "\n")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}

	return values
}

// matchesFilter applies the record filter the UI sends, mirroring the substring
// matching the other providers delegate to their vendor API.
func matchesFilter(record dns.Record, filter dns.RecordFilter) bool {
	if recordType := strings.ToUpper(strings.TrimSpace(filter.Type)); recordType != "" {
		if !strings.EqualFold(record.Type, recordType) {
			return false
		}
	}

	if name := strings.ToLower(strings.TrimSpace(filter.Name)); name != "" {
		if !strings.Contains(strings.ToLower(record.Name), name) {
			return false
		}
	}

	return true
}

// normalizeTTL clamps a TTL into the range Azure DNS accepts.
func normalizeTTL(ttl int) int64 {
	if ttl <= 0 {
		return 1
	}
	if ttl > math.MaxInt32 {
		return math.MaxInt32
	}

	return int64(ttl)
}

// fallbackInt32 validates a priority or weight taken from the dedicated form field,
// which the API layer does not range check, before it is narrowed to int32.
func fallbackInt32(value *int) (int32, error) {
	parsed := intValue(value)
	if parsed < 0 || parsed > math.MaxUint16 {
		return 0, fmt.Errorf("value %d out of range", parsed)
	}

	return int32(parsed), nil
}

func parseInt32(value string, minValue, maxValue int64) (int32, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, err
	}
	if parsed < minValue || parsed > maxValue {
		return 0, fmt.Errorf("value %d out of range", parsed)
	}

	return int32(parsed), nil
}

func unquote(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`) {
		if unquoted, err := strconv.Unquote(trimmed); err == nil {
			return unquoted
		}
		return trimmed[1 : len(trimmed)-1]
	}

	return trimmed
}

func ptr[T any](value T) *T {
	return &value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}

	return *value
}

func int32Value(value *int32) int32 {
	if value == nil {
		return 0
	}

	return *value
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}

	return *value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}

	return ""
}
