package middleware

import (
	"net/http"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/pretty66/websocketproxy"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
)

func ProxyWs() gin.HandlerFunc {
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

		if _, carriesSecrets := readOnlyServiceTokenRefusedProxyWebSockets[c.FullPath()]; carriesSecrets &&
			internalmcp.IsReadOnlyServiceTokenRequest(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, ErrRemoteFeedHasSecrets)
			return
		}

		defer c.Abort()

		nodeQuery := query.Node
		node, err := nodeQuery.Where(nodeQuery.ID.Eq(id)).First()
		if err != nil {
			logger.Error(err)
			return
		}
		if refuseDisabledProxyNode(c, node) {
			return
		}

		// Build the target from the parsed URL, which is the path the route
		// was matched and authorized on. Re-parsing the raw request URI would
		// treat a raw '#' as a fragment and forward a shorter path.
		// The proxy takes only the path from the target; the handshake query
		// comes from the cloned request in the callback below. Leaving the
		// query out keeps the caller's credentials out of the log line.
		decodedUri, err := node.GetWebSocketURL(c.Request.URL.EscapedPath())

		if err != nil {
			logger.Error(err)
			return
		}

		logger.Debug("Proxy request", decodedUri)

		// websocketproxy skips certificate verification for wss targets
		// unless it is given a TLS config. Use the one the HTTP node proxy
		// gets from the transport package, so [http] InsecureSkipVerify
		// decides for both and a handshake to an https child is verified by
		// default.
		baseTransport, err := transport.NewTransport()
		if err != nil {
			logger.Error(err)
			return
		}

		wp, err := websocketproxy.NewProxy(decodedUri, func(r *http.Request) error {
			r.Header.Del("X-Node-ID")
			queryValues := r.URL.Query()
			queryValues.Del("x_node_id")
			// The child authenticates the handshake as the node from the
			// signature added below, which covers the query, so the caller's
			// credentials are removed first.
			stripCallerCredentials(r.Header, queryValues, true)
			r.URL.RawQuery = queryValues.Encode()
			setProxyVerifiedTwoFactor(r, c)
			return nodeauth.SignWebSocketHeaders(node, r.URL.String(), r.Header)
		}, websocketproxy.SetTLSConfig(baseTransport.TLSClientConfig.Clone()))

		if err != nil {
			logger.Error(err)
			return
		}

		wp.Proxy(c.Writer, c.Request)
	}
}
