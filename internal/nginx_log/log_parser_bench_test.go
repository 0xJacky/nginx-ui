package nginx_log

import (
	"testing"
)

func BenchmarkLogParser_ParseLine(b *testing.B) {
	mockUA := NewMockUserAgentParser()
	parser := NewOptimizedLogParser(mockUA)

	logLine := `192.168.1.1 - - [25/Dec/2023:10:00:00 +0000] "GET /test HTTP/1.1" 200 1024 "https://example.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseLine(logLine)
	}
}

func BenchmarkUserAgentParser_Parse(b *testing.B) {
	parser := NewSimpleUserAgentParser()
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parser.Parse(userAgent)
	}
}

func BenchmarkLogParser_ParseLineComplex(b *testing.B) {
	parser := NewOptimizedLogParser(NewSimpleUserAgentParser())

	logLine := `192.168.1.1 - - [25/Dec/2023:10:00:00 +0000] "GET /api/v1/users/123?include=profile&format=json HTTP/1.1" 200 2048 "https://example.com/dashboard" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36" 0.456 0.123`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseLine(logLine)
	}
}

func BenchmarkUserAgentParser_ParseMobile(b *testing.B) {
	parser := NewSimpleUserAgentParser()
	userAgent := "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parser.Parse(userAgent)
	}
}

func BenchmarkUserAgentParser_ParseAndroid(b *testing.B) {
	parser := NewSimpleUserAgentParser()
	userAgent := "Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parser.Parse(userAgent)
	}
}

func BenchmarkDetectLogFormat(b *testing.B) {
	logLines := []string{`192.168.1.1 - - [25/Dec/2023:10:00:00 +0000] "GET /test HTTP/1.1" 200 1024 "https://example.com" "Mozilla/5.0"`}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DetectLogFormat(logLines)
	}
}