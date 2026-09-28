package nodeauth

import (
	"context"
	"net/http"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
)

// ReplicatedFromHeader marks a request a node sends to its sync targets on its
// own behalf. The value is an RFC 9651 sf-string holding the sender's instance
// id, e.g. `Replicated-From: "0b1c..."`.
//
// A write carrying it has already been fanned out by the node where the change
// was made. The receiver applies it locally and does not fan it out again;
// otherwise two nodes that list each other as sync targets would bounce the
// same write back and forth forever.
const ReplicatedFromHeader = "Replicated-From"

// IsReplicated reports whether ctx belongs to a request that another node
// forwarded as part of its synchronization. Such changes must not be
// synchronized onwards.
func IsReplicated(ctx context.Context) bool {
	principal, ok := PrincipalFromContext(ctx)
	return ok && principal.Replicated
}

func markReplicated(request *http.Request) {
	request.Header.Set(ReplicatedFromHeader, formatSFString(settings.NodeSettings.InstanceID))
}

func parseReplicatedFrom(header http.Header) (string, bool) {
	values := header.Values(ReplicatedFromHeader)
	if len(values) != 1 {
		return "", false
	}
	return parseSFString(values[0])
}

// formatSFString serializes an RFC 9651 sf-string. Characters outside the
// printable ASCII range cannot be represented and are dropped.
func formatSFString(value string) string {
	var builder strings.Builder
	builder.WriteByte('"')
	for index := 0; index < len(value); index++ {
		char := value[index]
		if char < 0x20 || char > 0x7e {
			continue
		}
		if char == '"' || char == '\\' {
			builder.WriteByte('\\')
		}
		builder.WriteByte(char)
	}
	builder.WriteByte('"')
	return builder.String()
}

// parseSFString parses a field value that must be exactly one RFC 9651
// sf-string, allowing the surrounding whitespace the field syntax permits.
func parseSFString(raw string) (string, bool) {
	raw = strings.Trim(raw, " \t")
	if len(raw) < 2 || raw[0] != '"' {
		return "", false
	}

	var builder strings.Builder
	for index := 1; index < len(raw); index++ {
		char := raw[index]
		switch {
		case char == '\\':
			index++
			if index >= len(raw) || (raw[index] != '"' && raw[index] != '\\') {
				return "", false
			}
			builder.WriteByte(raw[index])
		case char == '"':
			if index != len(raw)-1 {
				return "", false
			}
			return builder.String(), true
		case char < 0x20 || char > 0x7e:
			return "", false
		default:
			builder.WriteByte(char)
		}
	}
	return "", false
}
