package settings

import (
	"errors"

	"github.com/uozi-tech/cosy"
)

var (
	e = cosy.NewErrorScope("settings")
	// ErrUnstorableValue is shared with api/settings, which owns codes 40001
	// to 40006 of the same scope. cmd/errdef cannot read a message with a
	// double quote, so the triple quotes are named instead of shown.
	ErrUnstorableValue = e.New(40007, "The value of {0} cannot be stored in app.ini. Remove a trailing backslash, triple quotes next to a line break, or a %(name)s reference from it")
)

// IsUnstorableValue reports whether err is ErrUnstorableValue, with or without
// the setting name it carries.
func IsUnstorableValue(err error) bool {
	var cErr *cosy.Error
	if !errors.As(err, &cErr) {
		return false
	}
	target := ErrUnstorableValue.(*cosy.Error)
	return cErr.Scope == target.Scope && cErr.Code == target.Code
}
