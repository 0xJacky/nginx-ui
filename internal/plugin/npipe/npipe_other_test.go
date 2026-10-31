//go:build !windows

package npipe

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDialNeedsWindows(t *testing.T) {
	_, err := Dial(context.Background(), Prefix+"x")
	assert.ErrorIs(t, err, ErrUnsupported)
}
