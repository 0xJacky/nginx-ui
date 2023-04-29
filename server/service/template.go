package service

import (
	"bufio"
	"bytes"
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/0xJacky/Nginx-UI/server/settings"
	templ "github.com/0xJacky/Nginx-UI/template"
	"github.com/BurntSushi/toml"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/tufanbarisyildirim/gonginx/parser"
	"io"
	"log"
	"path/filepath"
	"strings"
	"text/template"
)

type TVariable struct {
	Type  string            `json:"type"`
	Name  map[string]string `json:"name"`
	Value interface{}       `json:"value"`
}

type ConfigInfoItem struct {
	Name        string               `json:"name"`
	Description map[string]string    `json:"description"`
	Author      string               `json:"author"`
	Filename    string               `json:"filename"`
	Variables   map[string]TVariable `json:"variables"`
}

func GetTemplateInfo(path, name string) (configListItem ConfigInfoItem) {
	configListItem = ConfigInfoItem{
		Description: make(map[string]string),
		Filename:    name,
	}

	file, _ := templ.DistFS.Open(filepath.Join(path, name))
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

	_, err = toml.Decode(content, &configListItem)
	if err != nil {
		log.Println("toml.Decode", err.Error())
	}
	return
}

type ConfigDetail struct {
	Custom string `json:"custom"`
	nginx.NgxServer
}

func ParseTemplate(path, name string, bindData map[string]TVariable) (c ConfigDetail, err error) {
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
		"HTTPPORT":   settings.ServerSettings.HttpPort,
		"HTTP01PORT": settings.ServerSettings.HTTPChallengePort,
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

	p := parser.NewStringParser(content)
	config := p.Parse()
	c.Custom = custom
	for _, d := range config.GetDirectives() {
		switch d.GetName() {
		case nginx.Location:
			l := &nginx.NgxLocation{
				Path: strings.Join(d.GetParameters(), " "),
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
