package performance

import (
	"os"
	"regexp"
	"runtime"
	"strconv"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/pkg/errors"
)

type NginxConfigInfo struct {
	WorkerProcesses           int              `json:"worker_processes"`
	WorkerConnections         int              `json:"worker_connections"`
	ProcessMode               string           `json:"process_mode"`
	KeepaliveTimeout          string           `json:"keepalive_timeout"`
	Gzip                      string           `json:"gzip"`
	GzipMinLength             int              `json:"gzip_min_length"`
	GzipCompLevel             int              `json:"gzip_comp_level"`
	ClientMaxBodySize         string           `json:"client_max_body_size"` // with unit
	ServerNamesHashBucketSize string           `json:"server_names_hash_bucket_size"`
	ClientHeaderBufferSize    string           `json:"client_header_buffer_size"` // with unit
	ClientBodyBufferSize      string           `json:"client_body_buffer_size"`   // with unit
	ProxyCache                ProxyCacheConfig `json:"proxy_cache"`
}

// GetNginxWorkerConfigInfo Get Nginx config info of worker_processes and worker_connections
func GetNginxWorkerConfigInfo() (*NginxConfigInfo, error) {
	result := &NginxConfigInfo{
		WorkerProcesses:           1,
		WorkerConnections:         1024,
		ProcessMode:               "manual",
		KeepaliveTimeout:          "65s",
		Gzip:                      "off",
		GzipMinLength:             1,
		GzipCompLevel:             1,
		ClientMaxBodySize:         "1m",
		ServerNamesHashBucketSize: "32",
		ClientHeaderBufferSize:    "1k",
		ClientBodyBufferSize:      "8k",
		ProxyCache: ProxyCacheConfig{
			Enabled:     false,
			Path:        "/var/cache/nginx/proxy_cache",
			Levels:      "1:2",
			UseTempPath: "off",
			KeysZone:    "proxy_cache:10m",
			Inactive:    "60m",
			MaxSize:     "1g",
			// Purger:      "off",
		},
	}

	confPath := nginx.GetConfEntryPath()
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
	ktRe := regexp.MustCompile(`keepalive_timeout\s+(\d+[smhdwMy]?);`)
	if matches := ktRe.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.KeepaliveTimeout = matches[1]
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
	hashRe := regexp.MustCompile(`server_names_hash_bucket_size\s+(\d+[kmg]?);`)
	if matches := hashRe.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.ServerNamesHashBucketSize = matches[1]
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

	// Parse proxy_cache_path settings
	proxyCachePathRe := regexp.MustCompile(`proxy_cache_path\s+([^;]+);`)
	if matches := proxyCachePathRe.FindStringSubmatch(outputStr); len(matches) > 1 {
		result.ProxyCache.Enabled = true
		proxyCacheParams := matches[1]

		// Extract path (first parameter)
		pathRe := regexp.MustCompile(`^\s*([^\s]+)`)
		if pathMatches := pathRe.FindStringSubmatch(proxyCacheParams); len(pathMatches) > 1 {
			result.ProxyCache.Path = pathMatches[1]
		}

		// Extract levels parameter
		levelsRe := regexp.MustCompile(`levels=([^\s]+)`)
		if levelsMatches := levelsRe.FindStringSubmatch(proxyCacheParams); len(levelsMatches) > 1 {
			result.ProxyCache.Levels = levelsMatches[1]
		}

		// Extract use_temp_path parameter
		useTempPathRe := regexp.MustCompile(`use_temp_path=(on|off)`)
		if useTempPathMatches := useTempPathRe.FindStringSubmatch(proxyCacheParams); len(useTempPathMatches) > 1 {
			result.ProxyCache.UseTempPath = useTempPathMatches[1]
		}

		// Extract keys_zone parameter
		keysZoneRe := regexp.MustCompile(`keys_zone=([^\s]+)`)
		if keysZoneMatches := keysZoneRe.FindStringSubmatch(proxyCacheParams); len(keysZoneMatches) > 1 {
			result.ProxyCache.KeysZone = keysZoneMatches[1]
		}

		// Extract inactive parameter
		inactiveRe := regexp.MustCompile(`inactive=([^\s]+)`)
		if inactiveMatches := inactiveRe.FindStringSubmatch(proxyCacheParams); len(inactiveMatches) > 1 {
			result.ProxyCache.Inactive = inactiveMatches[1]
		}

		// Extract max_size parameter
		maxSizeRe := regexp.MustCompile(`max_size=([^\s]+)`)
		if maxSizeMatches := maxSizeRe.FindStringSubmatch(proxyCacheParams); len(maxSizeMatches) > 1 {
			result.ProxyCache.MaxSize = maxSizeMatches[1]
		}

		// Extract min_free parameter
		minFreeRe := regexp.MustCompile(`min_free=([^\s]+)`)
		if minFreeMatches := minFreeRe.FindStringSubmatch(proxyCacheParams); len(minFreeMatches) > 1 {
			result.ProxyCache.MinFree = minFreeMatches[1]
		}

		// Extract manager_files parameter
		managerFilesRe := regexp.MustCompile(`manager_files=([^\s]+)`)
		if managerFilesMatches := managerFilesRe.FindStringSubmatch(proxyCacheParams); len(managerFilesMatches) > 1 {
			result.ProxyCache.ManagerFiles = managerFilesMatches[1]
		}

		// Extract manager_sleep parameter
		managerSleepRe := regexp.MustCompile(`manager_sleep=([^\s]+)`)
		if managerSleepMatches := managerSleepRe.FindStringSubmatch(proxyCacheParams); len(managerSleepMatches) > 1 {
			result.ProxyCache.ManagerSleep = managerSleepMatches[1]
		}

		// Extract manager_threshold parameter
		managerThresholdRe := regexp.MustCompile(`manager_threshold=([^\s]+)`)
		if managerThresholdMatches := managerThresholdRe.FindStringSubmatch(proxyCacheParams); len(managerThresholdMatches) > 1 {
			result.ProxyCache.ManagerThreshold = managerThresholdMatches[1]
		}

		// Extract loader_files parameter
		loaderFilesRe := regexp.MustCompile(`loader_files=([^\s]+)`)
		if loaderFilesMatches := loaderFilesRe.FindStringSubmatch(proxyCacheParams); len(loaderFilesMatches) > 1 {
			result.ProxyCache.LoaderFiles = loaderFilesMatches[1]
		}

		// Extract loader_sleep parameter
		loaderSleepRe := regexp.MustCompile(`loader_sleep=([^\s]+)`)
		if loaderSleepMatches := loaderSleepRe.FindStringSubmatch(proxyCacheParams); len(loaderSleepMatches) > 1 {
			result.ProxyCache.LoaderSleep = loaderSleepMatches[1]
		}

		// Extract loader_threshold parameter
		loaderThresholdRe := regexp.MustCompile(`loader_threshold=([^\s]+)`)
		if loaderThresholdMatches := loaderThresholdRe.FindStringSubmatch(proxyCacheParams); len(loaderThresholdMatches) > 1 {
			result.ProxyCache.LoaderThreshold = loaderThresholdMatches[1]
		}

		// Extract purger parameter
		// purgerRe := regexp.MustCompile(`purger=(on|off)`)
		// if purgerMatches := purgerRe.FindStringSubmatch(proxyCacheParams); len(purgerMatches) > 1 {
		// 	result.ProxyCache.Purger = purgerMatches[1]
		// }

		// // Extract purger_files parameter
		// purgerFilesRe := regexp.MustCompile(`purger_files=([^\s]+)`)
		// if purgerFilesMatches := purgerFilesRe.FindStringSubmatch(proxyCacheParams); len(purgerFilesMatches) > 1 {
		// 	result.ProxyCache.PurgerFiles = purgerFilesMatches[1]
		// }

		// // Extract purger_sleep parameter
		// purgerSleepRe := regexp.MustCompile(`purger_sleep=([^\s]+)`)
		// if purgerSleepMatches := purgerSleepRe.FindStringSubmatch(proxyCacheParams); len(purgerSleepMatches) > 1 {
		// 	result.ProxyCache.PurgerSleep = purgerSleepMatches[1]
		// }

		// // Extract purger_threshold parameter
		// purgerThresholdRe := regexp.MustCompile(`purger_threshold=([^\s]+)`)
		// if purgerThresholdMatches := purgerThresholdRe.FindStringSubmatch(proxyCacheParams); len(purgerThresholdMatches) > 1 {
		// 	result.ProxyCache.PurgerThreshold = purgerThresholdMatches[1]
		// }
	} else {
		// No proxy_cache_path directive found, so disable it
		result.ProxyCache.Enabled = false
	}

	return result, nil
}
