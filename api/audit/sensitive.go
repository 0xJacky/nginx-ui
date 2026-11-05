package audit

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const (
	redactedValue        = "[REDACTED]"
	redactedRequestBody  = "[sensitive request redacted]"
	redactedResponseBody = "[sensitive response redacted]"
	redactedSessionLogs  = "[sensitive session logs redacted]"
	// backupSecurityHeaderName carries the backup archive's decryption token
	// (api/backup); the install secret names mirror internal/system, which this
	// package does not import to keep its dependency graph small.
	backupSecurityHeaderName = "X-Backup-Security"
	installSecretHeaderName  = "X-Install-Secret"
	installSecretQueryKey    = "install_secret"
)

type routeRedaction struct {
	request  bool
	response bool
}

var (
	redactRequest  = routeRedaction{request: true}
	redactResponse = routeRedaction{response: true}
	redactBoth     = routeRedaction{request: true, response: true}
)

// sensitiveRoutes lists, by method and gin route template, the routes whose
// request or response body carries a stored credential: passwords, private
// keys, provider secrets, notifier tokens or one-time tokens. The rule is keyed
// on the matched route rather than set inside the handler because requests for
// a remote node are answered by the proxy middleware before any handler runs,
// and the proxied response still passes through the local audit log.
var sensitiveRoutes = map[string]routeRedaction{
	// Accounts and session tokens.
	"POST /api/user/password": redactRequest,
	"POST /api/users":         redactRequest,
	"POST /api/users/:id":     redactRequest,
	"PATCH /api/users/:id":    redactRequest,
	"POST /api/setup/install": redactRequest,
	"POST /api/token/short":   redactResponse,

	// The legacy MCP SSE transport answers POST /mcp_message with 202 and
	// delivers every tool result, configuration file content included, on
	// this long-lived stream. The stream's request names no file, so the
	// configuration file check cannot tell which results carry a secret.
	"GET /mcp": redactResponse,

	// Service tokens are minted or rotated in the response.
	"POST /api/mcp/tokens":                redactResponse,
	"POST /api/mcp/tokens/:id/rotate":     redactResponse,
	"POST /api/service_tokens":            redactResponse,
	"POST /api/service_tokens/:id/rotate": redactResponse,

	// Settings carry OIDC/Casdoor client secrets, the LLM token, the node
	// secret and the managed SSH private key.
	"POST /api/settings":                   redactRequest,
	"POST /api/settings/:section":          redactRequest,
	"POST /api/settings/nginx/private-key": redactRequest,
	"GET /api/settings/protected":          redactResponse,
	"POST /api/llm/models":                 redactRequest,
	"POST /api/host/setup/keypair":         redactResponse,

	// Certificates: key material is accepted on write and returned on read.
	"GET /api/certs/:id":             redactResponse,
	"POST /api/certs":                redactBoth,
	"POST /api/certs/:id":            redactBoth,
	"POST /api/certs/:id/download":   redactBoth,
	"POST /api/cert_import":          redactResponse,
	"POST /api/cert_parse_archive":   redactBoth,
	"PUT /api/cert_sync":             redactBoth,
	"POST /api/self_signed_cert":     redactResponse,
	"POST /api/self_signed_cert/:id": redactResponse,

	// DNS provider credentials.
	"GET /api/dns_credentials":      redactResponse,
	"GET /api/dns_credentials/:id":  redactResponse,
	"POST /api/dns_credentials":     redactBoth,
	"POST /api/dns_credentials/:id": redactBoth,
	// DNS domains preload their credential for interactive users and api:write
	// tokens. Create and update responses do not preload it.
	"GET /api/dns/domains":     redactResponse,
	"GET /api/dns/domains/:id": redactResponse,

	// ACME accounts carry the EAB HMAC key.
	"GET /api/acme_users":               redactResponse,
	"GET /api/acme_users/:id":           redactResponse,
	"POST /api/acme_users":              redactBoth,
	"POST /api/acme_users/:id":          redactBoth,
	"PATCH /api/acme_users/:id":         redactResponse,
	"POST /api/acme_users/:id/register": redactResponse,

	// External notifier configs hold bot tokens, webhook secrets and passwords.
	"GET /api/external_notifies":       redactResponse,
	"GET /api/external_notifies/:id":   redactResponse,
	"POST /api/external_notifies":      redactBoth,
	"POST /api/external_notifies/:id":  redactBoth,
	"PATCH /api/external_notifies/:id": redactResponse,
	"POST /api/external_notifies/test": redactRequest,

	// Backups: S3 keys, the backup archive and its decryption token.
	"GET /api/auto_backup":          redactResponse,
	"GET /api/auto_backup/:id":      redactResponse,
	"POST /api/auto_backup":         redactBoth,
	"POST /api/auto_backup/:id":     redactBoth,
	"PATCH /api/auto_backup/:id":    redactResponse,
	"POST /api/auto_backup/test_s3": redactRequest,
	"GET /api/backup":               redactResponse,
	"POST /api/restore":             redactRequest,
	"POST /api/setup/restore":       redactRequest,

	// Cluster node credentials.
	"POST /api/nodes":             redactRequest,
	"POST /api/nodes/:id":         redactRequest,
	"GET /api/nodes/:id/secret":   redactResponse,
	"POST /api/node/pair/upgrade": redactRequest,
}

// SensitiveRoutes lists the "METHOD /route/template" keys whose bodies are
// always redacted from audit logs.
func SensitiveRoutes() []string {
	routes := make([]string, 0, len(sensitiveRoutes))
	for route := range sensitiveRoutes {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	return routes
}

// credentialWords mark a field, header or query name as credential-bearing
// when they appear as a whole word in it.
var credentialWords = map[string]struct{}{
	"password":   {},
	"passwd":     {},
	"passphrase": {},
	"passcode":   {},
	"secret":     {},
	"token":      {},
	"signature":  {},
	"apikey":     {},
	"privatekey": {},
	"secretkey":  {},
	"accesskey":  {},
}

// envCredentialWords are plural forms that only mark ALL-CAPS environment
// style names (DNS provider settings such as HURRICANE_TOKENS or
// SYSE_CREDENTIALS). Lower-case JSON names keep the stricter singular rule so
// fields such as max_tokens or credentials objects stay readable.
var envCredentialWords = map[string]struct{}{
	"tokens":      {},
	"secrets":     {},
	"passwords":   {},
	"credentials": {},
}

// envCredentialNames are environment style names that hold a credential
// without any of the words above.
var envCredentialNames = map[string]struct{}{
	// Holds the service account JSON, including its private key.
	"GCE_SERVICE_ACCOUNT": {},
}

// credentialKeyQualifiers mark "<qualifier>_key" names as key material, while
// names such as public_key, site_key or key_type stay readable.
var credentialKeyQualifiers = map[string]struct{}{
	"private":     {},
	"secret":      {},
	"api":         {},
	"access":      {},
	"hmac":        {},
	"aes":         {},
	"auth":        {},
	"encryption":  {},
	"signing":     {},
	"device":      {},
	"certificate": {},
}

// nonSecretKeyQualifiers are the words before a trailing KEY that make an
// environment style name public rather than secret.
var nonSecretKeyQualifiers = map[string]struct{}{
	"public": {},
}

// nonSecretTrailingWords mark names that only point at a secret, such as a
// key file path, rather than hold it.
var nonSecretTrailingWords = map[string]struct{}{
	"path": {},
	"file": {},
	"id":   {},
}

// isCredentialName reports whether a JSON field, header or query parameter
// name looks like it holds a credential. Names are split into words on
// separators and camelCase boundaries so that, for example, "tokenized" or
// "max_tokens" are not mistaken for a token.
func isCredentialName(name string) bool {
	words := splitNameWords(name)
	if len(words) == 0 {
		return false
	}
	envStyle := isEnvStyleName(name)
	if envStyle {
		if _, ok := envCredentialNames[name]; ok {
			return true
		}
	}
	for _, word := range words {
		if _, ok := credentialWords[word]; ok {
			return true
		}
		if envStyle {
			if _, ok := envCredentialWords[word]; ok {
				return true
			}
		}
	}

	last := len(words) - 1
	if words[last] == "key" && last > 0 {
		if _, ok := credentialKeyQualifiers[words[last-1]]; ok {
			return true
		}
	}
	if !envStyle {
		return false
	}
	// Provider settings name their secret keys freely (OVH_CONSUMER_KEY,
	// TELE3_KEY, SELFHOSTDEV2_API_KEYS), so a trailing KEY or KEYS counts unless
	// it is a public key.
	if (words[last] == "key" || words[last] == "keys") && last > 0 {
		_, public := nonSecretKeyQualifiers[words[last-1]]
		return !public
	}
	// A private key mapping such as MYADDR_PRIVATE_KEYS_MAPPING still holds the
	// keys, while OCI_PRIVATE_KEY_PATH only points at one.
	if _, pointer := nonSecretTrailingWords[words[last]]; pointer {
		return false
	}
	for i := 0; i+1 < len(words); i++ {
		if words[i] == "private" && (words[i+1] == "key" || words[i+1] == "keys") {
			return true
		}
	}
	return false
}

// isEnvStyleName reports whether name is written like an environment
// variable: upper-case letters, digits and underscores only.
func isEnvStyleName(name string) bool {
	hasLetter := false
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9', r == '_':
		default:
			return false
		}
	}
	return hasLetter
}

func splitNameWords(name string) []string {
	var words []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	runes := []rune(name)
	for i, r := range runes {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if unicode.IsUpper(r) && i > 0 && unicode.IsLower(runes[i-1]) {
				flush()
			}
			current.WriteRune(unicode.ToLower(r))
		default:
			flush()
		}
	}
	flush()
	return words
}

// truncatedSuffix is what cosy appends to a body cut off at its capture limit.
const truncatedSuffix = " [truncated]"

// jsonStringField matches a JSON object member whose value is a string. The
// closing quote (group 3) is optional at the end of input, as is a dangling
// escape, so that a body cut off by the audit size limit cannot leak the start
// of a secret.
var jsonStringField = regexp.MustCompile(`"([^"\\]{1,128})"(\s*:\s*)"(?:[^"\\]|\\.)*\\?("|$)`)

// redactCredentialFields replaces the string value of every credential-named
// member in a JSON (or truncated JSON) body and leaves the rest of the body
// byte-for-byte intact. It only touches the copy kept for the audit log.
func redactCredentialFields(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return body
	}
	truncated := strings.HasSuffix(body, truncatedSuffix)
	return jsonStringField.ReplaceAllStringFunc(body, func(member string) string {
		parts := jsonStringField.FindStringSubmatch(member)
		if len(parts) < 4 || !isCredentialName(parts[1]) {
			return member
		}
		prefix := `"` + parts[1] + `"` + parts[2]
		// Keep empty values readable: they show a credential was cleared or unset.
		if member[len(prefix):] == `""` {
			return member
		}
		redacted := prefix + `"` + redactedValue + `"`
		// An unterminated value ran into cosy's truncation marker; keep the
		// marker so the log still shows the body was cut off.
		if parts[3] == "" && truncated {
			redacted += truncatedSuffix
		}
		return redacted
	})
}
