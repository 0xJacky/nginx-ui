package nginx

import (
	"os"
	"regexp"
	"runtime"
	"strconv"

	"github.com/pkg/errors"
)

type NginxConfigInfo struct {
	WorkerProcesses           int    `json:"worker_processes"`
	WorkerConnections         int    `json:"worker_connections"`
	ProcessMode               string `json:"process_mode"`
	KeepaliveTimeout          int    `json:"keepalive_timeout"`
	Gzip                      string `json:"gzip"`
	GzipMinLength             int    `json:"gzip_min_length"`
	GzipCompLevel             int    `json:"gzip_comp_level"`
	ClientMaxBodySize         string `json:"client_max_body_size"` // with unit
	ServerNamesHashBucketSize int    `json:"server_names_hash_bucket_size"`
	ClientHeaderBufferSize    string `json:"client_header_buffer_size"` // with unit
	ClientBodyBufferSize      string `json:"client_body_buffer_size"`   // with unit
}

// GetNginxWorkerConfigInfo Get Nginx config info of worker_processes and worker_connections
func GetNginxWorkerConfigInfo() (*NginxConfigInfo, error) {
	result := &NginxConfigInfo{
		WorkerProcesses:           1,
		WorkerConnections:         1024,
		ProcessMode:               "manual",
		KeepaliveTimeout:          65,
		Gzip:                      "off",
		GzipMinLength:             1,
		GzipCompLevel:             1,
		ClientMaxBodySize:         "1m",
		ServerNamesHashBucketSize: 32,
		ClientHeaderBufferSize:    "1k",
		ClientBodyBufferSize:      "8k",
	}

	confPath := GetConfPath("nginx.conf")
	if confPath == "" {
		return nil, errors.New("failed to get nginx.conf path")
	}

	// Read the current configuration
	content, err := os.ReadFile(confPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read nginx.conf")
	}

	outputStr := string(content)

	// Parse worker_processes
	wpRe := regexp.MustCompile(`worker_processes\s+(\d+|auto);`)
	if matches := wpRe.FindStringSubmatch(outputStr); len(matches) > 1 {
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
	if matches := wcRe.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.WorkerConnections, _ = strconv.Atoi(matches[1])
	}

	// Parse keepalive_timeout
	ktRe := regexp.MustCompile(`keepalive_timeout\s+(\d+);`)
	if matches := ktRe.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.KeepaliveTimeout, _ = strconv.Atoi(matches[1])
	}

	// Parse gzip
	gzipRe := regexp.MustCompile(`gzip\s+(on|off);`)
	if matches := gzipRe.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.Gzip = matches[1]
	}

	// Parse gzip_min_length
	gzipMinRe := regexp.MustCompile(`gzip_min_length\s+(\d+);`)
	if matches := gzipMinRe.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.GzipMinLength, _ = strconv.Atoi(matches[1])
	}

	// Parse gzip_comp_level
	gzipCompRe := regexp.MustCompile(`gzip_comp_level\s+(\d+);`)
	if matches := gzipCompRe.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.GzipCompLevel, _ = strconv.Atoi(matches[1])
	}

	// Parse client_max_body_size with any unit (k, m, g)
	cmaxRe := regexp.MustCompile(`client_max_body_size\s+(\d+[kmg]?);`)
	if matches := cmaxRe.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.ClientMaxBodySize = matches[1]
	}

	// Parse server_names_hash_bucket_size
	hashRe := regexp.MustCompile(`server_names_hash_bucket_size\s+(\d+);`)
	if matches := hashRe.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.ServerNamesHashBucketSize, _ = strconv.Atoi(matches[1])
	}

	// Parse client_header_buffer_size with any unit (k, m, g)
	headerRe := regexp.MustCompile(`client_header_buffer_size\s+(\d+[kmg]?);`)
	if matches := headerRe.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.ClientHeaderBufferSize = matches[1]
	}

	// Parse client_body_buffer_size with any unit (k, m, g)
	bodyRe := regexp.MustCompile(`client_body_buffer_size\s+(\d+[kmg]?);`)
	if matches := bodyRe.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.ClientBodyBufferSize = matches[1]
	}

	return result, nil
}
