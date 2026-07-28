package nodeauth

import (
	"context"
	"net/http"
)

const GinPrincipalKey = "VerifiedNodePrincipal"

type Principal struct {
	CredentialID         string
	ControllerInstanceID string
	AuthMethod           string
}

type principalContextKey struct{}

func WithPrincipal(request *http.Request, principal *Principal) *http.Request {
	if principal == nil {
		return request
	}
	ctx := context.WithValue(request.Context(), principalContextKey{}, principal)
	return request.WithContext(ctx)
}

func PrincipalFromRequest(request *http.Request) (*Principal, bool) {
	if request == nil {
		return nil, false
	}
	principal, ok := request.Context().Value(principalContextKey{}).(*Principal)
	return principal, ok && principal != nil
}
