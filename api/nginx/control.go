package nginx

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// Reload reloads the nginx
func Reload(c *gin.Context) {
	nginx.Control(nginx.Reload).Resp(c)
}

// TestConfig tests the nginx config
func TestConfig(c *gin.Context) {
	lastResult := nginx.Control(nginx.TestConfig)
	c.JSON(http.StatusOK, gin.H{
		"message": lastResult.GetOutput(),
		"level":   lastResult.GetLevel(),
	})
}

// TestConfigWithNamespace tests nginx config in isolated sandbox for a specific namespace
func TestConfigWithNamespace(c *gin.Context) {
	var req struct {
		NamespaceID uint64 `json:"namespace_id" form:"namespace_id"`
	}

	if !cosy.BindAndValid(c, &req) {
		return
	}

	// Get namespace and related configs
	var namespaceInfo *nginx.NamespaceInfo
	var sitePaths []string
	var streamPaths []string

	if req.NamespaceID > 0 {
		// Fetch namespace
		ns := query.Namespace
		namespace, err := ns.Where(ns.ID.Eq(req.NamespaceID)).First()
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}

		namespaceInfo = &nginx.NamespaceInfo{
			ID:         namespace.ID,
			Name:       namespace.Name,
			DeployMode: namespace.DeployMode,
		}

		// Fetch sites belonging to this namespace
		s := query.Site
		sites, err := s.Where(s.NamespaceID.Eq(req.NamespaceID)).Find()
		if err == nil {
			for _, site := range sites {
				sitePaths = append(sitePaths, site.Path)
			}
		}

		// Fetch streams belonging to this namespace
		st := query.Stream
		streams, err := st.Where(st.NamespaceID.Eq(req.NamespaceID)).Find()
		if err == nil {
			for _, stream := range streams {
				streamPaths = append(streamPaths, stream.Path)
			}
		}
	}

	// Use sandbox test with namespace-specific paths
	result := nginx.Control(func() (string, error) {
		return nginx.SandboxTestConfigWithPaths(namespaceInfo, sitePaths, streamPaths)
	})

	c.JSON(http.StatusOK, gin.H{
		"message":      result.GetOutput(),
		"level":        result.GetLevel(),
		"namespace_id": req.NamespaceID,
	})
}

// Restart restarts the nginx
func Restart(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
	go nginx.Restart()
}

// Status returns the status of the nginx
func Status(c *gin.Context) {
	lastResult := nginx.GetLastResult()

	running := nginx.IsRunning()

	c.JSON(http.StatusOK, gin.H{
		"running": running,
		"message": lastResult.GetOutput(),
		"level":   lastResult.GetLevel(),
	})
}
