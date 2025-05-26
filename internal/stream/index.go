package stream

import (
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
)

type StreamIndex struct {
	Path         string
	Content      string
	ProxyTargets []upstream.ProxyTarget
}

var (
	IndexedStreams = make(map[string]*StreamIndex)
)

func GetIndexedStream(path string) *StreamIndex {
	if stream, ok := IndexedStreams[path]; ok {
		return stream
	}
	return &StreamIndex{}
}

func init() {
	cache.RegisterCallback(scanForStream)
}

func scanForStream(configPath string, content []byte) error {
	// Only process stream configuration files
	if !isStreamConfig(configPath) {
		return nil
	}

	streamIndex := StreamIndex{
		Path:         configPath,
		Content:      string(content),
		ProxyTargets: []upstream.ProxyTarget{},
	}

	// Parse proxy targets from the configuration content
	streamIndex.ProxyTargets = upstream.ParseProxyTargetsFromRawContent(string(content))
	// Only store if we found proxy targets
	if len(streamIndex.ProxyTargets) > 0 {
		IndexedStreams[filepath.Base(configPath)] = &streamIndex
	}

	return nil
}

// isStreamConfig checks if the config path is a stream configuration
func isStreamConfig(configPath string) bool {
	return strings.Contains(configPath, "streams-available") ||
		strings.Contains(configPath, "streams-enabled")
}
