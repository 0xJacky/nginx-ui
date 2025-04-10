// GetDetailedStatus API 实现
// 该功能用于解决 Issue #850，提供类似宝塔面板的 Nginx 负载监控功能
// 返回详细的 Nginx 状态信息，包括请求统计、连接数、工作进程等数据
package nginx

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/uozi-tech/cosy/logger"
)

// NginxPerformanceInfo 存储 Nginx 性能相关信息
type NginxPerformanceInfo struct {
	// 基本状态信息
	Active   int `json:"active"`   // 活动连接数
	Accepts  int `json:"accepts"`  // 总握手次数
	Handled  int `json:"handled"`  // 总连接次数
	Requests int `json:"requests"` // 总请求数
	Reading  int `json:"reading"`  // 读取客户端请求数
	Writing  int `json:"writing"`  // 响应数
	Waiting  int `json:"waiting"`  // 驻留进程（等待请求）

	// 进程相关信息
	Workers     int     `json:"workers"`      // 工作进程数
	Master      int     `json:"master"`       // 主进程数
	Cache       int     `json:"cache"`        // 缓存管理进程数
	Other       int     `json:"other"`        // 其他Nginx相关进程数
	CPUUsage    float64 `json:"cpu_usage"`    // CPU 使用率
	MemoryUsage float64 `json:"memory_usage"` // 内存使用率（MB）

	// 配置信息
	WorkerProcesses   int `json:"worker_processes"`   // worker_processes 配置
	WorkerConnections int `json:"worker_connections"` // worker_connections 配置
}

// GetDetailedStatus 获取 Nginx 详细状态信息
func GetDetailedStatus(c *gin.Context) {
	// 检查 Nginx 是否运行
	pidPath := nginx.GetPIDPath()
	running := true
	if fileInfo, err := os.Stat(pidPath); err != nil || fileInfo.Size() == 0 {
		running = false
		c.JSON(http.StatusOK, gin.H{
			"running": false,
			"message": "Nginx is not running",
		})
		return
	}

	// 获取 stub_status 模块数据
	stubStatusInfo, err := getStubStatusInfo()
	if err != nil {
		logger.Warn("Failed to get stub_status info:", err)
	}

	// 获取进程信息
	processInfo, err := getNginxProcessInfo()
	if err != nil {
		logger.Warn("Failed to get process info:", err)
	}

	// 获取配置信息
	configInfo, err := getNginxConfigInfo()
	if err != nil {
		logger.Warn("Failed to get config info:", err)
	}

	// 组合所有信息
	info := NginxPerformanceInfo{
		Active:            stubStatusInfo["active"],
		Accepts:           stubStatusInfo["accepts"],
		Handled:           stubStatusInfo["handled"],
		Requests:          stubStatusInfo["requests"],
		Reading:           stubStatusInfo["reading"],
		Writing:           stubStatusInfo["writing"],
		Waiting:           stubStatusInfo["waiting"],
		Workers:           processInfo["workers"].(int),
		Master:            processInfo["master"].(int),
		Cache:             processInfo["cache"].(int),
		Other:             processInfo["other"].(int),
		CPUUsage:          processInfo["cpu_usage"].(float64),
		MemoryUsage:       processInfo["memory_usage"].(float64),
		WorkerProcesses:   configInfo["worker_processes"],
		WorkerConnections: configInfo["worker_connections"],
	}

	c.JSON(http.StatusOK, gin.H{
		"running": running,
		"info":    info,
	})
}

// StreamDetailedStatus 使用 SSE 流式推送 Nginx 详细状态信息
func StreamDetailedStatus(c *gin.Context) {
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
	// 检查 Nginx 是否运行
	pidPath := nginx.GetPIDPath()
	running := true
	if fileInfo, err := os.Stat(pidPath); err != nil || fileInfo.Size() == 0 {
		running = false
		// 发送 Nginx 未运行的状态
		c.SSEvent("message", gin.H{
			"running": false,
			"message": "Nginx is not running",
		})
		// 刷新缓冲区，确保数据立即发送
		c.Writer.Flush()
		return nil
	}

	// 获取性能数据
	stubStatusInfo, err := getStubStatusInfo()
	if err != nil {
		logger.Warn("Failed to get stub_status info:", err)
	}

	processInfo, err := getNginxProcessInfo()
	if err != nil {
		logger.Warn("Failed to get process info:", err)
	}

	configInfo, err := getNginxConfigInfo()
	if err != nil {
		logger.Warn("Failed to get config info:", err)
	}

	// 组合所有信息
	info := NginxPerformanceInfo{
		Active:            stubStatusInfo["active"],
		Accepts:           stubStatusInfo["accepts"],
		Handled:           stubStatusInfo["handled"],
		Requests:          stubStatusInfo["requests"],
		Reading:           stubStatusInfo["reading"],
		Writing:           stubStatusInfo["writing"],
		Waiting:           stubStatusInfo["waiting"],
		Workers:           processInfo["workers"].(int),
		Master:            processInfo["master"].(int),
		Cache:             processInfo["cache"].(int),
		Other:             processInfo["other"].(int),
		CPUUsage:          processInfo["cpu_usage"].(float64),
		MemoryUsage:       processInfo["memory_usage"].(float64),
		WorkerProcesses:   configInfo["worker_processes"],
		WorkerConnections: configInfo["worker_connections"],
	}

	// 发送 SSE 事件
	c.SSEvent("message", gin.H{
		"running": running,
		"info":    info,
	})

	// 刷新缓冲区，确保数据立即发送
	c.Writer.Flush()
	return nil
}

// 获取 stub_status 模块数据
func getStubStatusInfo() (map[string]int, error) {
	result := map[string]int{
		"active": 0, "accepts": 0, "handled": 0, "requests": 0,
		"reading": 0, "writing": 0, "waiting": 0,
	}

	// 默认尝试访问 stub_status 页面
	statusURL := "http://localhost/stub_status"

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 发送请求获取 stub_status 数据
	resp, err := client.Get(statusURL)
	if err != nil {
		return result, fmt.Errorf("failed to get stub status: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("failed to read response body: %v", err)
	}

	// 解析响应内容
	statusContent := string(body)

	// 匹配活动连接数
	activeRe := regexp.MustCompile(`Active connections:\s+(\d+)`)
	if matches := activeRe.FindStringSubmatch(statusContent); len(matches) > 1 {
		result["active"], _ = strconv.Atoi(matches[1])
	}

	// 匹配请求统计信息
	serverRe := regexp.MustCompile(`(\d+)\s+(\d+)\s+(\d+)`)
	if matches := serverRe.FindStringSubmatch(statusContent); len(matches) > 3 {
		result["accepts"], _ = strconv.Atoi(matches[1])
		result["handled"], _ = strconv.Atoi(matches[2])
		result["requests"], _ = strconv.Atoi(matches[3])
	}

	// 匹配读写等待数
	connRe := regexp.MustCompile(`Reading:\s+(\d+)\s+Writing:\s+(\d+)\s+Waiting:\s+(\d+)`)
	if matches := connRe.FindStringSubmatch(statusContent); len(matches) > 3 {
		result["reading"], _ = strconv.Atoi(matches[1])
		result["writing"], _ = strconv.Atoi(matches[2])
		result["waiting"], _ = strconv.Atoi(matches[3])
	}

	return result, nil
}

// 获取 Nginx 进程信息
func getNginxProcessInfo() (map[string]interface{}, error) {
	result := map[string]interface{}{
		"workers":      0,
		"master":       0,
		"cache":        0,
		"other":        0,
		"cpu_usage":    0.0,
		"memory_usage": 0.0,
	}

	// 查找所有 Nginx 进程
	processes, err := process.Processes()
	if err != nil {
		return result, fmt.Errorf("failed to get processes: %v", err)
	}

	totalMemory := 0.0
	workerCount := 0
	masterCount := 0
	cacheCount := 0
	otherCount := 0
	nginxProcesses := []*process.Process{}

	// 获取系统CPU核心数
	numCPU := runtime.NumCPU()

	// 获取Nginx主进程的PID
	var masterPID int32 = -1
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}

		cmdline, err := p.Cmdline()
		if err != nil {
			continue
		}

		// 检查是否是Nginx主进程
		if strings.Contains(strings.ToLower(name), "nginx") &&
			(strings.Contains(cmdline, "master process") ||
				!strings.Contains(cmdline, "worker process")) &&
			p.Pid > 0 {
			masterPID = p.Pid
			masterCount++
			nginxProcesses = append(nginxProcesses, p)

			// 获取内存使用情况 - 使用RSS代替
			// 注意：理想情况下我们应该使用USS（仅包含进程独占内存），但gopsutil不直接支持
			mem, err := p.MemoryInfo()
			if err == nil && mem != nil {
				// 转换为 MB
				memoryUsage := float64(mem.RSS) / 1024 / 1024
				totalMemory += memoryUsage
			}

			break
		}
	}

	// 遍历所有进程，区分工作进程和其他Nginx进程
	for _, p := range processes {
		if p.Pid == masterPID {
			continue // 已经计算过主进程
		}

		name, err := p.Name()
		if err != nil {
			continue
		}

		// 只处理Nginx相关进程
		if !strings.Contains(strings.ToLower(name), "nginx") {
			continue
		}

		// 添加到Nginx进程列表
		nginxProcesses = append(nginxProcesses, p)

		// 获取父进程PID
		ppid, err := p.Ppid()
		if err != nil {
			continue
		}

		cmdline, err := p.Cmdline()
		if err != nil {
			continue
		}

		// 获取内存使用情况 - 使用RSS代替
		// 注意：理想情况下我们应该使用USS（仅包含进程独占内存），但gopsutil不直接支持
		mem, err := p.MemoryInfo()
		if err == nil && mem != nil {
			// 转换为 MB
			memoryUsage := float64(mem.RSS) / 1024 / 1024
			totalMemory += memoryUsage
		}

		// 区分工作进程、缓存进程和其他进程
		if ppid == masterPID || strings.Contains(cmdline, "worker process") {
			workerCount++
		} else if strings.Contains(cmdline, "cache") {
			cacheCount++
		} else {
			otherCount++
		}
	}

	// 重新计算CPU使用率，更接近top命令的计算方式
	// 首先进行初始CPU时间测量
	times1 := make(map[int32]float64)
	for _, p := range nginxProcesses {
		times, err := p.Times()
		if err == nil {
			// CPU时间 = 用户时间 + 系统时间
			times1[p.Pid] = times.User + times.System
		}
	}

	// 等待一小段时间
	time.Sleep(100 * time.Millisecond)

	// 再次测量CPU时间
	totalCPUPercent := 0.0
	for _, p := range nginxProcesses {
		times, err := p.Times()
		if err != nil {
			continue
		}

		// 计算CPU时间差
		currentTotal := times.User + times.System
		if previousTotal, ok := times1[p.Pid]; ok {
			// 计算这段时间内的CPU使用百分比（考虑多核）
			cpuDelta := currentTotal - previousTotal
			// 计算每秒CPU使用率（考虑采样时间）
			cpuPercent := (cpuDelta / 0.1) * 100.0 / float64(numCPU)
			totalCPUPercent += cpuPercent
		}
	}

	// 四舍五入到整数，更符合top显示方式
	totalCPUPercent = math.Round(totalCPUPercent)

	// 四舍五入内存使用量到两位小数
	totalMemory = math.Round(totalMemory*100) / 100

	result["workers"] = workerCount
	result["master"] = masterCount
	result["cache"] = cacheCount
	result["other"] = otherCount
	result["cpu_usage"] = totalCPUPercent
	result["memory_usage"] = totalMemory

	return result, nil
}

// 获取 Nginx 配置信息
func getNginxConfigInfo() (map[string]int, error) {
	result := map[string]int{
		"worker_processes":   1,
		"worker_connections": 1024,
	}

	// 获取 worker_processes 配置
	cmd := exec.Command("nginx", "-T")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return result, fmt.Errorf("failed to get nginx config: %v", err)
	}

	// 解析 worker_processes
	wpRe := regexp.MustCompile(`worker_processes\s+(\d+|auto);`)
	if matches := wpRe.FindStringSubmatch(string(output)); len(matches) > 1 {
		if matches[1] == "auto" {
			result["worker_processes"] = runtime.NumCPU()
		} else {
			result["worker_processes"], _ = strconv.Atoi(matches[1])
		}
	}

	// 解析 worker_connections
	wcRe := regexp.MustCompile(`worker_connections\s+(\d+);`)
	if matches := wcRe.FindStringSubmatch(string(output)); len(matches) > 1 {
		result["worker_connections"], _ = strconv.Atoi(matches[1])
	}

	return result, nil
}
