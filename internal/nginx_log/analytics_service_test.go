package nginx_log

import (
	"context"
	"testing"
	"time"
)

func TestAnalyticsService_ValidateLogPath(t *testing.T) {
	service := NewAnalyticsService()

	testCases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "Empty path should be allowed",
			path:    "",
			wantErr: false,
		},
		{
			name:    "Invalid path should be rejected",
			path:    "/var/log/nginx/access.log",
			wantErr: true, // This path is not in whitelist in test environment
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidateLogPath(tc.path)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateLogPath() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestAnalyticsService_ValidateAndNormalizeSearchRequest(t *testing.T) {
	service := NewAnalyticsService()

	testCases := []struct {
		name    string
		req     *QueryRequest
		wantErr bool
		checkFn func(*QueryRequest) bool
	}{
		{
			name: "Default limit should be set",
			req: &QueryRequest{
				Limit: 0,
			},
			wantErr: false,
			checkFn: func(req *QueryRequest) bool {
				return req.Limit == 100
			},
		},
		{
			name: "Limit should be capped at 1000",
			req: &QueryRequest{
				Limit: 2000,
			},
			wantErr: false,
			checkFn: func(req *QueryRequest) bool {
				return req.Limit == 1000
			},
		},
		{
			name: "Negative offset should be normalized",
			req: &QueryRequest{
				Limit:  100,
				Offset: -10,
			},
			wantErr: false,
			checkFn: func(req *QueryRequest) bool {
				return req.Offset == 0
			},
		},
		{
			name: "Invalid time range should return error",
			req: &QueryRequest{
				StartTime: time.Now().Unix(),
				EndTime:   time.Now().Add(-1 * time.Hour).Unix(),
				Limit:     100,
			},
			wantErr: true,
			checkFn: nil,
		},
		{
			name: "Invalid status code should return error",
			req: &QueryRequest{
				Status: []int{999},
				Limit:  100,
			},
			wantErr: true,
			checkFn: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.validateAndNormalizeSearchRequest(tc.req)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateAndNormalizeSearchRequest() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			if !tc.wantErr && tc.checkFn != nil {
				if !tc.checkFn(tc.req) {
					t.Errorf("validateAndNormalizeSearchRequest() validation check failed")
				}
			}
		})
	}
}

func TestAnalyticsService_GetLogEntries(t *testing.T) {
	service := NewAnalyticsService()

	testCases := []struct {
		name    string
		path    string
		limit   int
		tail    bool
		wantErr bool
	}{
		{
			name:    "Empty path should return error",
			path:    "",
			limit:   100,
			tail:    false,
			wantErr: true,
		},
		{
			name:    "Invalid path should return error",
			path:    "/var/log/nginx/access.log",
			limit:   100,
			tail:    false,
			wantErr: true, // This path is not in whitelist in test environment
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entries, err := service.GetLogEntries(tc.path, tc.limit, tc.tail)
			if (err != nil) != tc.wantErr {
				t.Errorf("GetLogEntries() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			if !tc.wantErr {
				// Should return a slice (even if empty)
				if entries == nil {
					t.Errorf("GetLogEntries() returned nil entries")
				}
			}
		})
	}
}

func BenchmarkAnalyticsService_ValidateAndNormalizeSearchRequest(b *testing.B) {
	service := NewAnalyticsService()
	req := &QueryRequest{
		StartTime: time.Now().Add(-1 * time.Hour).Unix(),
		EndTime:   time.Now().Unix(),
		Query:     "test query",
		IP:        "192.168.1.1",
		Method:    "GET",
		Status:    []int{200, 404},
		Path:      "/api/test",
		UserAgent: "Mozilla/5.0",
		Limit:     100,
		Offset:    0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a copy for each iteration to avoid side effects
		reqCopy := *req
		_ = service.validateAndNormalizeSearchRequest(&reqCopy)
	}
}

func TestAnalyticsService_SearchLogs_WithoutIndexer(t *testing.T) {
	service := NewAnalyticsService()
	// Don't set indexer to test error handling

	ctx := context.Background()
	req := &QueryRequest{
		Limit: 100,
	}

	_, err := service.SearchLogs(ctx, req)
	if err == nil {
		t.Error("SearchLogs() should return error when indexer is not available")
	}

	if err != ErrIndexerNotAvailable {
		t.Errorf("SearchLogs() error = %v, want %v", err, ErrIndexerNotAvailable)
	}
}

func TestAnalyticsService_GetIndexStatus_WithoutIndexer(t *testing.T) {
	service := NewAnalyticsService()
	// Don't set indexer to test error handling

	_, err := service.GetIndexStatus()
	if err == nil {
		t.Error("GetIndexStatus() should return error when indexer is not available")
	}

	if err != ErrIndexerNotAvailable {
		t.Errorf("GetIndexStatus() error = %v, want %v", err, ErrIndexerNotAvailable)
	}
}
