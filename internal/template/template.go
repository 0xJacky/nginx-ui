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
	dirPath "path"
	"strings"
	"text/template"
)

// Marker lines of the template format.
const (
	HeaderStart = "# Nginx UI Template Start"
	HeaderEnd   = "# Nginx UI Template End"
	customStart = "# Nginx UI Custom Start"
	customEnd   = "# Nginx UI Custom End"
)

// Origins of a template.
const (
	OriginBuiltin = "builtin"
	// OriginCustom is a snippet the user keeps in the snippets directory.
	OriginCustom = "custom"
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
	// Origin is OriginBuiltin or OriginCustom.
	Origin string `json:"origin" toml:"-"`
}

func GetTemplateInfo(path, name string) (configListItem ConfigInfoItem) {
	configListItem = ConfigInfoItem{
		Description: make(map[string]string),
		Filename:    name,
		Origin:      OriginBuiltin,
	}

	file, err := templ.DistFS.Open(dirPath.Join(path, name))
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

	if line != HeaderStart {
		return
	}
	var content string
	for {
		lineBytes, _, err = r.ReadLine()
		if err == io.EOF {
			break
		}
		line = strings.TrimSpace(string(lineBytes))
		if line == HeaderEnd {
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
	file, err := templ.DistFS.Open(dirPath.Join(path, name))
	if err != nil {
		err = errors.Wrap(err, "error tokenized template")
		return
	}
	defer file.Close()

	return parseContent(name, file, bindData)
}

// RenderBlock renders a block template held in memory, such as a snippet of
// the user. Content without a header is a block without variables.
func RenderBlock(name string, content []byte, bindData map[string]Variable) (ConfigDetail, error) {
	if !bytes.Contains(content, []byte(HeaderEnd)) {
		content = append([]byte(HeaderStart+"\n"+HeaderEnd+"\n"), content...)
	}
	return parseContent(name, bytes.NewReader(content), bindData)
}

func parseContent(name string, source io.Reader, bindData map[string]Variable) (c ConfigDetail, err error) {
	r := bufio.NewReader(source)
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
		case line == customStart:
			flag = true
		case line == customEnd:
			flag = false
		case flag == true:
			custom += orig + "\n"
		case flag == false:
			content += orig + "\n"
		}
	}

	data := gin.H{
		"HTTPPORT":        cSettings.ServerSettings.Port,
		"UNIXSOCKET":      settings.ListenerSettings.UnixSocket,
		"NGINXUIUPSTREAM": settings.ListenerSettings.LocalUpstream(*cSettings.ServerSettings),
		"HTTP01PORT":      settings.CertSettings.HTTPChallengePort,
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

	templatePart := strings.Split(content, HeaderEnd)
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
