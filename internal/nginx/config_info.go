package nginx

import (
	"os/exec"
	"regexp"
	"runtime"
	"strconv"

	"github.com/pkg/errors"
)

type NginxConfigInfo struct {
	WorkerProcesses   int    `json:"worker_processes"`
	WorkerConnections int    `json:"worker_connections"`
	ProcessMode       string `json:"process_mode"`
}

// GetNginxWorkerConfigInfo Get Nginx config info of worker_processes and worker_connections
func GetNginxWorkerConfigInfo() (*NginxConfigInfo, error) {
	result := &NginxConfigInfo{
		WorkerProcesses:   1,
		WorkerConnections: 1024,
		ProcessMode:       "manual",
	}

	// Get worker_processes config
	cmd := exec.Command("nginx", "-T")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return result, errors.Wrap(err, "failed to get nginx config")
	}

	// Parse worker_processes
	wpRe := regexp.MustCompile(`worker_processes\s+(\d+|auto);`)
	if matches := wpRe.FindStringSubmatch(string(output)); len(matches) > 1 {
		if matches[1] == "auto" {
			result.WorkerProcesses = runtime.NumCPU()
			result.ProcessMode = "auto"
		} else {
			result.WorkerProcesses, _ = strconv.Atoi(matches[1])
			result.ProcessMode = "manual"
		}
	}

	// Parse worker_connections
	wcRe := regexp.MustCompile(`worker_connections\s+(\d+);`)
	if matches := wcRe.FindStringSubmatch(string(output)); len(matches) > 1 {
		result.WorkerConnections, _ = strconv.Atoi(matches[1])
	}

	return result, nil
}
