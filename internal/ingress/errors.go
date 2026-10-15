package ingress

import "github.com/uozi-tech/cosy"

var (
	e                    = cosy.NewErrorScope("ingress")
	ErrInvalidRequest    = e.New(40000, "invalid request: {0}")
	ErrInvalidName       = e.New(40001, "invalid ingress name: use 1-40 lowercase letters, digits and hyphens, starting and ending with a letter or digit")
	ErrInvalidOwner      = e.New(40002, "invalid owner: use 1-64 letters, digits, dots, underscores and hyphens")
	ErrInvalidHost       = e.New(40003, "invalid host: {0}")
	ErrInvalidBackend    = e.New(40004, "invalid backend {0}: {1}")
	ErrBackendNotAllowed = e.New(40005, "backend server {0} is outside the allowed backend ranges")
	ErrInvalidRoute      = e.New(40006, "invalid route {0}: {1}")
	ErrInvalidTLS        = e.New(40007, "invalid tls: {0}")
	ErrInvalidPolicy     = e.New(40008, "invalid policy: {0}")
	ErrServerNotFound    = e.New(40009, "server {0} not found in backend {1}")
	ErrNotFound          = e.New(40401, "ingress {0} not found")
	ErrOwnerMismatch     = e.New(40901, "ingress {0} is owned by {1}; pass force to take it over")
	ErrHostConflict      = e.New(40902, "host {0} is already served by ingress {1}")
	ErrSiteConflict      = e.New(40903, "site {0} already exists and is not managed by this ingress")
	ErrDeleteConflict    = e.New(40904, "ingress {0} was updated while being deleted")
	ErrWaitTimeout       = e.New(40801, "timed out waiting for ingress {0} to reach generation {1}")
	ErrApplyFailed       = e.New(50001, "apply failed: {0}")
	ErrCertNotReady      = e.New(50002, "certificate not ready: {0}")
	ErrNotImplemented    = e.New(50101, "not implemented")
)
