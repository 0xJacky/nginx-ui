package analytic

import (
	"errors"
	"math"
	"runtime"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

func TestCalculateCPUUsage(t *testing.T) {
	old := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(old)
	type testCase struct {
		name          string
		before, after cpu.TimesStat
		user, total   float64
	}
	tests := []testCase{
		{"eight core host with two CPU process limit", cpu.TimesStat{}, cpu.TimesStat{User: 1, System: 1, Idle: 6}, 12.5, 25},
		{"longer sample interval", cpu.TimesStat{}, cpu.TimesStat{User: 2, System: 2, Idle: 12}, 12.5, 25},
		{"fully busy", cpu.TimesStat{}, cpu.TimesStat{User: 4, System: 4}, 50, 100},
		{"idle", cpu.TimesStat{}, cpu.TimesStat{Idle: 8}, 0, 0},
		{"no elapsed CPU time", cpu.TimesStat{User: 1}, cpu.TimesStat{User: 1}, 0, 0},
		{"counter reset", cpu.TimesStat{User: 8, Idle: 8}, cpu.TimesStat{User: 1, Idle: 1}, 0, 0},
		{"busy counter regresses", cpu.TimesStat{User: 8, Idle: 8}, cpu.TimesStat{User: 7, Idle: 10}, 0, 0},
		{"iowait is not busy", cpu.TimesStat{}, cpu.TimesStat{User: 1, Idle: 3, Iowait: 4}, 12.5, 12.5},
		{"other busy CPU modes", cpu.TimesStat{}, cpu.TimesStat{User: 1, Nice: 1, Irq: 1, Softirq: 1, Steal: 1, Idle: 3}, 12.5, 62.5},
		{"iowait counter regresses", cpu.TimesStat{Iowait: 2}, cpu.TimesStat{User: 2, Iowait: 1}, 100, 100},
	}
	if runtime.GOOS == "linux" {
		tests = append(tests, testCase{"guest time is already included", cpu.TimesStat{}, cpu.TimesStat{User: 2, Nice: 2, Guest: 1, GuestNice: 1, Idle: 4}, 25, 50})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, total := calculateCPUUsage(tt.before, tt.after)
			if math.Abs(user-tt.user) > 1e-9 || math.Abs(total-tt.total) > 1e-9 || math.IsNaN(total) {
				t.Fatalf("got user=%v total=%v; want user=%v total=%v", user, total, tt.user, tt.total)
			}
		})
	}
}

func TestSampleCPUUsage(t *testing.T) {
	sampleErr := errors.New("CPU statistics unavailable")
	tests := []struct {
		name    string
		samples [][]cpu.TimesStat
		errs    []error
		wantErr bool
	}{
		{"valid", [][]cpu.TimesStat{{{}}, {{User: 2, Idle: 6}}}, []error{nil, nil}, false},
		{"first read error", nil, []error{sampleErr}, true},
		{"second read error", [][]cpu.TimesStat{{{}}}, []error{nil, sampleErr}, true},
		{"empty first sample", [][]cpu.TimesStat{nil}, []error{nil}, true},
		{"empty second sample", [][]cpu.TimesStat{{{}}, nil}, []error{nil, nil}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			read := func(perCPU bool) ([]cpu.TimesStat, error) {
				if perCPU {
					t.Fatal("expected host aggregate sample")
				}
				i := calls
				calls++
				if tt.errs[i] != nil {
					return nil, tt.errs[i]
				}
				return tt.samples[i], nil
			}
			user, total, err := sampleCPUUsage(0, read, func(time.Duration) {})
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr && (user != 25 || total != 25) {
				t.Fatalf("got user=%v total=%v", user, total)
			}
		})
	}
}
