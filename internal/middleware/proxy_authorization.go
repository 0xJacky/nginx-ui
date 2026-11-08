package middleware

import (
	"net/http"
	"net/url"
	"reflect"
	"strings"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
)

type proxyAuthorization uint8

const (
	proxySecureSession proxyAuthorization = 1 << iota
	proxyInteractiveUser
	proxyVerifiedTwoFactor
	proxyInteractiveUserOrNode
)

// Proxy runs before subrouter middleware, so the authorization guards of a
// proxied route must run against the original caller before the node transport
// replaces it with a node principal. A route's policy is the union of the
// guards actually registered on the route, read from its handler chain, and
// the explicit entries of proxyRouteAuthorizations.

// proxyGuardAuthorizations maps each route guard of this package, by function
// name, to the checks Proxy runs for it. Route guards here are named Require*.
// A proxied route carrying a Require* handler that is not listed is refused
// before proxying, so a new guard cannot be bypassed through a remote node.
var proxyGuardAuthorizations = map[string]proxyAuthorization{
	"RequireSecureSession":            proxySecureSession,
	"RequireInteractiveUser":          proxyInteractiveUser,
	"RequireVerifiedTwoFactorOrProxy": proxySecureSession | proxyVerifiedTwoFactor,
	"RequireInteractiveUserOrProxy":   proxyInteractiveUserOrNode,
	// Checks the browser origin of a WebSocket upgrade, not the caller.
	"RequireWebSocketOrigin": 0,
	// RequireNodePrincipal is deliberately absent: its routes serve only the
	// controller that signed the request and must never be forwarded.
}

// middlewarePackagePrefix prefixes the runtime function names of this
// package, which is how Gin reports handlers in Context.HandlerNames.
var middlewarePackagePrefix = reflect.TypeFor[proxyAuthorization]().PkgPath() + "."

// proxyGuardPolicy derives the policy of the guards registered on the matched
// route. known is false when the route carries a guard Proxy cannot enforce.
func proxyGuardPolicy(handlerNames []string) (policy proxyAuthorization, known bool) {
	for _, name := range handlerNames {
		guard, ok := strings.CutPrefix(name, middlewarePackagePrefix)
		if !ok || !strings.HasPrefix(guard, "Require") {
			continue
		}
		guard, _, _ = strings.Cut(guard, ".")
		guardPolicy, ok := proxyGuardAuthorizations[guard]
		if !ok {
			return 0, false
		}
		policy |= guardPolicy
	}
	return policy, true
}

// proxyRouteAuthorizations adds caller checks that are not visible as route
// middleware, such as checks a handler runs itself, and pins the policy of
// routes explicitly. Guards registered on a route are enforced without an
// entry here. The registration audit test checks the combined policy against
// every route that router.InitRouter mounts behind the proxy.
var proxyRouteAuthorizations = buildProxyRouteAuthorizations()

func buildProxyRouteAuthorizations() map[string]proxyAuthorization {
	policies := map[string]proxyAuthorization{}
	add := func(policy proxyAuthorization, method string, paths ...string) {
		for _, path := range paths {
			policies[method+" /api/"+path] |= policy
		}
	}
	secure := proxySecureSession
	interactive := secure | proxyInteractiveUser
	verified := secure | proxyVerifiedTwoFactor
	add(secure, http.MethodPost,
		"configs", "config", "config_mkdir", "config_rename", "config_delete", "config_sync_batch", "config_sync_directory",
		"access_lists", "access_lists/:id", "access_control/batch",
		"snippets", "snippets/:file", "snippet_sync",
		"nginx/reload", "nginx/restart", "nginx/stub_status", "nginx/performance", "nginx/modules/refresh",
		"streams/:name", "streams/:name/rename", "streams/:name/enable", "streams/:name/disable", "streams/:name/duplicate", "streams/:name/advance",
		"sites/:name/advance", "auto_cert/:name", "site_navigation/order", "site_navigation/health_check/:id",
		"sites/batch/enable", "sites/batch/disable", "sites/batch/maintenance", "sites/:name/rename", "sites/:name/enable", "sites/:name/disable", "sites/:name", "sites/:name/duplicate", "sites/:name/maintenance", "sites/:name/https/check",
		"dns_credentials", "dns_credentials/:id", "certs", "certs/:id", "certs/:id/auto_renewal", "cert_import", "cert_discover_new", "cert_parse_archive", "cert_sync/paths", "self_signed_cert", "self_signed_cert/:id",
		"acme_users", "acme_users/:id", "acme_users/:id/register",
		"dns/domains", "dns/domains/:id", "dns/domains/:id/records",
		"settings", "settings/:section",
		"nodes/reload_nginx", "nodes/restart_nginx", "nodes/sync", "namespace/sync", "namespaces/:id/sync", "namespaces", "namespaces/:id", "namespaces/:id/recover", "namespaces/order",
		"auto_backup", "auto_backup/:id", "auto_backup/:id/run", "auto_backup/test_s3", "auto_backup/test_storage",
		"upstreams", "upstreams/:name", "upstream/server_state", "upstream/convert",
		"self_check/:name/fix", "nginx_log/index/rebuild", "nginx_log/settings/advanced_indexing/enable", "nginx_log/settings/advanced_indexing/disable", "site_navigation/test_health_check/:id",
	)
	add(secure, http.MethodPut, "sites", "streams", "site_navigation/health_check/sync", "cert_sync", "dns/domains/:id/records/:record_id", "dns/domains/:id/ddns", "upstream/socket/:socket")
	add(secure, http.MethodDelete, "access_lists/:id", "snippets/:file", "streams/:name", "auto_cert/:name", "sites/:name", "dns_credentials/:id", "certs/:id", "cert_sync", "acme_users/:id", "dns/domains/:id", "dns/domains/:id/records/:record_id", "dns/domains/:id/ddns", "settings/auth/banned_ip", "namespaces/:id", "auto_backup/:id", "upstreams/:name")
	add(secure, http.MethodPatch, "acme_users/:id", "auto_backup/:id")
	add(secure, http.MethodPut, "ingresses", "ingresses/:name")
	add(secure, http.MethodDelete, "ingresses/:name")
	add(secure, http.MethodPatch, "ingresses/:name/backends/:backend/servers")
	add(secure, http.MethodPost, "ingresses/:name/reconcile")
	add(secure, http.MethodGet, "domain/:name/cert", "nodes/:id/domain/:name/cert", "certs/:id/revoke", "auto_backup/:id/stored", "sites/:name/https", "upgrade/perform", "host/setup/preview", "host/setup/publickey")
	add(secure, http.MethodPost, "host/setup/preview")
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodHead, http.MethodOptions, http.MethodDelete, http.MethodConnect, http.MethodTrace} {
		add(secure, method, "system/restart")
	}
	add(interactive, http.MethodGet, "settings/protected", "nodes/:id/secret", "nodes/:id/credentials", "node/credentials")
	add(proxyInteractiveUser, http.MethodGet, "service_tokens", "mcp/tokens")
	add(interactive, http.MethodPost, "users/:id/mfa/reset")
	add(interactive, http.MethodPost, "auto_backup/:id/stored/restore", "nodes", "nodes/:id", "nodes/load_from_settings", "nodes/:id/credentials/rotate", "nodes/:id/auth-upgrade/retry", "users", "users/:id", "external_notifies", "external_notifies/:id", "external_notifies/test", "service_tokens", "service_tokens/:id/rotate", "mcp/tokens", "mcp/tokens/:id/rotate")
	add(interactive, http.MethodDelete, "auto_backup/:id/stored", "nodes/:id", "node/credentials/:credential_id", "users/:id", "external_notifies/:id", "service_tokens/:id", "mcp/tokens/:id")
	add(interactive, http.MethodPatch, "nodes/:id", "users/:id", "external_notifies/:id")
	add(verified, http.MethodPost, "settings/nginx/control", "settings/nginx/private-key", "host/setup/keypair", "host/setup/connection", "host/setup/discover", "host/setup/diagnose", "host/setup/verify", "host/setup/host-key/scan", "host/setup/host-key/trust", "host/setup/host-key/replace")
	add(verified, http.MethodDelete, "host/setup/keypair", "host/setup/host-key")
	add(verified, http.MethodGet, "host/setup/ssh-targets")
	return policies
}

func authorizeProxyRequest(c *gin.Context) bool {
	// The policy below belongs to the route matched here. A path a proxy in
	// front of the child could resolve to another route is never forwarded.
	if !isCanonicalProxyPath(c.Request) {
		logger.GetLogger().Warnw("Refusing to proxy a request whose path is not canonical",
			"method", c.Request.Method, "route", proxyRoute(c.FullPath()))
		// Not 403: the web UI signs out on a 403 carrying its token, and a
		// malformed path says nothing about the caller's session.
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrProxyPathNotCanonical)
		return false
	}
	guardPolicy, known := proxyGuardPolicy(c.HandlerNames())
	if !known {
		logger.GetLogger().Errorw("Refusing to proxy a route carrying an authorization guard the node proxy does not enforce",
			"method", c.Request.Method, "route", proxyRoute(c.FullPath()))
		// Not 403: the web UI signs out on a 403 carrying its token, and this
		// refusal says nothing about the caller's session. 503 matches how the
		// proxy reports a child node's refusal.
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, ErrProxyGuardNotEnforced)
		return false
	}
	policy := guardPolicy | proxyRouteAuthorizations[c.Request.Method+" "+c.FullPath()]
	if policy&proxyInteractiveUser != 0 && !verifyInteractiveUser(c) {
		return false
	}
	if _, isServiceToken := c.Get(internalmcp.ServiceTokenPrincipalKey); isServiceToken && policy&proxyInteractiveUserOrNode != 0 {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"message": "This operation requires an interactive administrator",
		})
		return false
	}
	if policy&proxySecureSession != 0 && !verifySecureSession(c) {
		return false
	}
	if policy&proxyVerifiedTwoFactor != 0 && !VerifiedTwoFactorOrProxy(c, "Two-factor authentication is required for this operation") {
		return false
	}
	return true
}

// proxyVerifiedTwoFactorQuery is overwritten by every proxy before the node
// transport signs the complete query. It carries only a verified result, never
// a session credential. Absence identifies genuine node-originated operations.
const proxyVerifiedTwoFactorQuery = "__nginx_ui_proxy_verified_two_factor"

func nodeProxyVerifiedTwoFactor(c *gin.Context) bool {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return false
	}
	proof, present := values[proxyVerifiedTwoFactorQuery]
	if !present {
		return true
	}
	return len(proof) == 1 && proof[0] == "true"
}

func setProxyVerifiedTwoFactor(request *http.Request, c *gin.Context) {
	values := request.URL.Query()
	// Only the public-key reader has a conditional child-side stronger gate.
	// Other endpoints are authorized entirely before proxying; remove any
	// client-supplied reserved field without adding metadata to those APIs.
	if c.FullPath() != "/api/host/setup/publickey" {
		values.Del(proxyVerifiedTwoFactorQuery)
		request.URL.RawQuery = values.Encode()
		return
	}
	verified, _ := c.Get(SecureSessionVerifiedKey)
	hasProof := verified == true
	if _, isNode := c.Get(nodeauth.GinPrincipalKey); isNode {
		hasProof = nodeProxyVerifiedTwoFactor(c)
	}
	// Set replaces all inbound duplicates: clients cannot claim their own proof.
	values.Set(proxyVerifiedTwoFactorQuery, "false")
	if hasProof {
		values.Set(proxyVerifiedTwoFactorQuery, "true")
	}
	request.URL.RawQuery = values.Encode()
}
