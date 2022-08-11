package api

import (
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func GetTemplate(c *gin.Context) {
	content := `proxy_set_header Host $host;
proxy_set_header X-Real_IP $remote_addr;
proxy_set_header X-Forwarded-For $remote_addr:$remote_port;
proxy_pass http://127.0.0.1:{{ HTTP01PORT }};
`
	content = strings.ReplaceAll(content, "{{ HTTP01PORT }}",
		settings.ServerSettings.HTTPChallengePort)

	var ngxConfig *nginx.NgxConfig

	ngxConfig = &nginx.NgxConfig{
		Servers: []*nginx.NgxServer{
			{
				Directives: []*nginx.NgxDirective{
					{
						Directive: "listen",
						Params:    "80",
					},
					{
						Directive: "listen",
						Params:    "[::]:80",
					},
					{
						Directive: "server_name",
					},
					{
						Directive: "root",
					},
					{
						Directive: "index",
					},
				},
				Locations: []*nginx.NgxLocation{
					{
						Path:    "/.well-known/acme-challenge",
						Content: content,
					},
				},
			},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "ok",
		"template":  ngxConfig.BuildConfig(),
		"tokenized": ngxConfig,
	})
}
