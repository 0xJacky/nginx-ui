package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
)

const proxyUpstreamStatusHeader = "X-Nginx-UI-Upstream-Status"

const maxServiceTokenRedactionBodySize = 16 << 20

var serviceTokenRedactedResponseFields = map[string][]string{
	"/api/acme_users":          {"eab_key_id", "eab_hmac_key"},
	"/api/acme_users/:id":      {"eab_key_id", "eab_hmac_key"},
	"/api/auto_backup":         {"s3_access_key_id", "s3_secret_access_key"},
	"/api/auto_backup/:id":     {"s3_access_key_id", "s3_secret_access_key"},
	"/api/dns_credentials":     {"config", "configuration", "links"},
	"/api/dns_credentials/:id": {"config", "configuration", "links"},
}

type proxyLogFunc func(message string, keysAndValues ...any)

func Proxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeID, ok := c.Get("ProxyNodeID")
		if !ok {
			c.Next()
			return
		}
		id := cast.ToUint64(nodeID)
		if id == 0 {
			c.Next()
			return
		}

		defer c.Abort()

		nodeQuery := query.Node

		node, err := nodeQuery.Where(nodeQuery.ID.Eq(id)).First()
		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"message": err.Error(),
			})
			return
		}

		baseUrl, err := url.Parse(node.URL)
		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(baseUrl)

		customTransport, err := transport.NewTransport()
		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		proxy.Transport = nodeauth.NewTransport(node, customTransport)

		route := c.FullPath()
		isServiceToken := internalmcp.IsServiceTokenRequest(c)
		configureProxyDirector(proxy, isServiceToken && serviceTokenRedactedResponseFields[route] != nil)
		configureProxyResponse(proxy, id, route, isServiceToken, logger.GetLogger().Warnw)

		proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
		}

		logger.GetLogger().Debugw("Node proxy request",
			"node_id", id,
			"method", c.Request.Method,
			"route", proxyRoute(route),
			"target_authority", boundedLogValue(baseUrl.Host),
		)

		// resolve proxy panic when client disconnect
		ctx := context.WithValue(
			c.Request.Context(),
			http.ServerContextKey,
			nil,
		)
		req := c.Request.Clone(ctx)

		proxy.ServeHTTP(c.Writer, req)
	}
}

func configureProxyResponse(
	proxy *httputil.ReverseProxy,
	nodeID uint64,
	route string,
	isServiceToken bool,
	warn proxyLogFunc,
) {
	proxy.ModifyResponse = func(resp *http.Response) error {
		upstreamStatus := normalizeProxyResponse(resp)
		if isServiceToken && upstreamStatus >= http.StatusOK && upstreamStatus < http.StatusMultipleChoices {
			if err := redactServiceTokenProxyResponse(resp, route); err != nil {
				return err
			}
		}
		if shouldLogProxyResponse(upstreamStatus) {
			logProxyResponse(warn, nodeID, route, resp, upstreamStatus)
		}
		return nil
	}
}

func redactServiceTokenProxyResponse(response *http.Response, route string) error {
	fields := serviceTokenRedactedResponseFields[route]
	if len(fields) == 0 {
		return nil
	}
	if response.Body == nil {
		return fmt.Errorf("redact service token response for %s: response body is missing", route)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxServiceTokenRedactionBodySize+1))
	closeErr := response.Body.Close()
	if err != nil {
		return fmt.Errorf("redact service token response for %s: %w", route, err)
	}
	if closeErr != nil {
		return fmt.Errorf("redact service token response for %s: %w", route, closeErr)
	}
	if len(body) > maxServiceTokenRedactionBodySize {
		return fmt.Errorf("redact service token response for %s: response body is too large", route)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("redact service token response for %s: %w", route, err)
	}
	if strings.HasSuffix(route, "/:id") {
		redactResponseFields(payload, fields)
	} else {
		items, ok := payload["data"].([]any)
		if !ok {
			return fmt.Errorf("redact service token response for %s: list data is invalid", route)
		}
		for _, value := range items {
			item, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("redact service token response for %s: list item is invalid", route)
			}
			redactResponseFields(item, fields)
		}
	}

	redactedBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("redact service token response for %s: %w", route, err)
	}
	response.Body = io.NopCloser(bytes.NewReader(redactedBody))
	response.ContentLength = int64(len(redactedBody))
	response.Header.Set("Content-Length", strconv.FormatInt(response.ContentLength, 10))
	response.Header.Del("ETag")
	return nil
}

func redactResponseFields(payload map[string]any, fields []string) {
	for _, field := range fields {
		delete(payload, field)
	}
}

func normalizeProxyResponse(resp *http.Response) int {
	upstreamStatus := resp.StatusCode
	if resp.Header == nil {
		resp.Header = make(http.Header)
	}
	resp.Header.Del(proxyUpstreamStatusHeader)

	// Preserve the existing anti-logout behaviour while exposing the original
	// upstream status for diagnostics.
	if upstreamStatus == http.StatusForbidden {
		resp.StatusCode = http.StatusServiceUnavailable
		resp.Header.Set(proxyUpstreamStatusHeader, strconv.Itoa(upstreamStatus))
	}

	// resolve CORS header duplication issue
	resp.Header.Del("Access-Control-Allow-Origin")
	resp.Header.Del("Access-Control-Allow-Methods")
	resp.Header.Del("Access-Control-Allow-Headers")
	resp.Header.Del("Access-Control-Expose-Headers")
	resp.Header.Del("Access-Control-Max-Age")
	resp.Header.Del("Access-Control-Allow-Credentials")

	return upstreamStatus
}

func shouldLogProxyResponse(status int) bool {
	return status == http.StatusForbidden ||
		status == http.StatusTeapot ||
		status == http.StatusTooManyRequests
}

func logProxyResponse(warn proxyLogFunc, nodeID uint64, route string, resp *http.Response, upstreamStatus int) {
	if warn == nil {
		return
	}

	request := resp.Request
	method, targetAuthority, outboundAuthority := "", "", ""
	if request != nil && request.URL != nil {
		method = request.Method
		targetAuthority = request.URL.Host
		outboundAuthority = request.Host
	}

	hostMatchesTarget := targetAuthority != "" && targetAuthority == outboundAuthority
	edgeChallenge := strings.EqualFold(resp.Header.Get("cf-mitigated"), "challenge")

	warn("Node proxy upstream response rejected",
		"node_id", nodeID,
		"method", boundedLogValue(method),
		"route", proxyRoute(route),
		"target_authority", boundedLogValue(targetAuthority),
		"outbound_authority", boundedLogValue(outboundAuthority),
		"host_matches_target", hostMatchesTarget,
		"upstream_status", upstreamStatus,
		"downstream_status", resp.StatusCode,
		"classification", proxyResponseClassification(upstreamStatus, hostMatchesTarget, edgeChallenge),
		"content_type", boundedLogValue(resp.Header.Get("Content-Type")),
		"content_length", resp.ContentLength,
		"server", boundedLogValue(resp.Header.Get("Server")),
		"cf_ray", boundedLogValue(resp.Header.Get("CF-Ray")),
		"edge_challenge", edgeChallenge,
		"retry_after", boundedLogValue(resp.Header.Get("Retry-After")),
	)
}

func proxyResponseClassification(status int, hostMatchesTarget, edgeChallenge bool) string {
	if !hostMatchesTarget {
		return "host_mismatch"
	}
	if edgeChallenge {
		return "edge_challenge"
	}
	if status == http.StatusTooManyRequests {
		return "rate_limited"
	}
	return "upstream_rejected"
}

func proxyRoute(route string) string {
	if route == "" {
		return "unknown"
	}
	return boundedLogValue(route)
}

func boundedLogValue(value string) string {
	value = strings.TrimSpace(strings.ToValidUTF8(value, string(utf8.RuneError)))
	runes := []rune(value)
	if len(runes) > 128 {
		return string(runes[:128])
	}
	return value
}

func configureProxyDirector(proxy *httputil.ReverseProxy, requireIdentityEncoding bool) {
	defaultDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		defaultDirector(req)
		req.Host = req.URL.Host
		// drop proxy identifier from upstream query to avoid leaking internal ids
		query := req.URL.Query()
		query.Del("x_node_id")
		req.URL.RawQuery = query.Encode()
		req.Header.Del("X-Node-ID")
		if requireIdentityEncoding {
			req.Header.Set("Accept-Encoding", "identity")
		}
	}
}
