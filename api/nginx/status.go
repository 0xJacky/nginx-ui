// GetDetailedStatus API 实现
// 该功能用于解决 Issue #850，提供类似宝塔面板的 Nginx 负载监控功能
// 返回详细的 Nginx 状态信息，包括请求统计、连接数、工作进程等数据
package nginx

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// NginxPerformanceInfo 存储 Nginx 性能相关信息
type NginxPerformanceInfo struct {
	// 基本状态信息
	nginx.StubStatusData

	// 进程相关信息
	nginx.NginxProcessInfo

	// 配置信息
	nginx.NginxConfigInfo
}

// GetDetailStatus 获取 Nginx 详细状态信息
func GetDetailStatus(c *gin.Context) {
	response := nginx.GetPerformanceData()
	c.JSON(http.StatusOK, response)
}

// StreamDetailStatus 使用 SSE 流式推送 Nginx 详细状态信息
func StreamDetailStatus(c *gin.Context) {
	// 设置 SSE 的响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 创建上下文，当客户端断开连接时取消
	ctx := c.Request.Context()

	// 为防止 goroutine 泄漏，创建一个计时器通道
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// 立即发送一次初始数据
	sendPerformanceData(c)

	// 使用 goroutine 定期发送数据
	for {
		select {
		case <-ticker.C:
			// 发送性能数据
			if err := sendPerformanceData(c); err != nil {
				logger.Warn("Error sending SSE data:", err)
				return
			}
		case <-ctx.Done():
			// 客户端断开连接或请求被取消
			logger.Debug("Client closed connection")
			return
		}
	}
}

// sendPerformanceData 发送一次性能数据
func sendPerformanceData(c *gin.Context) error {
	response := nginx.GetPerformanceData()

	// 发送 SSE 事件
	c.SSEvent("message", response)

	// 刷新缓冲区，确保数据立即发送
	c.Writer.Flush()
	return nil
}

// CheckStubStatus 获取 Nginx stub_status 模块状态
func CheckStubStatus(c *gin.Context) {
	stubStatus := nginx.GetStubStatus()

	c.JSON(http.StatusOK, stubStatus)
}

// ToggleStubStatus 启用或禁用 stub_status 模块
func ToggleStubStatus(c *gin.Context) {
	var json struct {
		Enable bool `json:"enable"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	stubStatus := nginx.GetStubStatus()

	// 如果当前状态与期望状态相同，则无需操作
	if stubStatus.Enabled == json.Enable {
		c.JSON(http.StatusOK, stubStatus)
		return
	}

	var err error
	if json.Enable {
		err = nginx.EnableStubStatus()
	} else {
		err = nginx.DisableStubStatus()
	}

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// 重新加载 Nginx 配置
	reloadOutput := nginx.Reload()
	if len(reloadOutput) > 0 && (strings.Contains(strings.ToLower(reloadOutput), "error") ||
		strings.Contains(strings.ToLower(reloadOutput), "failed")) {
		cosy.ErrHandler(c, errors.New("Reload Nginx failed"))
		return
	}

	// 检查操作后的状态
	newStubStatus := nginx.GetStubStatus()

	c.JSON(http.StatusOK, newStubStatus)
}
