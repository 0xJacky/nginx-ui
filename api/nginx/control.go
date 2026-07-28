package nginx

import (
	"errors"
	"io"
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy"
)

type restartRequest struct {
	OperationID string `json:"operation_id"`
}

type startRestartFunc func(operationID string) (*nginx.ControlOperation, error)

func buildNamespaceTestConfigResponse(namespaceID uint64, siteCount, streamCount int, result nginx.TestConfigResult) gin.H {
	return gin.H{
		"message":        result.Message,
		"level":          result.Level,
		"namespace_id":   namespaceID,
		"site_count":     siteCount,
		"stream_count":   streamCount,
		"test_scope":     result.TestScope,
		"sandbox_status": result.SandboxStatus,
		"sandbox_reason": result.SandboxReason,
		"error_category": result.ErrorCategory,
	}
}

type namespaceTestConfigDependencies struct {
	findNamespace   func(namespaceID uint64) (*model.Namespace, error)
	findSitePaths   func(namespaceID uint64) ([]string, error)
	findStreamPaths func(namespaceID uint64) ([]string, error)
	testConfig      func(namespace *nginx.NamespaceInfo, sitePaths, streamPaths []string) nginx.TestConfigResult
}

func defaultNamespaceTestConfigDependencies() namespaceTestConfigDependencies {
	return namespaceTestConfigDependencies{
		findNamespace: func(namespaceID uint64) (*model.Namespace, error) {
			ns := query.Namespace
			return ns.Where(ns.ID.Eq(namespaceID)).First()
		},
		findSitePaths: func(namespaceID uint64) ([]string, error) {
			s := query.Site
			sites, err := s.Where(s.NamespaceID.Eq(namespaceID)).Find()
			if err != nil {
				return nil, err
			}

			paths := make([]string, 0, len(sites))
			for _, site := range sites {
				paths = append(paths, site.Path)
			}
			return paths, nil
		},
		findStreamPaths: func(namespaceID uint64) ([]string, error) {
			st := query.Stream
			streams, err := st.Where(st.NamespaceID.Eq(namespaceID)).Find()
			if err != nil {
				return nil, err
			}

			paths := make([]string, 0, len(streams))
			for _, stream := range streams {
				paths = append(paths, stream.Path)
			}
			return paths, nil
		},
		testConfig: nginx.SandboxTestConfigWithPaths,
	}
}

// Reload reloads the nginx
func Reload(c *gin.Context) {
	nginx.Control(nginx.Reload).Resp(c)
}

// TestConfig tests the nginx config
func TestConfig(c *gin.Context) {
	lastResult := nginx.Control(nginx.TestConfig)
	result := nginx.NewTestConfigResult(lastResult.GetStdOut(), lastResult.GetStdErr(), nginx.TestScopeGlobal, "")
	c.JSON(http.StatusOK, result)
}

// TestConfigWithNamespace tests nginx config in isolated sandbox for a specific namespace
func TestConfigWithNamespace(c *gin.Context) {
	var req struct {
		NamespaceID uint64 `json:"namespace_id" form:"namespace_id"`
	}

	if !cosy.BindAndValid(c, &req) {
		return
	}

	testConfigWithNamespace(c, req.NamespaceID, defaultNamespaceTestConfigDependencies())
}

func testConfigWithNamespace(c *gin.Context, namespaceID uint64, dependencies namespaceTestConfigDependencies) {
	var namespaceInfo *nginx.NamespaceInfo
	var sitePaths []string
	var streamPaths []string

	if namespaceID > 0 {
		namespace, err := dependencies.findNamespace(namespaceID)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}

		namespaceInfo = &nginx.NamespaceInfo{
			ID:         namespace.ID,
			Name:       namespace.Name,
			DeployMode: namespace.DeployMode,
		}

		sitePaths, err = dependencies.findSitePaths(namespaceID)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}

		streamPaths, err = dependencies.findStreamPaths(namespaceID)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	result := dependencies.testConfig(namespaceInfo, sitePaths, streamPaths)
	c.JSON(http.StatusOK, buildNamespaceTestConfigResponse(namespaceID, len(sitePaths), len(streamPaths), result))
}

// Restart restarts the nginx
func Restart(c *gin.Context) {
	restart(c, nginx.StartRestart)
}

func restart(c *gin.Context, startRestart startRestartFunc) {
	var request restartRequest
	if err := c.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "invalid restart request",
		})
		return
	}
	if request.OperationID != "" {
		if _, err := uuid.Parse(request.OperationID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "operation_id must be a valid UUID",
			})
			return
		}
	}

	operation, err := startRestart(request.OperationID)
	if err != nil {
		if errors.Is(err, nginx.ErrControlOperationRunning) {
			c.JSON(http.StatusConflict, gin.H{
				"message": "another Nginx control operation is already running",
				"control": nginx.GetControlOperation(),
			})
			return
		}
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
		"control": operation,
	})
}

// Status returns the status of the nginx
func Status(c *gin.Context) {
	lastResult, operation := nginx.GetStatusSnapshot()

	running := nginx.IsRunning()

	c.JSON(http.StatusOK, gin.H{
		"running": running,
		"message": lastResult.GetOutput(),
		"level":   lastResult.GetLevel(),
		"control": operation,
	})
}
