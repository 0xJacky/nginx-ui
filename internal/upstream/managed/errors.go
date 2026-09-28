package managed

import "github.com/uozi-tech/cosy"

var (
	e                         = cosy.NewErrorScope("upstream")
	ErrInvalidName            = e.New(40001, "invalid upstream name: use letters, digits, underscores and hyphens, starting with a letter or underscore (max 64 characters)")
	ErrInvalidMethod          = e.New(40002, "unsupported load balancing method: {0}")
	ErrHashKeyRequired        = e.New(40003, "the hash method needs a key")
	ErrInvalidHashKey         = e.New(40004, "invalid hash key: {0}")
	ErrNoServers              = e.New(40005, "an upstream needs at least one server")
	ErrInvalidServerAddress   = e.New(40006, "invalid server address: {0}")
	ErrInvalidWeight          = e.New(40007, "server weight must be at least 1: {0}")
	ErrInvalidMaxFails        = e.New(40008, "server max_fails must not be negative: {0}")
	ErrInvalidFailTimeout     = e.New(40009, "invalid server fail_timeout: {0}")
	ErrBackupNotSupported     = e.New(40010, "backup servers cannot be used with the {0} method")
	ErrInvalidKeepalive       = e.New(40011, "keepalive must not be negative")
	ErrInvalidExtraDirectives = e.New(40012, "additional directives must not contain blocks or braces")
	ErrInvalidServerParams    = e.New(40013, "invalid server parameters: {0}")
	ErrInvalidZoneSize        = e.New(40014, "invalid shared memory zone size: {0}; use a number with a k or m suffix, at least 32k")
	ErrZoneInExtraDirectives  = e.New(40015, "the shared memory zone is set by the zone switch; remove the zone directive from the additional directives")
	ErrUpstreamNotFound       = e.New(40401, "upstream not found: {0}")
	ErrUpstreamExists         = e.New(40901, "upstream already exists: {0}")
	ErrUpstreamNameConflict   = e.New(40902, "an upstream with this name is already defined in {0}")
	ErrUpstreamInUse          = e.New(40903, "upstream is still referenced by: {0}")
	ErrUpstreamFileNotManaged = e.New(40904, "{0} is not an upstream file managed by Nginx UI")
	ErrZoneNameConflict       = e.New(40905, "the shared memory zone {0} is already used by another directive; nginx reported: {1}")
	ErrConfDirUnavailable     = e.New(50001, "the nginx conf.d directory is not available: {0}")
)
