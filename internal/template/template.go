package template

import (
	"bufio"
	"bytes"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	templ "github.com/0xJacky/Nginx-UI/template"
	"github.com/BurntSushi/toml"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/tufanbarisyildirim/gonginx/parser"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
	"io"
	"path/filepath"
	"strings"
	"text/template"
)

type Variable struct {
	Type  string                       `json:"type"` // string, bool, select
	Name  map[string]string            `json:"name"`
	Value interface{}                  `json:"value"`
	Mask  map[string]map[string]string `json:"mask,omitempty"`
}

type ConfigInfoItem struct {
	Name        string              `json:"name"`
	Description map[string]string   `json:"description"`
	Author      string              `json:"author"`
	Filename    string              `json:"filename"`
	Variables   map[string]Variable `json:"variables"`
}

func GetTemplateInfo(path, name string) (configListItem ConfigInfoItem) {
	configListItem = ConfigInfoItem{
		Description: make(map[string]string),
		Filename:    name,
	}

	file, err := templ.DistFS.Open(filepath.Join(path, name))
	if err != nil {
		logger.Error(err)
		return
	}

	defer file.Close()

	r := bufio.NewReader(file)
	lineBytes, _, err := r.ReadLine()
	if err == io.EOF {
		return
	}
	line := strings.TrimSpace(string(lineBytes))

	if line != "# Nginx UI Template Start" {
		return
	}
	var content string
	for {
		lineBytes, _, err = r.ReadLine()
		if err == io.EOF {
			break
		}
		line = strings.TrimSpace(string(lineBytes))
		if line == "# Nginx UI Template End" {
			break
		}
		content += line + "\n"
	}

	_, err = toml.Decode(content, &configListItem)
	if err != nil {
		logger.Error(name, err)
	}
	return
}

type ConfigDetail struct {
	Custom string `json:"custom"`
	nginx.NgxServer
}

func ParseTemplate(path, name string, bindData map[string]Variable) (c ConfigDetail, err error) {
	file, err := templ.DistFS.Open(filepath.Join(path, name))
	if err != nil {
		err = errors.Wrap(err, "error tokenized template")
		return
	}
	defer file.Close()

	r := bufio.NewReader(file)
	var flag bool
	custom := ""
	content := ""
	for {
		lineBytes, _, err := r.ReadLine()
		if err == io.EOF {
			break
		}
		orig := string(lineBytes)
		line := strings.TrimSpace(orig)
		switch {
		case line == "# Nginx UI Custom Start":
			flag = true
		case line == "# Nginx UI Custom End":
			flag = false
		case flag == true:
			custom += orig + "\n"
		case flag == false:
			content += orig + "\n"
		}
	}

	data := gin.H{
		"HTTPPORT":   cSettings.ServerSettings.Port,
		"HTTP01PORT": settings.CertSettings.HTTPChallengePort,
	}

	for k, v := range bindData {
		data[k] = v.Value
	}

	t, err := template.New(name).Parse(custom)
	if err != nil {
		err = errors.Wrap(err, "error parse template.custom")
		return
	}

	var buf bytes.Buffer

	err = t.Execute(&buf, data)
	if err != nil {
		err = errors.Wrap(err, "error execute template")
		return
	}

	custom = strings.TrimSpace(buf.String())

	templatePart := strings.Split(content, "# Nginx UI Template End")
	if len(templatePart) < 2 {
		return
	}

	content = templatePart[1]

	t, err = template.New(name).Parse(content)
	if err != nil {
		err = errors.Wrap(err, "error parse template")
		return
	}

	buf.Reset()

	err = t.Execute(&buf, data)
	if err != nil {
		err = errors.Wrap(err, "error execute template")
		return
	}

	content = buf.String()

	p := parser.NewStringParser(content, parser.WithSkipValidDirectivesErr())
	config, err := p.Parse()
	if err != nil {
		return
	}
	c.Custom = custom
	for _, d := range config.GetDirectives() {
		switch d.GetName() {
		case nginx.Location:
			var params []string
			for _, param := range d.GetParameters() {
				params = append(params, param.Value)
			}
			l := &nginx.NgxLocation{
				Path: strings.Join(params, " "),
			}
			l.ParseLocation(d, 0)
			c.NgxServer.Locations = append(c.NgxServer.Locations, l)
		default:
			dir := &nginx.NgxDirective{
				Directive: d.GetName(),
			}
			dir.ParseDirective(d, 0)
			c.NgxServer.Directives = append(c.NgxServer.Directives, dir)
		}
	}
	return
}

func GetTemplateList(path string) (configList []ConfigInfoItem, err error) {
	configs, err := templ.DistFS.ReadDir(path)
	if err != nil {
		err = errors.Wrap(err, "error get template list")
		return
	}

	for _, config := range configs {
		configList = append(configList, GetTemplateInfo(path, config.Name()))
	}

	return
}
