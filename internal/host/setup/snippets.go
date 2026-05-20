package setup

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/uozi-tech/cosy"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// Rendered is the structured result returned to API callers and CLI --json.
type Rendered struct {
	ComposeSnippet  string `json:"compose_snippet"`
	ComposeOverride string `json:"compose_override"`
	DockerRun       string `json:"docker_run"`
	AuthorizedKeys  string `json:"authorized_keys"`
	Sudoers         string `json:"sudoers"`
	ACLCommands     string `json:"acl_commands"`
}

func renderTemplate(name string, p SetupParams) (string, error) {
	t, err := template.ParseFS(templateFS, "templates/"+name)
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrTemplateRender, name, err.Error())
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, p); err != nil {
		return "", cosy.WrapErrorWithParams(ErrTemplateRender, name, err.Error())
	}
	// Normalise to LF endings; templates may pick up CRLF on Windows checkouts.
	return strings.ReplaceAll(buf.String(), "\r\n", "\n"), nil
}

func RenderCompose(p SetupParams) (string, error) {
	return renderTemplate("compose.yml.tmpl", p.FillDefaults())
}

func RenderComposeOverride(p SetupParams) (string, error) {
	return renderTemplate("compose.override.yml.tmpl", p.FillDefaults())
}

func RenderDockerRun(p SetupParams) (string, error) {
	return renderTemplate("docker_run.sh.tmpl", p.FillDefaults())
}

func RenderAuthorizedKeys(p SetupParams) (string, error) {
	return renderTemplate("authorized_keys.tmpl", p.FillDefaults())
}

func RenderSudoers(p SetupParams) (string, error) {
	return renderTemplate("sudoers.tmpl", p.FillDefaults())
}

func RenderACL(p SetupParams) (string, error) {
	return renderTemplate("acl_commands.sh.tmpl", p.FillDefaults())
}

// RenderAll renders all six snippets in one go.
func RenderAll(p SetupParams) (*Rendered, error) {
	p = p.FillDefaults()
	r := &Rendered{}
	var err error
	if r.ComposeSnippet, err = RenderCompose(p); err != nil {
		return nil, fmt.Errorf("compose: %w", err)
	}
	if r.ComposeOverride, err = RenderComposeOverride(p); err != nil {
		return nil, fmt.Errorf("override: %w", err)
	}
	if r.DockerRun, err = RenderDockerRun(p); err != nil {
		return nil, fmt.Errorf("docker run: %w", err)
	}
	if r.AuthorizedKeys, err = RenderAuthorizedKeys(p); err != nil {
		return nil, fmt.Errorf("authorized_keys: %w", err)
	}
	if r.Sudoers, err = RenderSudoers(p); err != nil {
		return nil, fmt.Errorf("sudoers: %w", err)
	}
	if r.ACLCommands, err = RenderACL(p); err != nil {
		return nil, fmt.Errorf("acl: %w", err)
	}
	return r, nil
}
