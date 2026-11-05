package audit

import (
	"encoding/json"
	"net/http"
	"strings"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
)

const (
	sensitiveRequestAuditKey  = "SensitiveRequestAudit"
	sensitiveResponseAuditKey = "SensitiveResponseAudit"
)

// MarkSensitiveRequest prevents one-time or legacy credentials in a request
// from being copied into audit storage.
func MarkSensitiveRequest(c *gin.Context) {
	c.Set(sensitiveRequestAuditKey, true)
}

// MarkSensitiveResponse prevents one-time credentials in a response from
// being copied into audit storage.
func MarkSensitiveResponse(c *gin.Context) {
	c.Set(sensitiveResponseAuditKey, true)
}

func LoggingMiddleware() gin.HandlerFunc {
	return logger.AuditMiddleware(func(c *gin.Context, logMap map[string]string) {
		var userId uint64
		if token, ok := c.Get(internalmcp.ServiceTokenPrincipalKey); ok {
			if principal, valid := token.(*internalmcp.ServiceTokenPrincipal); valid {
				userId = principal.CreatorID
				logMap["service_token_id"] = principal.PublicID
				logMap["service_token_name"] = principal.Name
			}
		} else if user, ok := c.Get("user"); ok {
			if currentUser, valid := user.(*model.User); valid {
				userId = currentUser.ID
			}
		}
		logMap["user_id"] = cast.ToString(userId)
		sanitizeAuditLog(c, logMap)
	})
}

// redactHeaders replaces the listed headers, and any header whose name looks
// credential-bearing, with a placeholder.
func redactHeaders(headers http.Header, names ...string) {
	for _, name := range names {
		if headers.Get(name) != "" {
			headers.Set(name, redactedValue)
		}
	}
	for name := range headers {
		if isCredentialName(name) {
			headers[name] = []string{redactedValue}
		}
	}
}

func isMarked(c *gin.Context, key string) bool {
	sensitive, ok := c.Get(key)
	if !ok {
		return false
	}
	isSensitive, valid := sensitive.(bool)
	return valid && isSensitive
}

func sanitizeAuditLog(c *gin.Context, logMap map[string]string) {
	responseHeaders := c.Writer.Header().Clone()
	redactHeaders(responseHeaders, "Set-Cookie", backupSecurityHeaderName)
	headers := c.Request.Header.Clone()
	redactHeaders(headers, "Authorization", "X-Node-Secret", "Cookie", "X-MFA-Pre-Auth-ID", "X-Secure-Session-ID", "X-Passkey-Pre-Auth-ID", "X-Passkey-Session-ID", "X-Current-Password", installSecretHeaderName)
	if encodedHeaders, err := json.Marshal(headers); err == nil {
		logMap["req_header"] = string(encodedHeaders)
	}

	requestURL := *c.Request.URL
	query := requestURL.Query()
	if strings.HasSuffix(requestURL.Path, "/oidc_callback") || strings.HasSuffix(requestURL.Path, "/casdoor_callback") {
		for _, name := range []string{"code", "state"} {
			if query.Has(name) {
				query.Set(name, redactedValue)
			}
		}
	}
	for name := range query {
		switch name {
		case "X-Secure-Session-ID", "token", "mfa_pre_auth", "node_secret", installSecretQueryKey:
			query.Set(name, redactedValue)
		default:
			if isCredentialName(name) {
				query.Set(name, redactedValue)
			}
		}
	}
	requestURL.RawQuery = query.Encode()
	logMap["req_url"] = requestURL.String()

	// Bodies that are kept still lose the values of credential-named fields.
	logMap["req_body"] = redactCredentialFields(logMap["req_body"])
	logMap["resp_body"] = redactCredentialFields(logMap["resp_body"])

	path := c.Request.URL.Path
	if strings.Contains(path, "/mfa/pre_auth/") || strings.Contains(path, "/otp_") || strings.Contains(path, "/recovery_codes") || strings.Contains(path, "/2fa_secure_session/") || strings.HasSuffix(path, "/login") || strings.HasSuffix(path, "/oidc_callback") || strings.HasSuffix(path, "/casdoor_callback") || strings.Contains(path, "passkey") {
		logMap["req_body"] = redactedRequestBody
		logMap["resp_body"] = redactedResponseBody
		logMap["session_logs"] = redactedSessionLogs
		if responseHeaders.Get("Location") != "" {
			responseHeaders.Set("Location", redactedValue)
		}
	}
	if encodedHeaders, err := json.Marshal(responseHeaders); err == nil {
		logMap["resp_header"] = string(encodedHeaders)
	}

	rule := sensitiveRoutes[c.Request.Method+" "+c.FullPath()]
	// Session logs hold the SQL written while handling the request, which can
	// include the same credential values the request body carried.
	if rule.request || isMarked(c, sensitiveRequestAuditKey) {
		logMap["req_body"] = redactedRequestBody
		logMap["session_logs"] = redactedSessionLogs
	}
	if rule.response || isMarked(c, sensitiveResponseAuditKey) {
		logMap["resp_body"] = redactedResponseBody
	}
}
