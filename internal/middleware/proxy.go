package middleware

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
)

const proxyUpstreamStatusHeader = "X-Nginx-UI-Upstream-Status"

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

		configureProxyDirector(proxy)

		route := c.FullPath()
		configureProxyResponse(proxy, id, route, logger.GetLogger().Warnw)

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

func configureProxyResponse(proxy *httputil.ReverseProxy, nodeID uint64, route string, warn proxyLogFunc) {
	proxy.ModifyResponse = func(resp *http.Response) error {
		upstreamStatus := normalizeProxyResponse(resp)
		if shouldLogProxyResponse(upstreamStatus) {
			logProxyResponse(warn, nodeID, route, resp, upstreamStatus)
		}
		return nil
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

func configureProxyDirector(proxy *httputil.ReverseProxy) {
	defaultDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		defaultDirector(req)
		req.Host = req.URL.Host
		// drop proxy identifier from upstream query to avoid leaking internal ids
		query := req.URL.Query()
		query.Del("x_node_id")
		req.URL.RawQuery = query.Encode()
		req.Header.Del("X-Node-ID")
	}
}
