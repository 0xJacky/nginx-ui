package stream

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
)

type Index struct {
	Path         string
	Content      string
	ProxyTargets []upstream.ProxyTarget
}

var (
	IndexedStreams = make(map[string]*Index)
)

func GetIndexedStream(path string) *Index {
	if stream, ok := IndexedStreams[path]; ok {
		return stream
	}
	return &Index{}
}

func init() {
	cache.RegisterCallback("stream.scanForStream", scanForStream)
}

func scanForStream(configPath string, content []byte) error {
	// Only process stream configuration files
	if !isStreamConfig(configPath) {
		return nil
	}

	streamIndex := Index{
		Path:         configPath,
		Content:      string(content),
		ProxyTargets: []upstream.ProxyTarget{},
	}

	// Parse proxy targets from the configuration content with timeout protection
	done := make(chan []upstream.ProxyTarget, 1)
	go func() {
		targets := upstream.ParseProxyTargetsFromRawContent(string(content))
		done <- targets
	}()
	
	select {
	case targets := <-done:
		streamIndex.ProxyTargets = targets
		// Only store if we found proxy targets
		if len(streamIndex.ProxyTargets) > 0 {
			IndexedStreams[filepath.Base(configPath)] = &streamIndex
		}
	case <-time.After(2 * time.Second):
		// Timeout protection - skip this file's stream processing
		// This prevents the callback from blocking indefinitely
		return nil
	}

	return nil
}

// isStreamConfig checks if the config path is a stream configuration
func isStreamConfig(configPath string) bool {
	return strings.Contains(configPath, "streams-available") ||
		strings.Contains(configPath, "streams-enabled")
}
