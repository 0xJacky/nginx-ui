package helper

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsUnderDirectory(t *testing.T) {
	assert.Equal(t, true, IsUnderDirectory("/etc/nginx/nginx.conf", "/etc/nginx"))
	assert.Equal(t, false, IsUnderDirectory("../../root/nginx.conf", "/etc/nginx"))
	assert.Equal(t, false, IsUnderDirectory("/etc/nginx/../../root/nginx.conf", "/etc/nginx"))
}
