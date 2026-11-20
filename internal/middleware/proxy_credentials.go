package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// callerCredentialHeaders carry what a person or a client presents to
// authenticate with this nginx-ui instance: the session token, the
// step-up secure session, the browser cookies that bind short tokens and
// secure sessions, and the MFA and passkey login tickets. They prove who is
// calling this instance and mean nothing anywhere else.
var callerCredentialHeaders = []string{
	"Authorization",
	"Proxy-Authorization",
	"Cookie",
	"X-Secure-Session-ID",
	"X-MFA-Pre-Auth-ID",
	"X-Passkey-Pre-Auth-ID",
	"X-Passkey-Session-ID",
	"X-Current-Password",
}

// callerCredentialQueryKeys are the query parameters that carry the same
// credentials where a header cannot be set, such as a browser WebSocket
// handshake.
var callerCredentialQueryKeys = []string{
	"token",
	"X-Secure-Session-ID",
}

// IsCallerCredentialHeader reports whether a request header is one of the
// credentials a caller authenticates to this instance with.
func IsCallerCredentialHeader(key string) bool {
	for _, name := range callerCredentialHeaders {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

// StripCallerCredentialQuery removes the credential query parameters from
// values.
func StripCallerCredentialQuery(values url.Values) {
	for _, key := range callerCredentialQueryKeys {
		values.Del(key)
	}
}

// stripCallerCredentials removes the caller's credentials from a request this
// instance forwards to a child node. The controller has already authorized
// the caller, and the node transport authenticates the forwarded request as
// the controller's node principal, which is all the child checks. Forwarding
// the caller's own credentials as well would hand a child, or anything
// between the controller and the child, a session that is valid on the
// controller.
//
// It must run before the request is signed, because the node signature
// covers the query.
func stripCallerCredentials(header http.Header, query url.Values) {
	for _, name := range callerCredentialHeaders {
		header.Del(name)
	}
	StripCallerCredentialQuery(query)
}
