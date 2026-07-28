package audit

import (
	"encoding/json"

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
		if user, ok := c.Get("user"); ok {
			userId = user.(*model.User).ID
		}
		logMap["user_id"] = cast.ToString(userId)
		sanitizeAuditLog(c, logMap)
	})
}

func sanitizeAuditLog(c *gin.Context, logMap map[string]string) {
	headers := c.Request.Header.Clone()
	for _, name := range []string{"Authorization", "X-Node-Secret"} {
		if headers.Get(name) != "" {
			headers.Set(name, "[REDACTED]")
		}
	}
	if encodedHeaders, err := json.Marshal(headers); err == nil {
		logMap["req_header"] = string(encodedHeaders)
	}

	requestURL := *c.Request.URL
	query := requestURL.Query()
	if query.Has("node_secret") {
		query.Set("node_secret", "[REDACTED]")
		requestURL.RawQuery = query.Encode()
		logMap["req_url"] = requestURL.String()
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
