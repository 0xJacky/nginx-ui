package model

import (
	"net/url"
	"strings"
)

type Node struct {
	Model
	Name    string `json:"name"`
	URL     string `json:"url"`
	Token   string `json:"token"`
	Enabled bool   `json:"enabled" gorm:"default:false"`
}

func (n *Node) GetUrl(uri string) (decodedUri string, err error) {
	baseUrl, err := url.Parse(n.URL)
	if err != nil {
		return
	}

	u, err := url.JoinPath(baseUrl.String(), uri)
	if err != nil {
		return
	}

	decodedUri, err = url.QueryUnescape(u)
	if err != nil {
		return
	}

	return
}

func (n *Node) GetWebSocketURL(uri string) (decodedUri string, err error) {
	baseUrl, err := url.Parse(n.URL)
	if err != nil {
		return
	}

	defaultPort := ""
	if baseUrl.Port() == "" {
		switch baseUrl.Scheme {
		default:
			fallthrough
		case "http":
			defaultPort = "80"
		case "https":
			defaultPort = "443"
		}

		baseUrl.Host = baseUrl.Hostname() + ":" + defaultPort
	}

	u, err := url.JoinPath(baseUrl.String(), uri)

	if err != nil {
		return
	}

	decodedUri, err = url.QueryUnescape(u)

	if err != nil {
		return
	}
	// http will be replaced with ws, https will be replaced with wss
	decodedUri = strings.ReplaceAll(decodedUri, "http", "ws")
	return
}
