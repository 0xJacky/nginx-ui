package snippet

import "github.com/uozi-tech/cosy"

var (
	e = cosy.NewErrorScope("snippet")

	ErrInvalidFile     = e.New(57001, "the file name must end in .conf and use only letters, digits, dots, dashes and underscores")
	ErrNotFound        = e.New(57002, "snippet not found")
	ErrTooLarge        = e.New(57003, "the snippet exceeds 256 KiB")
	ErrInvalidHeader   = e.New(57004, "the header of the snippet does not parse: {0}")
	ErrInUse           = e.New(57005, "the snippet is still included by {0}")
	ErrAlreadyExists   = e.New(57006, "a snippet with this file name already exists")
	ErrInvalidVariable = e.New(57007, "the variable {0} is invalid: {1}")
	ErrInvalidTemplate = e.New(57008, "the content does not parse as a template: {0}")
)
