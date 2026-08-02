package analytic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateBytesPerSecond(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		current  uint64
		previous uint64
		elapsed  time.Duration
		want     uint64
	}{
		{
			name:     "normal interval",
			current:  5000,
			previous: 1000,
			elapsed:  2 * time.Second,
			want:     2000,
		},
		{
			name:     "fractional interval",
			current:  2000,
			previous: 1000,
			elapsed:  500 * time.Millisecond,
			want:     2000,
		},
		{
			name:     "counter reset",
			current:  100,
			previous: 1000,
			elapsed:  time.Second,
			want:     0,
		},
		{
			name:     "missing interval",
			current:  2000,
			previous: 1000,
			elapsed:  0,
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, calculateBytesPerSecond(tt.current, tt.previous, tt.elapsed))
		})
	}
}
