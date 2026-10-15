package ingress

import "github.com/0xJacky/Nginx-UI/model"

// Validate checks a spec without rendering it: names, hosts (IDNA), backend
// addresses against settings.IngressSettings.AllowedBackends, routes, TLS and
// policy values.
func Validate(spec *model.IngressSpec) error {
	return ErrNotImplemented
}

// Render turns an ingress into its site and upstream files. The output is
// deterministic: the same ingress and options always give the same bytes.
func Render(ing *model.Ingress, opts RenderOptions) (*Rendered, error) {
	return nil, ErrNotImplemented
}
