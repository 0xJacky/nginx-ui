package service

import (
	"bufio"
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/0xJacky/Nginx-UI/template"
	"github.com/pkg/errors"
	"github.com/tufanbarisyildirim/gonginx/parser"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

type ConfigInfoItem struct {
	Name        string            `json:"name"`
	Description map[string]string `json:"description"`
	Author      string            `json:"author"`
	Filename    string            `json:"filename"`
}

func GetTemplateInfo(path, name string) (configListItem ConfigInfoItem) {
	configListItem = ConfigInfoItem{
		Description: make(map[string]string),
		Filename:    name,
	}

	file, _ := template.DistFS.Open(filepath.Join(path, name))
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

	return
}

type ConfigDetail struct {
	Custom string `json:"custom"`
	nginx.NgxServer
}

func ParseTemplate(path, name string) (c ConfigDetail, err error) {
	file, err := template.DistFS.Open(filepath.Join(path, name))
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
		bytes, _, err := r.ReadLine()
		if err == io.EOF {
			break
		}
		orig := string(bytes)
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
	content = strings.ReplaceAll(content, "{{ HTTP01PORT }}", settings.ServerSettings.HTTPChallengePort)
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
	configs, err := template.DistFS.ReadDir(path)
	if err != nil {
		err = errors.Wrap(err, "error get template list")
		return
	}

	for _, config := range configs {
		configList = append(configList, GetTemplateInfo(path, config.Name()))
	}

	return
}
