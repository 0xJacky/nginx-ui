package template

import (
	"bufio"
	"bytes"
	"io"
	"io/fs"
	dirPath "path"
	"regexp"
	"strings"
	"text/template"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	templ "github.com/0xJacky/Nginx-UI/template"
	"github.com/BurntSushi/toml"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/tufanbarisyildirim/gonginx/parser"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
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
	OriginPlugin  = "plugin"
	// OriginCustom is a snippet the user keeps in the snippets directory.
	OriginCustom = "custom"
)

type Variable struct {
	Type  string                       `json:"type" toml:"type"` // string, boolean, select
	Name  map[string]string            `json:"name" toml:"name,omitempty"`
	Value interface{}                  `json:"value" toml:"value"`
	Mask  map[string]map[string]string `json:"mask,omitempty" toml:"mask,omitempty"`
}

type ConfigInfoItem struct {
	Name string `json:"name"`
	// NameI18n is the name per language, when the template has one.
	NameI18n    map[string]string   `json:"name_i18n,omitempty" toml:"-"`
	Description map[string]string   `json:"description"`
	Author      string              `json:"author"`
	Filename    string              `json:"filename" toml:"-"`
	Variables   map[string]Variable `json:"variables"`
	// Origin is OriginBuiltin, OriginPlugin or OriginCustom.
	Origin string `json:"origin" toml:"-"`
	// PluginID names the plugin a template comes from.
	PluginID string `json:"plugin_id,omitempty" toml:"-"`
}

// location is where a template file lives.
type location struct {
	fsys     fs.FS
	origin   string
	pluginID string
}

var builtin = location{fsys: templ.DistFS, origin: OriginBuiltin}

// GetTemplateInfo reads the header of a built-in template. A header that
// does not parse is logged and leaves the fields it would have set empty.
func GetTemplateInfo(path, name string) ConfigInfoItem {
	info, err := readInfo(builtin, path, name)
	if err != nil {
		logger.Error(name, err)
	}
	return info
}

// GetPluginTemplateInfo reads the header of a template of an enabled plugin.
func GetPluginTemplateInfo(pluginID, path, name string) (ConfigInfoItem, error) {
	loc, err := pluginLocation(pluginID)
	if err != nil {
		return ConfigInfoItem{}, err
	}
	if !IsValidFileName(name) {
		return ConfigInfoItem{}, ErrTemplateNotFound
	}
	if err = checkPluginFile(loc.fsys, path, name); err != nil {
		return ConfigInfoItem{}, err
	}
	return readInfo(loc, path, name)
}

// readInfo decodes the header of one template.
func readInfo(loc location, path, name string) (ConfigInfoItem, error) {
	info, _, err := readSource(loc, path, name)
	return info, err
}

// readSource decodes the header of one template and returns the body below
// it. The fields that identify the file are set after decoding, so a header
// cannot change them.
func readSource(loc location, path, name string) (info ConfigInfoItem, body string, err error) {
	info = ConfigInfoItem{Description: map[string]string{}}
	defer func() {
		info.Filename = name
		info.Origin = loc.origin
		info.PluginID = loc.pluginID
	}()

	file, err := loc.fsys.Open(dirPath.Join(path, name))
	if err != nil {
		return info, "", err
	}
	defer file.Close()

	var header string
	header, body, err = splitHeader(io.LimitReader(file, maxTemplateSize))
	if err != nil {
		return info, "", err
	}
	if _, err = toml.Decode(header, &info); err != nil {
		return info, "", errors.Wrap(err, "decode template header")
	}
	if info.Description == nil {
		info.Description = map[string]string{}
	}
	return info, body, nil
}

// splitHeader returns the TOML header between the marker lines and the rest
// of the file.
func splitHeader(r io.Reader) (header, body string, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTemplateSize)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != HeaderStart {
		if scanner.Err() != nil {
			return "", "", scanner.Err()
		}
		return "", "", errors.Errorf("the first line must be %q", HeaderStart)
	}

	var headerBuf, bodyBuf strings.Builder
	inHeader := true
	for scanner.Scan() {
		line := scanner.Text()
		if inHeader {
			if strings.TrimSpace(line) == HeaderEnd {
				inHeader = false
				continue
			}
			headerBuf.WriteString(strings.TrimSpace(line))
			headerBuf.WriteString("\n")
			continue
		}
		bodyBuf.WriteString(line)
		bodyBuf.WriteString("\n")
	}
	if err = scanner.Err(); err != nil {
		return "", "", err
	}
	if inHeader {
		return "", "", errors.Errorf("the line %q is missing", HeaderEnd)
	}
	return headerBuf.String(), bodyBuf.String(), nil
}

type ConfigDetail struct {
	Custom string `json:"custom"`
	nginx.NgxServer
}

// ParseTemplate renders a built-in template with the given variables.
func ParseTemplate(path, name string, bindData map[string]Variable) (c ConfigDetail, err error) {
	return parseTemplate(builtin, path, name, bindData)
}

// ParsePluginTemplate renders a template of an enabled plugin.
func ParsePluginTemplate(pluginID, path, name string, bindData map[string]Variable) (ConfigDetail, error) {
	loc, err := pluginLocation(pluginID)
	if err != nil {
		return ConfigDetail{}, err
	}
	if !IsValidFileName(name) {
		return ConfigDetail{}, ErrTemplateNotFound
	}
	if err = checkPluginFile(loc.fsys, path, name); err != nil {
		return ConfigDetail{}, err
	}
	return parseTemplate(loc, path, name, bindData)
}

func parseTemplate(loc location, path, name string, bindData map[string]Variable) (c ConfigDetail, err error) {
	file, err := loc.fsys.Open(dirPath.Join(path, name))
	if err != nil {
		err = errors.Wrap(err, "error tokenized template")
		return
	}
	defer file.Close()

	// A plugin template is untrusted input, so it may only use the actions
	// and functions the built-in templates need, and its output is bounded.
	restricted := loc.origin == OriginPlugin

	return parseContent(name, io.LimitReader(file, maxTemplateSize), bindData, restricted)
}

// RenderBlock renders a block template held in memory, such as a snippet of
// the user. Content without a header is a block without variables.
func RenderBlock(name string, content []byte, bindData map[string]Variable) (ConfigDetail, error) {
	if !bytes.Contains(content, []byte(HeaderEnd)) {
		content = append([]byte(HeaderStart+"\n"+HeaderEnd+"\n"), content...)
	}
	return parseContent(name, bytes.NewReader(content), bindData, false)
}

// templateData is what a template is rendered with: the values every
// template gets and the variables.
func templateData(bindData map[string]Variable) gin.H {
	data := gin.H{
		"HTTPPORT":        cSettings.ServerSettings.Port,
		"UNIXSOCKET":      settings.ListenerSettings.UnixSocket,
		"NGINXUIUPSTREAM": settings.ListenerSettings.LocalUpstream(*cSettings.ServerSettings),
		"HTTP01PORT":      settings.CertSettings.HTTPChallengePort,
	}
	for k, v := range bindData {
		data[k] = v.Value
	}
	return data
}

// Actions that print nothing: control structure, comments and assignments.
var (
	silentLine   = regexp.MustCompile(`^[ \t]*(\{\{-?\s*(?:(?:if|else|end|range|with|break|continue|define|block)\b|/\*|\$\w*\s*:?=)(?:[^}]|\}[^}])*\}\}[ \t]*)+$`)
	actionOpener = regexp.MustCompile(`^([ \t]*)\{\{-?`)
)

// TrimActionLines drops the lines that hold only actions printing nothing,
// such as "    {{ if .gzip }}", from the output, as if they began with "{{-":
// the indentation and the line break before them go, so an if block leaves
// no blank lines. Lines that print a value keep their place.
func TrimActionLines(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !silentLine.MatchString(line) {
			continue
		}
		if i == 0 {
			// Nothing comes before the first line, so the break after it goes.
			// A trim marker needs a space next to it, so "{{end}}" becomes
			// "{{end -}}".
			if trimmed := strings.TrimRight(line, " \t"); !strings.HasSuffix(trimmed, " -}}") {
				lines[i] = strings.TrimSuffix(trimmed, "}}") + " -}}"
			}
			continue
		}
		lines[i] = actionOpener.ReplaceAllString(line, "$1{{- ")
	}
	return strings.Join(lines, "\n")
}

// RenderText renders the body of a block template to text and checks that
// the result parses as nginx configuration.
func RenderText(name, content string, bindData map[string]Variable) (string, error) {
	return renderText(name, content, bindData, false)
}

// RenderPluginText renders a block template of a plugin like RenderText,
// with the limits every plugin template is rendered with.
func RenderPluginText(name, content string, bindData map[string]Variable) (string, error) {
	return renderText(name, content, bindData, true)
}

func renderText(name, content string, bindData map[string]Variable, restricted bool) (string, error) {
	t, err := template.New(name).Parse(TrimActionLines(content))
	if err != nil {
		return "", err
	}
	if restricted {
		if err = checkRestricted(t); err != nil {
			return "", err
		}
	}
	rendered, err := execute(t, templateData(bindData), restricted)
	if err != nil {
		return "", err
	}
	if _, err = parser.NewStringParser(rendered, parser.WithSkipValidDirectivesErr()).Parse(); err != nil {
		return rendered, err
	}
	return rendered, nil
}

func parseContent(name string, source io.Reader, bindData map[string]Variable, restricted bool) (c ConfigDetail, err error) {
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

	data := templateData(bindData)

	custom, err = render(name, custom, data, restricted)
	if err != nil {
		err = errors.Wrap(err, "error parse template.custom")
		return
	}
	custom = strings.TrimSpace(custom)

	templatePart := strings.Split(content, HeaderEnd)
	if len(templatePart) < 2 {
		return
	}

	content, err = render(name, templatePart[1], data, restricted)
	if err != nil {
		err = errors.Wrap(err, "error parse template")
		return
	}

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

// ErrBuiltinNotFound is returned for a built-in block template that does not
// exist.
var ErrBuiltinNotFound = errors.New("built-in template not found")

// BuiltinBlockSource returns a built-in block template as written: what its
// header holds and the body below the header.
func BuiltinBlockSource(name string) (ConfigInfoItem, string, error) {
	if name != dirPath.Base(name) || !strings.HasSuffix(name, ".conf") {
		return ConfigInfoItem{}, "", ErrBuiltinNotFound
	}
	content, err := templ.DistFS.ReadFile(dirPath.Join("block", name))
	if err != nil {
		return ConfigInfoItem{}, "", ErrBuiltinNotFound
	}
	info := GetTemplateInfo("block", name)
	body := string(content)
	if _, after, found := strings.Cut(body, HeaderEnd); found {
		body = after
	}
	return info, strings.TrimLeft(body, "\r\n"), nil
}

// PluginBlockSource returns a block template of an enabled plugin as
// written, like BuiltinBlockSource.
func PluginBlockSource(pluginID, name string) (ConfigInfoItem, string, error) {
	loc, err := pluginLocation(pluginID)
	if err != nil {
		return ConfigInfoItem{}, "", err
	}
	if !IsValidFileName(name) {
		return ConfigInfoItem{}, "", ErrTemplateNotFound
	}
	if err = checkPluginFile(loc.fsys, "block", name); err != nil {
		return ConfigInfoItem{}, "", err
	}
	info, body, err := readSource(loc, "block", name)
	if err != nil {
		return ConfigInfoItem{}, "", ErrTemplateNotFound
	}
	return info, strings.TrimLeft(body, "\r\n"), nil
}

// render parses and executes one template section.
func render(name, text string, data gin.H, restricted bool) (string, error) {
	t, err := template.New(name).Parse(text)
	if err != nil {
		return "", err
	}
	if restricted {
		if err = checkRestricted(t); err != nil {
			return "", err
		}
	}
	rendered, err := execute(t, data, restricted)
	if err != nil {
		return "", errors.Wrap(err, "error execute template")
	}
	return rendered, nil
}

// execute runs a parsed template. The output of a restricted one is
// bounded.
func execute(t *template.Template, data gin.H, restricted bool) (string, error) {
	var buf bytes.Buffer
	var out io.Writer = &buf
	if restricted {
		out = &limitedWriter{w: &buf, left: maxRenderedSize}
	}
	if err := t.Execute(out, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// GetTemplateList lists the built-in templates of a kind ("conf" or
// "block") followed by the templates every enabled plugin contributes.
func GetTemplateList(path string) (configList []ConfigInfoItem, err error) {
	configs, err := templ.DistFS.ReadDir(path)
	if err != nil {
		err = errors.Wrap(err, "error get template list")
		return
	}

	for _, config := range configs {
		configList = append(configList, GetTemplateInfo(path, config.Name()))
	}

	configList = append(configList, pluginTemplateList(path)...)
	return
}
