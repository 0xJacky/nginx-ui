package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

const proxyUpstreamStatusHeader = "X-Nginx-UI-Upstream-Status"

const maxServiceTokenRedactionBodySize = 16 << 20

// serviceTokenRedactedResponseFields removes, for every service token, the
// stored credentials a child node returns in full to the controller's node
// principal. Each entry mirrors a local handler that withholds the same fields
// from service tokens; TestServiceTokenAwareHandlersHaveProxyRedaction keeps
// the two in step. The fields are removed from the answer of every method on
// the route. Locally the answer to an api:write token's create or modify is
// the whole stored record, these fields included; relayed it comes without
// them, which is stricter.
var serviceTokenRedactedResponseFields = map[string][]string{
	"/api/acme_users":               {"eab_key_id", "eab_hmac_key"},
	"/api/acme_users/:id":           {"eab_key_id", "eab_hmac_key"},
	"/api/auto_backup":              {"s3_access_key_id", "s3_secret_access_key", "storage_config"},
	"/api/auto_backup/:id":          {"s3_access_key_id", "s3_secret_access_key", "storage_config"},
	"/api/blocklist_sources":        {"config"},
	"/api/blocklist_sources/:id":    {"config"},
	"/api/cert_deploy_targets":      {"config"},
	"/api/cert_deploy_targets/:id":  {"config"},
	"/api/dns_credentials":          {"config", "configuration", "links"},
	"/api/dns_credentials/:id":      {"config", "configuration", "links"},
	"/api/upstream_discoveries":     {"config"},
	"/api/upstream_discoveries/:id": {"config"},
}

// serviceTokenProxyRedactions rewrites, for every service token, the values a
// child node returns in full to the controller's node principal. Unlike
// serviceTokenRedactedResponseFields the keys stay and the values are replaced
// with the shared placeholder, mirroring the local handler, so a client that
// saves the record back sends the placeholder the child knows to keep. The
// form values of a plugin probe kind (a site's probe_config) are hidden from
// api:write tokens too, because the plugin defines that form and any of its
// values may be a credential.
var serviceTokenProxyRedactions = map[string]proxyJSONRedactor{
	"/api/site_navigation":                  redactJSONListItems(redactJSONValues("probe_config")),
	"/api/site_navigation/health_check/:id": redactJSONValues("probe_config"),
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

		if !authorizeProxyRequest(c) {
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
		if refuseDisabledProxyNode(c, node) {
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
		hidesStoredSecrets := internalmcp.IsReadOnlyServiceTokenRequest(c)
		configureProxyDirector(proxy, proxyRequiresIdentityEncoding(route, isServiceToken, hidesStoredSecrets))
		director := proxy.Director
		proxy.Director = func(request *http.Request) {
			director(request)
			setProxyVerifiedTwoFactor(request, c)
			setProxyReadOnlyServiceToken(request, route, hidesStoredSecrets)
		}
		configureProxyResponse(proxy, id, route, isServiceToken, hidesStoredSecrets, logger.GetLogger().Warnw)

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

// refuseDisabledProxyNode refuses a request that selects a disabled node,
// before anything is sent to the node or read from the request body. The
// relationship maintenance job skips disabled nodes, so such a node may keep
// an outdated authentication method and must not stay reachable here.
func refuseDisabledProxyNode(c *gin.Context, node *model.Node) bool {
	if node.Enabled {
		return false
	}
	logger.GetLogger().Warnw("Refusing to proxy a request to a disabled node",
		"node_id", node.ID, "method", c.Request.Method, "route", proxyRoute(c.FullPath()))
	// Not 403: the web UI signs out on a 403 carrying its token, and a
	// disabled node says nothing about the caller's session. 503 matches how
	// the proxy reports a node it cannot reach.
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, ErrProxyNodeDisabled)
	return true
}

func configureProxyResponse(
	proxy *httputil.ReverseProxy,
	nodeID uint64,
	route string,
	isServiceToken bool,
	hidesStoredSecrets bool,
	warn proxyLogFunc,
) {
	proxy.ModifyResponse = func(resp *http.Response) error {
		upstreamStatus := normalizeProxyResponse(resp)
		childWithheldSecretFiles := takeSecretFilesWithheld(resp)
		if hidesStoredSecrets {
			relayChildSecretFileRefusal(resp, route, upstreamStatus, childWithheldSecretFiles)
		}
		if upstreamStatus >= http.StatusOK && upstreamStatus < http.StatusMultipleChoices {
			if isServiceToken {
				if err := redactServiceTokenProxyResponse(resp, route); err != nil {
					return err
				}
				if err := rewriteProxyJSONResponse(resp, route, serviceTokenProxyRedactions[route]); err != nil {
					return err
				}
			}
			if hidesStoredSecrets {
				if err := redactReadOnlyServiceTokenProxyResponse(resp, route, childWithheldSecretFiles); err != nil {
					return err
				}
			}
		}
		if shouldLogProxyResponse(upstreamStatus) {
			logProxyResponse(warn, nodeID, route, resp, upstreamStatus)
		}
		return nil
	}
}

// proxyRequiresIdentityEncoding asks the child for an uncompressed body when
// the controller has to rewrite the response before relaying it.
func proxyRequiresIdentityEncoding(route string, isServiceToken, hidesStoredSecrets bool) bool {
	return (isServiceToken && serviceTokenRedactedResponseFields[route] != nil) ||
		(isServiceToken && serviceTokenProxyRedactions[route] != nil) ||
		(hidesStoredSecrets && readOnlyServiceTokenProxyRedactions[route] != nil)
}

// readProxyResponseBody reads a relayed body the controller rewrites before
// relaying it.
func readProxyResponseBody(response *http.Response, route string) ([]byte, error) {
	if response.Body == nil {
		return nil, fmt.Errorf("redact service token response for %s: response body is missing", route)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxServiceTokenRedactionBodySize+1))
	closeErr := response.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("redact service token response for %s: %w", route, err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("redact service token response for %s: %w", route, closeErr)
	}
	if len(body) > maxServiceTokenRedactionBodySize {
		return nil, fmt.Errorf("redact service token response for %s: response body is too large", route)
	}
	return body, nil
}

func readProxyJSONResponse(response *http.Response, route string) (map[string]any, error) {
	body, err := readProxyResponseBody(response, route)
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("redact service token response for %s: %w", route, err)
	}
	return payload, nil
}

func writeProxyJSONResponse(response *http.Response, route string, payload any) error {
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

// redactServiceTokenProxyResponse removes the route's stored credentials from
// a child's successful answer, whatever the method. A list answers
// {"data": [...]}, a read, create or modify answers the record itself, and a
// delete or recover answers 204 without a body. A body that is empty, or is
// uncompressed, declared as something other than JSON and does not parse as
// JSON, holds no field to remove and is relayed as it is. Anything else that
// does not parse is refused rather than relayed unredacted.
func redactServiceTokenProxyResponse(response *http.Response, route string) error {
	fields := serviceTokenRedactedResponseFields[route]
	if len(fields) == 0 || response.Body == nil {
		return nil
	}

	body, err := readProxyResponseBody(response, route)
	if err != nil {
		return err
	}
	relayUnchanged := func() error {
		response.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return relayUnchanged()
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		if !declaresJSON(response) && !isContentEncoded(response) {
			return relayUnchanged()
		}
		return fmt.Errorf("redact service token response for %s: %w", route, err)
	}
	if err := redactServiceTokenRecords(payload, fields); err != nil {
		return fmt.Errorf("redact service token response for %s: %w", route, err)
	}
	return writeProxyJSONResponse(response, route, payload)
}

// declaresJSON reports whether a response does not rule out a JSON body: it
// names a JSON media type, or none at all.
func declaresJSON(response *http.Response) bool {
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return true
	}
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

// isContentEncoded reports whether a body is compressed or otherwise encoded,
// which hides any field it holds from the JSON check.
func isContentEncoded(response *http.Response) bool {
	for _, encoding := range response.Header.Values("Content-Encoding") {
		if value := strings.TrimSpace(encoding); value != "" && !strings.EqualFold(value, "identity") {
			return true
		}
	}
	return false
}

// redactServiceTokenRecords removes fields from a relayed record and from
// every record it lists under "data". A bare array is a list of records too.
func redactServiceTokenRecords(payload any, fields []string) error {
	switch value := payload.(type) {
	case map[string]any:
		redactResponseFields(value, fields)
		switch data := value["data"].(type) {
		case []any:
			return redactServiceTokenRecords(data, fields)
		case map[string]any:
			redactResponseFields(data, fields)
		}
	case []any:
		for _, item := range value {
			record, ok := item.(map[string]any)
			if !ok {
				return errProxyRedactionShape
			}
			redactResponseFields(record, fields)
		}
	}
	return nil
}

func redactResponseFields(payload map[string]any, fields []string) {
	for _, field := range fields {
		delete(payload, field)
	}
}

// redactReadOnlyServiceTokenProxyResponse applies the read-only rule to a
// child node's response. A refused read replaces the response with the same
// cosy error the local handler returns. On a route the child classifies
// itself, a child that did not confirm doing so gets the stricter redaction.
func redactReadOnlyServiceTokenProxyResponse(response *http.Response, route string, childWithheldSecretFiles bool) error {
	redact := readOnlyServiceTokenProxyRedactions[route]
	if unconfirmed, classifiedByChild := readOnlyServiceTokenUnconfirmedProxyRedactions[route]; classifiedByChild && !childWithheldSecretFiles {
		redact = unconfirmed
	}
	return rewriteProxyJSONResponse(response, route, redact)
}

// rewriteProxyJSONResponse applies redact to a child's JSON answer. A nil
// redact leaves the answer alone. A refusal replaces the answer with the cosy
// error the local handler returns.
func rewriteProxyJSONResponse(response *http.Response, route string, redact proxyJSONRedactor) error {
	if redact == nil {
		return nil
	}

	payload, err := readProxyJSONResponse(response, route)
	if err != nil {
		return err
	}
	if err := redact(payload); err != nil {
		var refusal *cosy.Error
		if !errors.As(err, &refusal) {
			return fmt.Errorf("redact service token response for %s: %w", route, err)
		}
		// A 403 is allowed here although proxy refusals avoid it: this runs
		// only for a service token, never for the web UI's login token, so it
		// cannot sign the web UI out.
		response.StatusCode = http.StatusForbidden
		response.Status = strconv.Itoa(http.StatusForbidden) + " " + http.StatusText(http.StatusForbidden)
		response.Header.Set("Content-Type", "application/json; charset=utf-8")
		return writeProxyJSONResponse(response, route, refusal)
	}
	return writeProxyJSONResponse(response, route, payload)
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
		// The node transport signs the request as the controller's node after
		// the director runs; the caller's own credentials must not travel on.
		// The director runs before hop-by-hop headers are removed, so an
		// upgrade is still recognizable here.
		stripCallerCredentials(req.Header, query, IsWebSocketUpgrade(req))
		req.URL.RawQuery = query.Encode()
		req.Header.Del("X-Node-ID")
		// Only a node's own synchronization may mark a request as replicated;
		// a change a person makes on the target node must still fan out there.
		req.Header.Del(nodeauth.ReplicatedFromHeader)
		if requireIdentityEncoding {
			req.Header.Set("Accept-Encoding", "identity")
		}
	}
}
