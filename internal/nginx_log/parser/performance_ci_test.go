package parser

import (
	"os"
	"testing"
)

func skipHostPerformanceTestInCI(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Skip("skipping host-dependent production-scale performance test in CI")
	}
}
