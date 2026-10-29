package middleware

import (
	"errors"

	"github.com/0xJacky/Nginx-UI/internal/config"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/settings"
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

// redactJSONHealthCheck mirrors the local health check redaction: header
// values, the request body and credentials embedded in the probe and click
// URLs.
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
