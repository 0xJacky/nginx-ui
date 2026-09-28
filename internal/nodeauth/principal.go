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
	// ReplicatedFrom is the instance id of the node that forwarded this request
	// as part of its own synchronization, taken from the Replicated-From field.
	ReplicatedFrom string
	// Replicated is set when the request carried a valid Replicated-From field.
	Replicated bool
}

type principalContextKey struct{}

// WithPrincipal attaches the verified node identity to the request. The
// replication marker is only honored here, behind node authentication, so a
// browser cannot switch off the synchronization of its own changes.
func WithPrincipal(request *http.Request, principal *Principal) *http.Request {
	if principal == nil {
		return request
	}
	attached := *principal
	attached.ReplicatedFrom, attached.Replicated = parseReplicatedFrom(request.Header)
	ctx := context.WithValue(request.Context(), principalContextKey{}, &attached)
	return request.WithContext(ctx)
}

func PrincipalFromRequest(request *http.Request) (*Principal, bool) {
	if request == nil {
		return nil, false
	}
	return PrincipalFromContext(request.Context())
}

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	if ctx == nil {
		return nil, false
	}
	principal, ok := ctx.Value(principalContextKey{}).(*Principal)
	return principal, ok && principal != nil
}
