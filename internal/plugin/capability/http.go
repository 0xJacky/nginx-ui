package capability

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

const (
	// httpSocketName is the unix socket a plugin's http capability listens on,
	// relative to its data directory.
	httpSocketName = "http.sock"
	// maxRPCBodyBytes bounds the request body the rpc fallback base64 encodes
	// into one JSON-RPC call.
	maxRPCBodyBytes = 4 << 20 // 4 MiB

	// headerPluginUser and headerPluginUserID identify the nginx-ui user
	// behind a proxied request. HTTP header names are case-insensitive, so
	// the exact casing here is cosmetic.
	headerPluginUser   = "X-Nginx-UI-User"
	headerPluginUserID = "X-Nginx-UI-User-ID"
)

// errRPCBodyTooLarge marks a request body over maxRPCBodyBytes.
var errRPCBodyTooLarge = errors.New("plugin http body exceeds the rpc limit")

// HTTPHost is the part of the plugin manager the http capability needs.
type HTTPHost interface {
	// Get returns one plugin, so the handler can check it is enabled and
	// declares the http capability.
	Get(id string) (*plugin.Info, error)
	// Manifest returns the manifest of an installed plugin.
	Manifest(id string) (*protocol.Manifest, bool)
	// Acquire returns a client for a plugin and a func that releases it,
	// starting an on_demand plugin. The release func is safe to call even
	// when the error is not nil.
	Acquire(ctx context.Context, id string) (jsonrpc.Caller, func(), error)
	// DataDir is the private writable directory handed to one plugin.
	DataDir(id string) string
	// InitializeResult returns what the running plugin answered during its
	// handshake, used for the Windows loopback port.
	InitializeResult(id string) (protocol.InitializeResult, bool)
}

// NewHTTPHandler exposes the http capability of a plugin as a gin route.
// Listen "unix" reverse proxies (with WebSocket and streaming support) to the
// plugin's own HTTP server, listen "rpc" falls back to a single http.handle
// call for plugins that cannot run a listener of their own.
func NewHTTPHandler(h HTTPHost) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if !plugin.IsValidID(id) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		info, err := h.Get(id)
		if err != nil || !info.Enabled || !slices.Contains(info.Capabilities, protocol.CapabilityHTTP) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		manifest, ok := h.Manifest(id)
		if !ok || manifest.HTTP == nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		subPath := c.Param("path")
		if subPath == "" {
			subPath = "/"
		}

		switch manifest.HTTP.Listen {
		case "unix":
			serveHTTPUnix(c, h, id, subPath)
		case "rpc":
			serveHTTPRPC(c, h, id, subPath)
		default:
			c.AbortWithStatus(http.StatusNotFound)
		}
	}
}

// serveHTTPUnix reverse proxies the request to the plugin's own HTTP server.
func serveHTTPUnix(c *gin.Context, h HTTPHost, id, subPath string) {
	caller, release, err := h.Acquire(c.Request.Context(), id)
	defer releaseIfSet(release)
	if err != nil {
		respondUnavailable(c.Writer, err)
		return
	}
	_ = caller // Acquire's only role here is starting an on_demand plugin.

	dial, err := dialerFor(h, id)
	if err != nil {
		respondUnavailable(c.Writer, err)
		return
	}

	user := currentUser(c)
	upgrade := middleware.IsWebSocketUpgrade(c.Request)
	proxy := &httputil.ReverseProxy{
		Transport:     &http.Transport{DialContext: dial},
		FlushInterval: -1, // stream every write immediately.
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "plugin"
			req.URL.Path = subPath
			req.URL.RawPath = ""
			if upgrade {
				stripHandshakeCredentials(req.URL)
			}
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")
			req.Header.Set(headerPluginUser, user.Name)
			req.Header.Set(headerPluginUserID, user.ID)
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			respondUnavailable(w, err)
		},
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

// stripHandshakeCredentials removes the query parameters a browser WebSocket
// uses to authenticate with the host, so the plugin never sees a session token.
func stripHandshakeCredentials(u *url.URL) {
	values := u.Query()
	values.Del("token")
	values.Del("x_node_id")
	u.RawQuery = values.Encode()
}

// dialerFor resolves how to reach the plugin's http capability: a unix socket
// everywhere, a loopback TCP port on Windows where unix sockets need the
// plugin to opt in and are not guaranteed.
func dialerFor(h HTTPHost, id string) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	if runtime.GOOS != "windows" {
		socketPath := filepath.Join(h.DataDir(id), httpSocketName)
		return func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		}, nil
	}

	result, ok := h.InitializeResult(id)
	if !ok || result.HTTPPort == 0 {
		return nil, plugin.ErrPluginNotRunning
	}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(result.HTTPPort))
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}, nil
}

// serveHTTPRPC sends one http.handle call and writes the reply back. It is
// the fallback for plugins that cannot run their own HTTP listener.
func serveHTTPRPC(c *gin.Context, h HTTPHost, id, subPath string) {
	body, err := readLimitedBody(c.Request.Body)
	if err != nil {
		if errors.Is(err, errRPCBodyTooLarge) {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	caller, release, err := h.Acquire(c.Request.Context(), id)
	defer releaseIfSet(release)
	if err != nil {
		respondUnavailable(c.Writer, err)
		return
	}

	params := protocol.HTTPHandleParams{
		Method:     c.Request.Method,
		Path:       subPath,
		Query:      c.Request.URL.RawQuery,
		Headers:    sanitizedHeaders(c.Request.Header),
		BodyBase64: base64.StdEncoding.EncodeToString(body),
		User:       currentUser(c),
	}

	var result protocol.HTTPHandleResult
	if err = caller.Call(c.Request.Context(), protocol.MethodHTTPHandle, params, &result); err != nil {
		respondUnavailable(c.Writer, plugin.WrapRPCError(err))
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(result.BodyBase64)
	if err != nil {
		respondUnavailable(c.Writer, plugin.ErrPluginHandshake)
		return
	}
	for key, values := range result.Headers {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	status := result.Status
	if status == 0 {
		status = http.StatusOK
	}
	c.Writer.WriteHeader(status)
	_, _ = c.Writer.Write(decoded)
}

// readLimitedBody reads at most maxRPCBodyBytes+1, so a body that is exactly
// on the limit is accepted and anything past it is reported.
func readLimitedBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxRPCBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxRPCBodyBytes {
		return nil, errRPCBodyTooLarge
	}
	return body, nil
}

// sanitizedHeaders copies the request headers for the rpc fallback. The user
// identity travels only through HTTPHandleParams.User, so credentials and any
// client supplied identity headers never reach the plugin.
func sanitizedHeaders(h http.Header) map[string][]string {
	out := make(map[string][]string, len(h))
	for key, values := range h {
		switch {
		case strings.EqualFold(key, "Authorization"),
			strings.EqualFold(key, "Cookie"),
			strings.EqualFold(key, headerPluginUser),
			strings.EqualFold(key, headerPluginUserID):
			continue
		}
		out[key] = values
	}
	return out
}

// currentUser reads the user AuthRequired put in the context. It returns a
// zero value for unauthenticated requests, which should not happen since the
// route sits behind AuthRequired.
func currentUser(c *gin.Context) protocol.HTTPUser {
	raw, ok := c.Get("user")
	if !ok {
		return protocol.HTTPUser{}
	}
	u, ok := raw.(*model.User)
	if !ok {
		return protocol.HTTPUser{}
	}
	return protocol.HTTPUser{ID: strconv.FormatUint(u.ID, 10), Name: u.Name}
}

// releaseIfSet calls release when Acquire returned one. Acquire always
// returns a non-nil func, this only guards test doubles that do not.
func releaseIfSet(release func()) {
	if release != nil {
		release()
	}
}

// respondUnavailable answers 503 with a cosy shaped error body, so a plugin
// client sees the same error envelope as the rest of the API.
func respondUnavailable(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(cosyError(err))
}

// cosyError extracts the cosy error shape from err, falling back to
// ErrPluginNotRunning when err does not already carry one, e.g. a dial
// failure against a missing socket.
func cosyError(err error) *cosy.Error {
	var cErr *cosy.Error
	if err != nil && errors.As(err, &cErr) {
		return cErr
	}
	errors.As(plugin.ErrPluginNotRunning, &cErr)
	return cErr
}
