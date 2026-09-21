package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareVersions(t *testing.T) {
	assert.Equal(t, 0, CompareVersions("1.2.3", "1.2.3"))
	assert.Equal(t, 0, CompareVersions("v1.2.3", "1.2.3+build.5"))
	assert.Equal(t, -1, CompareVersions("1.2.3", "1.3.0"))
	assert.Equal(t, 1, CompareVersions("2.0.0", "1.9.9"))
	// A pre-release ranks below the release it leads to.
	assert.Equal(t, -1, CompareVersions("1.0.0-rc.1", "1.0.0"))
	assert.Equal(t, -1, CompareVersions("1.0.0-alpha.1", "1.0.0-alpha.2"))
	assert.Equal(t, -1, CompareVersions("1.0.0-alpha.2", "1.0.0-alpha.beta"))
}

func TestVersionSatisfies(t *testing.T) {
	cases := []struct {
		version  string
		rangeStr string
		want     bool
	}{
		{"1.2.3", "", true},
		{"1.2.3", "*", true},
		{"1.2.3", "1.2.3", true},
		{"1.2.4", "1.2.3", false},
		{"1.2.3", ">=1.0.0", true},
		{"0.9.0", ">=1.0.0", false},
		{"1.2.3", ">=1.0.0 <2.0.0", true},
		{"2.0.0", ">=1.0.0 <2.0.0", false},
		{"1.9.0", "^1.2.0", true},
		{"2.0.0", "^1.2.0", false},
		{"1.2.0", "^1.2.0", true},
		{"0.2.9", "^0.2.0", true},
		{"0.3.0", "^0.2.0", false},
		{"1.2.9", "~1.2.0", true},
		{"1.3.0", "~1.2.0", false},
		{"3.0.0", "^1.0.0 || ^3.0.0", true},
		{"not-a-version", ">=1.0.0", false},
	}
	for _, testCase := range cases {
		assert.Equalf(t, testCase.want, VersionSatisfies(testCase.version, testCase.rangeStr),
			"%s against %q", testCase.version, testCase.rangeStr)
	}
}
