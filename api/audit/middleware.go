package audit

import (
	"encoding/json"
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

func sanitizeAuditLog(c *gin.Context, logMap map[string]string) {
	responseHeaders := c.Writer.Header().Clone()
	if responseHeaders.Get("Set-Cookie") != "" {
		responseHeaders.Set("Set-Cookie", "[REDACTED]")
	}
	headers := c.Request.Header.Clone()
	for _, name := range []string{"Authorization", "X-Node-Secret", "Cookie", "X-MFA-Pre-Auth-ID", "X-Secure-Session-ID", "X-Passkey-Pre-Auth-ID", "X-Passkey-Session-ID", "X-Current-Password"} {
		if headers.Get(name) != "" {
			headers.Set(name, "[REDACTED]")
		}
	}
	if encodedHeaders, err := json.Marshal(headers); err == nil {
		logMap["req_header"] = string(encodedHeaders)
	}

	requestURL := *c.Request.URL
	query := requestURL.Query()
	if strings.HasSuffix(requestURL.Path, "/oidc_callback") || strings.HasSuffix(requestURL.Path, "/casdoor_callback") {
		for _, name := range []string{"code", "state"} {
			if query.Has(name) {
				query.Set(name, "[REDACTED]")
			}
		}
	}
	for _, name := range []string{"X-Secure-Session-ID", "token", "mfa_pre_auth"} {
		if query.Has(name) {
			query.Set(name, "[REDACTED]")
		}
	}
	if query.Has("node_secret") {
		query.Set("node_secret", "[REDACTED]")
		requestURL.RawQuery = query.Encode()
		logMap["req_url"] = requestURL.String()
	}
	requestURL.RawQuery = query.Encode()
	logMap["req_url"] = requestURL.String()
	path := c.Request.URL.Path
	if strings.Contains(path, "/mfa/pre_auth/") || strings.Contains(path, "/otp_") || strings.Contains(path, "/recovery_codes") || strings.Contains(path, "/2fa_secure_session/") || strings.HasSuffix(path, "/login") || strings.HasSuffix(path, "/oidc_callback") || strings.HasSuffix(path, "/casdoor_callback") || strings.Contains(path, "passkey") {
		logMap["req_body"] = "[sensitive request redacted]"
		logMap["resp_body"] = "[sensitive response redacted]"
		logMap["session_logs"] = "[sensitive session logs redacted]"
		if responseHeaders.Get("Location") != "" {
			responseHeaders.Set("Location", "[REDACTED]")
		}
	}
	if encodedHeaders, err := json.Marshal(responseHeaders); err == nil {
		logMap["resp_header"] = string(encodedHeaders)
	}
	if sensitive, ok := c.Get(sensitiveRequestAuditKey); ok {
		if isSensitive, valid := sensitive.(bool); valid && isSensitive {
			logMap["req_body"] = "[sensitive request redacted]"
		}
	}

	if sensitive, ok := c.Get(sensitiveResponseAuditKey); ok {
		if isSensitive, valid := sensitive.(bool); valid && isSensitive {
			logMap["resp_body"] = "[sensitive response redacted]"
		}
	}
}
