package audit

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
	cosySettings "github.com/uozi-tech/cosy/settings"
)

const (
	sensitiveRequestAuditKey  = "SensitiveRequestAudit"
	sensitiveResponseAuditKey = "SensitiveResponseAudit"
	// loggingInstalledKey marks a request that an outer LoggingMiddleware
	// already audits.
	loggingInstalledKey = "AuditLoggingInstalled"

	// auditContextHoldTimeout bounds how long a request waits for its audit
	// goroutine to pick up the snapshot. The goroutine normally starts within
	// microseconds; the bound only keeps a stuck goroutine from hanging the
	// request.
	auditContextHoldTimeout = 2 * time.Second

	redactedAuditContext = "[audit context unavailable, redacted]"
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

// requestSnapshot is everything the sanitizer needs from the gin context,
// copied while the request is still being served.
type requestSnapshot struct {
	method            string
	fullPath          string
	url               url.URL
	requestHeader     http.Header
	responseHeader    http.Header
	sensitiveRequest  bool
	sensitiveResponse bool
	proxied           bool
	userID            uint64
	serviceTokenID    string
	serviceTokenName  string
	hasServiceToken   bool
}

func newRequestSnapshot(c *gin.Context) *requestSnapshot {
	snapshot := &requestSnapshot{
		method:            c.Request.Method,
		fullPath:          c.FullPath(),
		url:               *c.Request.URL,
		requestHeader:     c.Request.Header.Clone(),
		responseHeader:    c.Writer.Header().Clone(),
		sensitiveRequest:  isMarked(c, sensitiveRequestAuditKey),
		sensitiveResponse: isMarked(c, sensitiveResponseAuditKey),
	}
	if nodeID, ok := c.Get("ProxyNodeID"); ok && cast.ToUint64(nodeID) != 0 {
		snapshot.proxied = true
	}
	if token, ok := c.Get(internalmcp.ServiceTokenPrincipalKey); ok {
		if principal, valid := token.(*internalmcp.ServiceTokenPrincipal); valid {
			snapshot.hasServiceToken = true
			snapshot.userID = principal.CreatorID
			snapshot.serviceTokenID = principal.PublicID
			snapshot.serviceTokenName = principal.Name
		}
	} else if user, ok := c.Get("user"); ok {
		if currentUser, valid := user.(*model.User); valid {
			snapshot.userID = currentUser.ID
		}
	}
	return snapshot
}

// pendingAudit pairs one request with its audit goroutine.
type pendingAudit struct {
	ready    chan struct{} // closed once snapshot is set
	released chan struct{} // closed once the goroutine is done with the context
	snapshot *requestSnapshot
}

// pendingAudits maps a cosy request ID to its pendingAudit. Whichever side
// arrives first creates the entry; the audit goroutine removes it, or the
// request does when the goroutine never arrives, so entries cannot pile up.
var pendingAudits sync.Map

func pendingAuditFor(requestID string) *pendingAudit {
	entry, _ := pendingAudits.LoadOrStore(requestID, &pendingAudit{
		ready:    make(chan struct{}),
		released: make(chan struct{}),
	})
	return entry.(*pendingAudit)
}

// auditSinkConfigured reports whether a finished audit entry is sent
// anywhere. cosy only hands the entry to the monitor reporter and to SLS;
// without either, nothing reads the sanitized copy.
func auditSinkConfigured(reporter logger.MonitorReporter, sls *cosySettings.SLS) bool {
	return reporter != nil || sls.Enable()
}

func hasAuditSink() bool {
	return auditSinkConfigured(logger.GetMonitorReporter(), cosySettings.SLSSettings)
}

// LoggingMiddleware records the audit log through cosy and sanitizes it.
//
// cosy builds the log entry in a goroutine that starts after the handlers
// return and still reads the gin context, which gin recycles for the next
// request as soon as this middleware returns. Reading the context there could
// apply another request's redaction rules to this request's bodies. So, once
// the handlers are done, the request copies what the sanitizer needs into a
// snapshot and hands it to the goroutine by request ID, then waits until the
// goroutine has picked it up, which is after cosy's own reads of the context.
// The sanitizer itself never touches the context.
//
// Without an audit sink the snapshot and the sanitizer are skipped. The
// request still waits for the goroutine, because cosy itself reads the context
// there. The sinks are configured at startup; if one appears while a request
// is in flight, that request's entry is redacted as if its snapshot were lost.
//
// Installing the middleware twice is harmless: the inner copy only passes the
// request on, so the request is audited once and does not wait for a snapshot
// pickup that never comes.
func LoggingMiddleware() gin.HandlerFunc {
	audit := logger.AuditMiddleware(func(_ *gin.Context, logMap map[string]string) {
		handleAuditLog(logMap)
	})
	return func(c *gin.Context) {
		if _, installed := c.Get(loggingInstalledKey); installed {
			c.Next()
			return
		}
		c.Set(loggingInstalledKey, true)

		audit(c)

		if skip, ok := c.Get(logger.CosySkipAuditKey); ok {
			if skipped, valid := skip.(bool); valid && skipped {
				// cosy will not call the log handler, so there is nobody to wait for.
				return
			}
		}
		requestID := c.GetString(logger.CosyRequestIDKey)
		if requestID == "" {
			return
		}

		pending := pendingAuditFor(requestID)
		if hasAuditSink() {
			pending.snapshot = newRequestSnapshot(c)
		}
		close(pending.ready)

		timer := time.NewTimer(auditContextHoldTimeout)
		defer timer.Stop()
		select {
		case <-pending.released:
		case <-timer.C:
			// The goroutine never arrived; drop the entry so it cannot leak. A
			// late goroutine then finds no snapshot and redacts everything.
			pendingAudits.CompareAndDelete(requestID, pending)
		}
	}
}

// handleAuditLog runs on cosy's audit goroutine and must not read the gin
// context.
func handleAuditLog(logMap map[string]string) {
	requestID := logMap[logger.FieldRequestID]
	if requestID == "" {
		redactWithoutSnapshot(logMap)
		return
	}

	pending := pendingAuditFor(requestID)
	timer := time.NewTimer(auditContextHoldTimeout)
	var snapshot *requestSnapshot
	select {
	case <-pending.ready:
		snapshot = pending.snapshot
	case <-timer.C:
	}
	timer.Stop()
	pendingAudits.CompareAndDelete(requestID, pending)
	// cosy has finished reading the context before calling this handler, so
	// the request may now return and gin may recycle the context.
	close(pending.released)

	if !hasAuditSink() {
		// Nobody reads the entry, so there is nothing to sanitize.
		return
	}
	if snapshot == nil {
		redactWithoutSnapshot(logMap)
		return
	}

	logMap["user_id"] = cast.ToString(snapshot.userID)
	if snapshot.hasServiceToken {
		logMap["service_token_id"] = snapshot.serviceTokenID
		logMap["service_token_name"] = snapshot.serviceTokenName
	}
	sanitizeAuditLog(snapshot, logMap)
}

// redactWithoutSnapshot fails closed when the request that produced logMap
// cannot be identified: nothing that could carry a credential is kept.
func redactWithoutSnapshot(logMap map[string]string) {
	logMap["req_body"] = redactedRequestBody
	logMap["resp_body"] = redactedResponseBody
	logMap["session_logs"] = redactedSessionLogs
	logMap["req_header"] = redactedAuditContext
	logMap["resp_header"] = redactedAuditContext
	if requestURL, err := url.Parse(logMap["req_url"]); err == nil {
		requestURL.RawQuery = ""
		logMap["req_url"] = requestURL.String()
	} else {
		logMap["req_url"] = redactedAuditContext
	}
	logMap["user_id"] = "0"
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

func sanitizeAuditLog(snapshot *requestSnapshot, logMap map[string]string) {
	responseHeaders := snapshot.responseHeader.Clone()
	redactHeaders(responseHeaders, "Set-Cookie", backupSecurityHeaderName)
	headers := snapshot.requestHeader.Clone()
	redactHeaders(headers, "Authorization", "Proxy-Authorization", "X-Node-Secret", "Cookie", "X-MFA-Pre-Auth-ID", "X-Secure-Session-ID", "X-Passkey-Pre-Auth-ID", "X-Passkey-Session-ID", "X-Current-Password", installSecretHeaderName)
	if encodedHeaders, err := json.Marshal(headers); err == nil {
		logMap["req_header"] = string(encodedHeaders)
	}

	requestURL := snapshot.url
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

	// Decisions that look at the bodies use the raw capture.
	rawRequestBody, rawResponseBody := logMap["req_body"], logMap["resp_body"]
	secretConfig := targetsSecretConfigFile(snapshot, rawRequestBody, rawResponseBody)

	// Bodies that are kept still lose the values of credential-named fields.
	logMap["req_body"] = redactCredentialFields(rawRequestBody)
	logMap["resp_body"] = redactCredentialFields(rawResponseBody)

	path := requestURL.Path
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

	rule := sensitiveRoutes[snapshot.method+" "+snapshot.fullPath]
	// Session logs hold the SQL run while handling the request, which can
	// include the same credential values the bodies carried, so they go
	// whenever either body does.
	if rule.request || snapshot.sensitiveRequest || secretConfig || containsPrivateKey(rawRequestBody) {
		logMap["req_body"] = redactedRequestBody
		logMap["session_logs"] = redactedSessionLogs
	}
	if rule.response || snapshot.sensitiveResponse || secretConfig || containsPrivateKey(rawResponseBody) {
		logMap["resp_body"] = redactedResponseBody
		logMap["session_logs"] = redactedSessionLogs
	}
}
