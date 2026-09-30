package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEffectiveMemoryMB(t *testing.T) {
	assert.Equal(t, 8192, effectiveMemoryMB(8192, 0))
	assert.Equal(t, 512, effectiveMemoryMB(8192, 512<<20))
	assert.Equal(t, 8192, effectiveMemoryMB(8192, 64<<30))
	assert.Equal(t, 512, effectiveMemoryMB(0, 512<<20))
	assert.Equal(t, 0, effectiveMemoryMB(0, 0))
	assert.Equal(t, 0, effectiveMemoryMB(-1, -1))
}

func TestParseMemoryMax(t *testing.T) {
	assert.Equal(t, int64(0), parseMemoryMax("max\n"))
	assert.Equal(t, int64(0), parseMemoryMax(""))
	assert.Equal(t, int64(0), parseMemoryMax("junk"))
	assert.Equal(t, int64(0), parseMemoryMax("-5"))
	assert.Equal(t, int64(268435456), parseMemoryMax("268435456\n"))
}

func TestCgroupV2Path(t *testing.T) {
	path, ok := cgroupV2Path("12:cpu:/x\n0::/system.slice/nginx-ui.service\n")
	assert.True(t, ok)
	assert.Equal(t, "/system.slice/nginx-ui.service", path)
	_, ok = cgroupV2Path("12:cpu:/x\n")
	assert.False(t, ok)
}

func TestSystemMemoryMBIsNotNegative(t *testing.T) {
	assert.GreaterOrEqual(t, SystemMemoryMB(), 0)
}
