package protocol

// HTTPHandleParams is the payload of http.handle (RPC fallback for the http capability).
type HTTPHandleParams struct {
	Method     string              `json:"method"`
	Path       string              `json:"path"`
	Query      string              `json:"query"`
	Headers    map[string][]string `json:"headers"`
	BodyBase64 string              `json:"body_base64,omitempty"`
	User       HTTPUser            `json:"user"`
}

// HTTPUser identifies the nginx-ui user behind a proxied request.
type HTTPUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// HTTPHandleResult is the reply to http.handle.
type HTTPHandleResult struct {
	Status     int                 `json:"status"`
	Headers    map[string][]string `json:"headers,omitempty"`
	BodyBase64 string              `json:"body_base64,omitempty"`
}
