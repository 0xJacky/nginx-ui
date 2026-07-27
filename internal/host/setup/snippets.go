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
	// AuthorizedKeysInstall appends the key with the permissions sshd requires.
	// sshd silently ignores authorized_keys when they are too permissive.
	AuthorizedKeysInstall string `json:"authorized_keys_install"`
	Sudoers               string `json:"sudoers"`
	ACLCommands           string `json:"acl_commands"`
	// SudoersRequired lets the UI hide the sudoers instructions and their
	// checks when the target needs none.
	SudoersRequired bool `json:"sudoers_required"`
}

// shellQuote wraps a value in single quotes so a public key comment cannot
// break out of the generated command.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

var templateFuncs = template.FuncMap{"shellQuote": shellQuote}

func renderTemplate(name string, p SetupParams) (string, error) {
	t, err := template.New(name).Funcs(templateFuncs).ParseFS(templateFS, "templates/"+name)
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

func RenderAuthorizedKeysInstall(p SetupParams) (string, error) {
	return renderTemplate("authorized_keys_install.sh.tmpl", p.FillDefaults())
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
	if err := p.ValidateSnippetValues(); err != nil {
		return nil, err
	}
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
	if r.AuthorizedKeysInstall, err = RenderAuthorizedKeysInstall(p); err != nil {
		return nil, fmt.Errorf("authorized_keys install: %w", err)
	}
	if r.Sudoers, err = RenderSudoers(p); err != nil {
		return nil, fmt.Errorf("sudoers: %w", err)
	}
	if r.ACLCommands, err = RenderACL(p); err != nil {
		return nil, fmt.Errorf("acl: %w", err)
	}
	r.SudoersRequired = p.NeedsSudoers()
	return r, nil
}
