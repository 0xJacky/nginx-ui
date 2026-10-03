package parser

import "testing"

func TestParser_ParseLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantErr  bool
		validate func(*AccessLogEntry) bool
	}{
		{
			name: "combined log format",
			line: `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.IP == "127.0.0.1" &&
					entry.Method == "GET" &&
					entry.Path == "/index.html" &&
					entry.Status == 200 &&
					entry.BytesSent == 1234
			},
		},
		{
			name: "with request and upstream time",
			line: `192.168.1.1 - - [25/Dec/2023:10:00:00 +0000] "POST /api/data HTTP/1.1" 201 567 "-" "curl/7.68.0" 0.123 0.045`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.IP == "192.168.1.1" &&
					entry.Method == "POST" &&
					entry.Status == 201 &&
					entry.RequestTime == 0.123 &&
					entry.UpstreamTime != nil &&
					*entry.UpstreamTime == 0.045
			},
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: true,
		},
		{
			name:    "malformed line",
			line:    "not a valid log line",
			wantErr: false, // Non-strict mode should handle this gracefully
			validate: func(entry *AccessLogEntry) bool {
				return entry.Raw == "not a valid log line"
			},
		},
		{
			name: "minimal valid line",
			line: `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET / HTTP/1.1" 200 -`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.IP == "127.0.0.1" &&
					entry.Method == "GET" &&
					entry.Path == "/" &&
					entry.Status == 200 &&
					entry.BytesSent == 0
			},
		},
		{
			name: "WebDAV PROPFIND method",
			line: `192.168.1.100 - - [25/Dec/2023:10:00:00 +0000] "PROPFIND /webdav/ HTTP/1.1" 207 1234 "-" "davfs2/1.5.4"`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.IP == "192.168.1.100" &&
					entry.Method == "PROPFIND" &&
					entry.Path == "/webdav/" &&
					entry.Status == 207 &&
					entry.BytesSent == 1234
			},
		},
		{
			name: "WebDAV MKCOL method",
			line: `10.0.0.5 - - [25/Dec/2023:10:00:00 +0000] "MKCOL /webdav/newdir/ HTTP/1.1" 201 0 "-" "WebDAVClient/1.0"`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.IP == "10.0.0.5" &&
					entry.Method == "MKCOL" &&
					entry.Path == "/webdav/newdir/" &&
					entry.Status == 201
			},
		},
		{
			name: "WebDAV LOCK method",
			line: `172.16.0.1 - - [25/Dec/2023:10:00:00 +0000] "LOCK /webdav/file.txt HTTP/1.1" 200 512 "-" "Microsoft-WebDAV"`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.IP == "172.16.0.1" &&
					entry.Method == "LOCK" &&
					entry.Path == "/webdav/file.txt" &&
					entry.Status == 200
			},
		},
	}

	config := DefaultParserConfig()
	config.StrictMode = false // Use non-strict mode to handle malformed lines gracefully

	parser := NewParser(config)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.ParseLine(tt.line)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if entry == nil {
				t.Error("expected entry but got nil")
				return
			}

			if tt.validate != nil && !tt.validate(entry) {
				t.Errorf("entry validation failed: %+v", entry)
			}

			// Verify common fields
			if entry.Raw != tt.line {
				t.Errorf("raw line mismatch: got %q, want %q", entry.Raw, tt.line)
			}
		})
	}
}
