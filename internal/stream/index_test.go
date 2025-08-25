package stream

import (
	"testing"
)

func TestIsStreamConfig(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"streams-available/test.conf", true},
		{"streams-enabled/test.conf", true},
		{"/etc/nginx/streams-available/test.conf", true},
		{"/etc/nginx/streams-enabled/test.conf", true},
		{"/var/lib/nginx/streams-available/my-stream.conf", true},
		{"/home/user/nginx/streams-enabled/tcp-proxy.conf", true},
		{"sites-available/test.conf", false},
		{"sites-enabled/test.conf", false},
		{"/etc/nginx/conf.d/test.conf", false},
		{"test.conf", false},
	}

	for _, test := range tests {
		result := isStreamConfig(test.path)
		if result != test.expected {
			t.Errorf("isStreamConfig(%q) = %v, expected %v", test.path, result, test.expected)
		}
	}
}

func TestScanForStream(t *testing.T) {
	// Clear the IndexedStreams map
	IndexedStreams = make(map[string]*Index)

	config := `upstream my-tcp {
    server 127.0.0.1:9000;
}
server {
    listen 1234-1236;
    resolver 8.8.8.8 valid=1s;
    proxy_pass example.com:8080;
}`

	// Test with a valid stream config path
	err := scanForStream("streams-available/test.conf", []byte(config))
	if err != nil {
		t.Errorf("scanForStream failed: %v", err)
	}

	// Check if the stream was indexed
	if len(IndexedStreams) != 1 {
		t.Errorf("Expected 1 indexed stream, got %d", len(IndexedStreams))
	}

	stream := IndexedStreams["test.conf"]
	if stream == nil {
		t.Fatal("Stream not found in index")
	}

	if len(stream.ProxyTargets) != 2 {
		t.Errorf("Expected 2 proxy targets, got %d", len(stream.ProxyTargets))
		for i, target := range stream.ProxyTargets {
			t.Logf("Target %d: %+v", i, target)
		}
	}

	// Test with a non-stream config path
	IndexedStreams = make(map[string]*Index)
	err = scanForStream("sites-available/test.conf", []byte(config))
	if err != nil {
		t.Errorf("scanForStream failed: %v", err)
	}

	// Should not be indexed
	if len(IndexedStreams) != 0 {
		t.Errorf("Expected 0 indexed streams for non-stream config, got %d", len(IndexedStreams))
	}
}
