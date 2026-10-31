package middleware

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// LargeUploadLimit is the request body cap for routes that receive plugin
// packages. It matches the maximum plugin package size.
const LargeUploadLimit int64 = 256 << 20

// largeUploadRoutes lists the POST paths allowed to carry a large body.
var largeUploadRoutes = map[string]struct{}{
	"/api/plugins":         {},
	"/api/plugins/inspect": {},
}

// uploadBodyKey carries the original body of an upload request past the cosy
// body cap.
type uploadBodyKey struct{}

// LargeUploads wraps the server handler so the upload routes keep their
// original body. cosy caps every body in its first middleware, before any
// route specific handler runs and with no way to exempt a route, so the body
// is put aside here and ScopedBodyLimit takes it back under the upload cap.
// The configured cap itself is never touched, it still applies to every other
// route and a settings save writes it back unchanged.
func LargeUploads(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isLargeUpload(r) {
			r = r.WithContext(context.WithValue(r.Context(), uploadBodyKey{}, r.Body))
		}
		next.ServeHTTP(w, r)
	})
}

// ScopedBodyLimit lets the upload routes accept plugin packages, see
// LargeUploads. Every other request passes through untouched.
func ScopedBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if body, ok := c.Request.Context().Value(uploadBodyKey{}).(io.ReadCloser); ok && body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, body, LargeUploadLimit)
		}
		c.Next()
	}
}

func isLargeUpload(r *http.Request) bool {
	if r.Method != http.MethodPost || r.Body == nil {
		return false
	}
	_, ok := largeUploadRoutes[strings.TrimSuffix(r.URL.Path, "/")]
	return ok
}
