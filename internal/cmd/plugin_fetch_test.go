package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitPlatforms(t *testing.T) {
	// No flag means the platform of this node.
	assert.Equal(t, []string{""}, splitPlatforms(""))
	assert.Equal(t, []string{""}, splitPlatforms(" , "))

	assert.Equal(t, []string{"linux-amd64"}, splitPlatforms("linux-amd64"))
	assert.Equal(t, []string{"linux-amd64", "darwin-arm64"}, splitPlatforms(" Linux-AMD64,darwin-arm64,linux-amd64 "))

	// "all" wins over any explicit list.
	assert.Equal(t, []string{"all"}, splitPlatforms("linux-amd64,all"))
}
