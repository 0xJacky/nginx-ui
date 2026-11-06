package stream

// MutateOption adjusts a stream mutation. Options are additive, so a caller
// that passes none keeps the default behavior.
type MutateOption func(*mutateOptions)

type mutateOptions struct {
	allowRestrictedDirectives bool
}

// AllowRestrictedDirectives keeps the restricted js_*, perl* and load_module
// statements in the saved configuration instead of rejecting them. Those
// statements load code into the Nginx worker, so only an API handler that
// verified the two-factor step-up may pass it; see
// middleware.RestrictedDirectiveSaveAllowed.
func AllowRestrictedDirectives(allow bool) MutateOption {
	return func(o *mutateOptions) {
		o.allowRestrictedDirectives = allow
	}
}

func resolveMutateOptions(opts []MutateOption) mutateOptions {
	var o mutateOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}
