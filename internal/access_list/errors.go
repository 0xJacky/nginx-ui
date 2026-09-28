package access_list

import "github.com/uozi-tech/cosy"

var (
	e                      = cosy.NewErrorScope("access_list")
	ErrInvalidName         = e.New(40001, "access list name is required and must be a single line")
	ErrInvalidSlug         = e.New(40002, "identifier must use lowercase letters, digits and hyphens, and start with a letter or digit")
	ErrSlugExists          = e.New(40003, "identifier {0} is already used by another access list")
	ErrInvalidFallback     = e.New(40004, "unknown fallback {0}")
	ErrInvalidRuleType     = e.New(40005, "rule {0} has an unknown type")
	ErrInvalidAddress      = e.New(40006, "rule {0}: {1} is not an IP address, CIDR, all or unix:")
	ErrReferenceNotFound   = e.New(40007, "rule {0} references an access list that does not exist")
	ErrReferenceSelf       = e.New(40008, "rule {0} references the list itself")
	ErrReferenceCycle      = e.New(40009, "circular reference: {0}")
	ErrAccessListNotFound  = e.New(40401, "access list not found")
	ErrAccessListInUse     = e.New(40010, "access list is still used by {0}")
	ErrUnknownList         = e.New(40011, "access list {0} does not exist")
	ErrConfigSyntax        = e.New(40012, "the configuration could not be parsed")
	ErrServerNotFound      = e.New(40013, "server block {0} not found")
	ErrLocationNotFound    = e.New(40014, "location {0} not found in server block {1}")
	ErrManualRules         = e.New(40015, "{0} contains allow or deny rules that were not written by Nginx UI; remove them first")
	ErrInvalidMode         = e.New(40016, "unknown access mode {0}")
	ErrNoteMultiline       = e.New(40017, "rule {0}: the note must be a single line")
	ErrNginxTestFailed     = e.New(50001, "nginx test failed: {0}")
	ErrWriteAccessListFile = e.New(50002, "failed to write access list file: {0}")
)
