package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
)

type proxyJSONRedactor func(payload map[string]any) error

var errProxyRedactionShape = errors.New("response shape is invalid")

// readOnlyServiceTokenProxyRedactions hides the stored secrets a child node
// returns for a caller whose service token lacks api:write. The child
// authenticates the controller's node principal rather than the original
// token, so it answers in full and the controller must apply the read-only
// rule before relaying the response. Each entry mirrors the redaction its local
// handler applies, keeping the response shape and replacing values with the
// shared placeholder.
var readOnlyServiceTokenProxyRedactions = map[string]proxyJSONRedactor{
	"/api/certs/:id":                        redactJSONString("ssl_certificate_key"),
	"/api/config":                           refuseJSONPrivateKeyFile,
	"/api/config_histories":                 redactJSONListItems(redactJSONPrivateKeyBackup),
	"/api/dns/domains":                      redactJSONListItems(redactJSONDomainCredential),
	"/api/dns/domains/:id":                  redactJSONDomainCredential,
	"/api/external_notifies":                redactJSONListItems(redactJSONValues("config")),
	"/api/external_notifies/:id":            redactJSONValues("config"),
	"/api/site_navigation":                  redactJSONListItems(redactJSONHealthCheck),
	"/api/site_navigation/health_check/:id": redactJSONHealthCheck,
}

// A file can be secret on a child node only because the child's own
// configuration names it in a secret directive, which the controller cannot
// see. For the routes below the controller therefore asks the child to apply
// its own classification: it relays a read-only service token's request with
// proxyReadOnlyServiceTokenQuery, and a child that withheld secret files for
// it answers with secretFilesWithheldHeader. The controller still applies its
// own extension and content check afterwards.
//
// A child that does not answer predates this exchange and has applied nothing,
// so the controller cannot tell its secret files from the others. It then
// withholds every file instead, with the redaction listed here: a refused file
// read and history entries without content.
var readOnlyServiceTokenUnconfirmedProxyRedactions = map[string]proxyJSONRedactor{
	"/api/config":           refuseJSONUnconfirmedFile,
	"/api/config_histories": redactJSONListItems(redactJSONBackupContent),
}

// proxyReadOnlyServiceTokenQuery marks a request relayed for a read-only
// service token. Like proxyVerifiedTwoFactorQuery it is set or removed by
// every proxy before the node transport signs the complete query, so a client
// cannot supply it, and a child honors it only behind node authentication.
const proxyReadOnlyServiceTokenQuery = "__nginx_ui_proxy_read_only_service_token"

// secretFilesWithheldHeader is the child's confirmation that it withheld its
// secret files from the relayed read-only caller. The value is the RFC 9651
// boolean true, `Secret-Files-Withheld: ?1`. The controller removes it from
// every relayed response.
const secretFilesWithheldHeader = "Secret-Files-Withheld"

// WithholdSecretFiles reports whether a configuration read must withhold
// secret files from its caller: a read-only service token calling this node,
// or a controller relaying the request of one. In the relayed case it also
// confirms to the controller that this node applied the rule, so call it
// before the response is written.
func WithholdSecretFiles(c *gin.Context) bool {
	if internalmcp.IsReadOnlyServiceTokenRequest(c) {
		return true
	}
	if _, isNode := c.Get(nodeauth.GinPrincipalKey); !isNode ||
		!c.Request.URL.Query().Has(proxyReadOnlyServiceTokenQuery) {
		return false
	}
	c.Header(secretFilesWithheldHeader, "?1")
	return true
}

// setProxyReadOnlyServiceToken replaces any client-supplied marker with the
// controller's own decision, which is only sent on the routes that use it.
func setProxyReadOnlyServiceToken(request *http.Request, route string, readOnly bool) {
	values := request.URL.Query()
	values.Del(proxyReadOnlyServiceTokenQuery)
	if _, classifiedByChild := readOnlyServiceTokenUnconfirmedProxyRedactions[route]; readOnly && classifiedByChild {
		values.Set(proxyReadOnlyServiceTokenQuery, "true")
	}
	request.URL.RawQuery = values.Encode()
}

// takeSecretFilesWithheld reports whether a child confirmed that it withheld
// secret files, and removes the confirmation from the relayed response.
func takeSecretFilesWithheld(response *http.Response) bool {
	values := response.Header.Values(secretFilesWithheldHeader)
	response.Header.Del(secretFilesWithheldHeader)
	return len(values) == 1 && strings.Trim(values[0], " \t") == "?1"
}

// relayChildSecretFileRefusal keeps the 403 a child returns when it refuses a
// secret file to a relayed read-only caller, which the proxy would otherwise
// turn into a 503. It applies only to a refusal the child confirmed. The 403
// reaches only read-only service tokens, the only callers this runs for, never
// the web UI's login token, so it cannot sign the web UI out.
func relayChildSecretFileRefusal(response *http.Response, route string, upstreamStatus int, childWithheld bool) {
	if _, classifiedByChild := readOnlyServiceTokenUnconfirmedProxyRedactions[route]; !classifiedByChild ||
		!childWithheld || upstreamStatus != http.StatusForbidden {
		return
	}
	response.StatusCode = http.StatusForbidden
	response.Status = strconv.Itoa(http.StatusForbidden) + " " + http.StatusText(http.StatusForbidden)
	response.Header.Del(proxyUpstreamStatusHeader)
}

// readOnlyServiceTokenRefusedProxyWebSockets lists child node feeds that stream
// stored secrets. WebSocket frames are relayed without being decoded, so the
// controller cannot redact them and refuses to relay them for read-only tokens.
// The controller's own feeds redact in their handlers and stay available.
var readOnlyServiceTokenRefusedProxyWebSockets = map[string]struct{}{
	"/api/site_navigation_ws": {},
}

func redactJSONListItems(redact proxyJSONRedactor) proxyJSONRedactor {
	return func(payload map[string]any) error {
		if payload["data"] == nil {
			return nil
		}
		items, ok := payload["data"].([]any)
		if !ok {
			return errProxyRedactionShape
		}
		for _, value := range items {
			item, ok := value.(map[string]any)
			if !ok {
				return errProxyRedactionShape
			}
			if err := redact(item); err != nil {
				return err
			}
		}
		return nil
	}
}

// jsonObjectAt follows path through nested objects. A missing or null step
// means there is nothing to redact.
func jsonObjectAt(payload map[string]any, path ...string) (map[string]any, error) {
	current := payload
	for _, key := range path {
		value, ok := current[key]
		if !ok || value == nil {
			return nil, nil
		}
		next, ok := value.(map[string]any)
		if !ok {
			return nil, errProxyRedactionShape
		}
		current = next
	}
	return current, nil
}

// redactJSONValues replaces every value of the object at path with the
// placeholder and keeps its keys.
func redactJSONValues(path ...string) proxyJSONRedactor {
	return func(payload map[string]any) error {
		values, err := jsonObjectAt(payload, path...)
		if err != nil || values == nil {
			return err
		}
		for key := range values {
			values[key] = settings.RedactedSensitiveValue
		}
		return nil
	}
}

func redactJSONString(field string) proxyJSONRedactor {
	return func(payload map[string]any) error {
		if value, ok := payload[field].(string); ok && value != "" {
			payload[field] = settings.RedactedSensitiveValue
		}
		return nil
	}
}

func redactJSONDomainCredential(payload map[string]any) error {
	for _, field := range []string{"credentials", "additional"} {
		redact := redactJSONValues("dns_credential", "config", "configuration", field)
		if err := redact(payload); err != nil {
			return err
		}
	}
	return nil
}

// redactJSONHealthCheck mirrors the local health check redaction for
// read-only tokens: header values, the request body and credentials embedded
// in the probe and click URLs. The form values of a plugin probe kind are
// hidden from every service token by serviceTokenProxyRedactions.
func redactJSONHealthCheck(payload map[string]any) error {
	if err := redactJSONValues("health_check_config", "headers")(payload); err != nil {
		return err
	}
	healthCheck, err := jsonObjectAt(payload, "health_check_config")
	if err != nil || healthCheck == nil {
		return err
	}
	if body, ok := healthCheck["body"].(string); ok && body != "" {
		healthCheck["body"] = settings.RedactedSensitiveValue
	}
	for _, field := range []string{"target_url", "click_url"} {
		if rawURL, ok := healthCheck[field].(string); ok {
			healthCheck[field] = internalmcp.RedactURLCredentials(rawURL)
		}
	}
	return nil
}

// jsonSecretFile judges a relayed file by its extension and content. The
// references a child's own configuration makes are not visible here, so files
// that are secret only because a directive names them depend on the child
// applying the rule itself.
func jsonSecretFile(payload map[string]any) bool {
	path, _ := payload["filepath"].(string)
	content, _ := payload["content"].(string)
	return config.IsSecretFileContent(path, []byte(content))
}

func redactJSONPrivateKeyBackup(payload map[string]any) error {
	if jsonSecretFile(payload) {
		payload["content"] = settings.RedactedSensitiveValue
	}
	return nil
}

func refuseJSONPrivateKeyFile(payload map[string]any) error {
	if jsonSecretFile(payload) {
		return config.ErrSecretFileHiddenFromReadOnly
	}
	return nil
}

func refuseJSONUnconfirmedFile(map[string]any) error {
	return ErrProxyNodeCannotWithholdSecretFiles
}

func redactJSONBackupContent(payload map[string]any) error {
	if _, ok := payload["content"]; ok {
		payload["content"] = settings.RedactedSensitiveValue
	}
	return nil
}
