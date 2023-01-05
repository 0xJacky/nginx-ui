package api

import (
	"bufio"
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/0xJacky/Nginx-UI/template"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
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

func GetTemplateConfList(c *gin.Context) {
	configs, err := template.DistFS.ReadDir("conf")
	if err != nil {
		ErrHandler(c, err)
		return
	}
	type configItem struct {
		Name        string            `json:"name"`
		Description map[string]string `json:"description"`
		Author      string            `json:"author"`
	}

	var configList []configItem
	for _, config := range configs {
		func() {
			configListItem := configItem{
				Description: make(map[string]string),
			}

			file, _ := template.DistFS.Open(filepath.Join("conf", config.Name()))
			defer file.Close()
			r := bufio.NewReader(file)
			bytes, _, err := r.ReadLine()
			if err == io.EOF {
				return
			}
			line := strings.TrimSpace(string(bytes))

			if line != "# Nginx UI Template Start" {
				return
			}
			var content string
			for {
				bytes, _, err = r.ReadLine()
				if err == io.EOF {
					break
				}
				line = strings.TrimSpace(string(bytes))
				if line == "# Nginx UI Template End" {
					break
				}
				content += line + "\n"
			}
			re := regexp.MustCompile(`# (\S+): (.*)`)
			matches := re.FindAllStringSubmatch(content, -1)
			for _, match := range matches {
				if len(match) < 3 {
					continue
				}
				key := match[1]
				switch {
				case key == "Name":
					configListItem.Name = match[2]
				case key == "Author":
					configListItem.Author = match[2]
				case strings.Contains(key, "Description"):
					re = regexp.MustCompile(`(\w+)\[(\w+)\]`)
					matches = re.FindAllStringSubmatch(key, -1)
					for _, m := range matches {
						if len(m) < 3 {
							continue
						}
						// lang => description
						configListItem.Description[m[2]] = match[2]
					}
				}
			}

			configList = append(configList, configListItem)
		}()

	}

	c.JSON(http.StatusOK, gin.H{
		"data": configList,
	})
}
