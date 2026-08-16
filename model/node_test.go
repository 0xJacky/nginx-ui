package model

import "testing"

func TestNodeGetWebSocketURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		uri  string
		want string
	}{
		{
			name: "https gets the wss scheme and the default port",
			url:  "https://node.example.com",
			uri:  "/api/analytic/intro",
			want: "wss://node.example.com:443/api/analytic/intro",
		},
		{
			name: "http gets the ws scheme and the default port",
			url:  "http://node.example.com",
			uri:  "/api/analytic/intro",
			want: "ws://node.example.com:80/api/analytic/intro",
		},
		{
			name: "an explicit port is kept",
			url:  "https://node.example.com:9000",
			uri:  "/api/analytic/intro",
			want: "wss://node.example.com:9000/api/analytic/intro",
		},
		{
			name: "a host containing http is left intact",
			url:  "https://http-proxy.example.com:9000",
			uri:  "/api/analytic/intro",
			want: "wss://http-proxy.example.com:9000/api/analytic/intro",
		},
		{
			name: "a base path containing http is left intact",
			url:  "http://10.0.0.1:9000/httpd",
			uri:  "/api/analytic/intro",
			want: "ws://10.0.0.1:9000/httpd/api/analytic/intro",
		},
		{
			name: "a query value containing http is left intact",
			url:  "https://node.example.com:9000",
			uri:  "/api/nginx_log?type=http",
			want: "wss://node.example.com:9000/api/nginx_log?type=http",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := &Node{URL: test.url}
			got, err := node.GetWebSocketURL(test.uri)
			if err != nil {
				t.Fatalf("GetWebSocketURL returned error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}
