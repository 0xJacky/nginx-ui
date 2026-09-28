package access_list

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uozi-tech/cosy"
)

// assertErrCode compares cosy errors by scope and code, since wrapping an
// error with parameters creates a new value.
func assertErrCode(t *testing.T, err error, target error) {
	t.Helper()
	var got, want *cosy.Error
	if !errors.As(err, &got) || !errors.As(target, &want) {
		assert.Fail(t, "not a cosy error", "got %v, want %v", err, target)
		return
	}
	assert.Equal(t, want.Scope, got.Scope)
	assert.Equal(t, want.Code, got.Code, got.Error())
}
