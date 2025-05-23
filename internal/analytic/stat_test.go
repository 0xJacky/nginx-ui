package analytic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDiskStat(t *testing.T) {
	diskStat, err := GetDiskStat()

	// Test that the function doesn't return an error
	assert.NoError(t, err)

	// Test that partitions are populated
	assert.NotEmpty(t, diskStat.Partitions)

	// Test that overall stats are calculated
	assert.NotEmpty(t, diskStat.Total)
	assert.NotEmpty(t, diskStat.Used)
	assert.GreaterOrEqual(t, diskStat.Percentage, 0.0)
	assert.LessOrEqual(t, diskStat.Percentage, 100.0)

	// Test each partition has required fields
	for _, partition := range diskStat.Partitions {
		assert.NotEmpty(t, partition.Mountpoint)
		assert.NotEmpty(t, partition.Device)
		assert.NotEmpty(t, partition.Fstype)
		assert.NotEmpty(t, partition.Total)
		assert.NotEmpty(t, partition.Used)
		assert.NotEmpty(t, partition.Free)
		assert.GreaterOrEqual(t, partition.Percentage, 0.0)
		assert.LessOrEqual(t, partition.Percentage, 100.0)
	}
}

func TestIsVirtualFilesystem(t *testing.T) {
	// Test virtual filesystems
	assert.True(t, isVirtualFilesystem("proc"))
	assert.True(t, isVirtualFilesystem("sysfs"))
	assert.True(t, isVirtualFilesystem("tmpfs"))
	assert.True(t, isVirtualFilesystem("devpts"))

	// Test real filesystems
	assert.False(t, isVirtualFilesystem("ext4"))
	assert.False(t, isVirtualFilesystem("xfs"))
	assert.False(t, isVirtualFilesystem("ntfs"))
	assert.False(t, isVirtualFilesystem("fat32"))
}
