// Package scaffold writes a template plugin repository for
// "nginx-ui plugin init". Every file lives under templates/ as a
// text/template source and is rendered with the options the caller supplies.
package scaffold

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"text/template"
	"time"
)

//go:embed templates/*
var templatesFS embed.FS

// idPattern mirrors the plugin id grammar.
var idPattern = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9-]+)+$`)

// InitOptions configures Init.
type InitOptions struct {
	// ID is the plugin id, e.g. "io.github.example.mydns".
	ID string
	// Name is the human readable display name.
	Name string
	// Lang selects the implementation language: "go", "rust", "python" or "node".
	Lang string
	// Capability selects which capability to scaffold. Only "dns01" is
	// supported today; empty defaults to it.
	Capability string
}

// data is what every template file sees.
type data struct {
	ID             string
	Name           string
	Lang           string
	Capability     string
	ProviderCode   string
	ProviderName   string
	CredentialKey  string
	ModulePath     string
	Platform       string
	ExecutablePath string
	GoTypeName     string
	// BinaryName is the name of the compiled executable without a suffix.
	BinaryName  string
	Interpreter string
	Today       string
	Year        int
}

var funcMap = template.FuncMap{
	// json renders a Go string as a JSON string literal, so a template can
	// write `"key": {{json .Value}}` without hand rolling escaping rules.
	"json": func(s string) (string, error) {
		b, err := json.Marshal(s)
		return string(b), err
	},
}

// Init writes a template plugin repository into dir, creating it if needed.
func Init(dir string, opts InitOptions) error {
	if !idPattern.MatchString(opts.ID) {
		return fmt.Errorf(`scaffold: id %q must look like "vendor.name" (%s)`, opts.ID, idPattern)
	}
	if opts.Name == "" {
		return fmt.Errorf("scaffold: name is required")
	}
	capability := opts.Capability
	if capability == "" {
		capability = "dns01"
	}
	if capability != "dns01" {
		return fmt.Errorf("scaffold: capability %q is not supported yet, only \"dns01\" is", capability)
	}

	var langDir, interpreter string
	switch opts.Lang {
	case "go":
		langDir = "go"
	case "rust":
		langDir = "rust"
	case "python":
		langDir, interpreter = "python", "python3"
	case "node":
		langDir, interpreter = "node", "node"
	default:
		return fmt.Errorf("scaffold: lang %q must be one of go, rust, python, node", opts.Lang)
	}

	lastSegment := opts.ID[strings.LastIndex(opts.ID, ".")+1:]
	providerCode := sanitizeProviderCode(lastSegment)
	platform := runtime.GOOS + "-" + runtime.GOARCH
	execName := lastSegment
	if runtime.GOOS == "windows" {
		execName += ".exe"
	}

	d := data{
		ID:             opts.ID,
		Name:           opts.Name,
		Lang:           opts.Lang,
		Capability:     capability,
		ProviderCode:   providerCode,
		ProviderName:   opts.Name,
		CredentialKey:  strings.ToUpper(strings.ReplaceAll(providerCode, "-", "_")) + "_API_TOKEN",
		ModulePath:     "github.com/example/" + lastSegment, // TODO(user): pick a real module path.
		Platform:       platform,
		ExecutablePath: "dist/" + platform + "/" + execName,
		GoTypeName:     pascalCase(lastSegment),
		BinaryName:     lastSegment,
		Interpreter:    interpreter,
		Today:          time.Now().Format("2006-01-02"),
		Year:           time.Now().Year(),
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := renderDir(dir, "templates/common", d); err != nil {
		return err
	}
	if err := renderDir(dir, "templates/"+langDir, d); err != nil {
		return err
	}
	if opts.Lang == "go" || opts.Lang == "rust" {
		if err := writePlaceholderBinary(dir, d); err != nil {
			return err
		}
	}
	return nil
}

// renderDir renders every template under the embedded fsRoot into destRoot,
// stripping the ".tmpl" suffix and translating the "gitignore" filename
// go:embed cannot see back to ".gitignore".
func renderDir(destRoot, fsRoot string, d data) error {
	return fs.WalkDir(templatesFS, fsRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		rel := strings.TrimPrefix(path, fsRoot+"/")
		rel = strings.TrimSuffix(rel, ".tmpl")
		parts := strings.Split(rel, "/")
		if parts[len(parts)-1] == "gitignore" {
			parts[len(parts)-1] = ".gitignore"
		}
		target := filepath.Join(append([]string{destRoot}, parts...)...)

		raw, err := templatesFS.ReadFile(path)
		if err != nil {
			return err
		}
		tmpl, err := template.New(path).Funcs(funcMap).Parse(string(raw))
		if err != nil {
			return fmt.Errorf("scaffold: parse %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if parts[len(parts)-1] == "build.sh" {
			mode = 0o755
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			return err
		}
		defer f.Close()
		return tmpl.Execute(f, d)
	})
}

// writePlaceholderBinary drops a stub file where plugin.json's
// server.executables entry expects the real binary, so a freshly scaffolded
// compiled plugin already passes "nginx-ui plugin lint" before build.sh
// has ever run. build.sh overwrites it with the real build.
func writePlaceholderBinary(dir string, d data) error {
	target := filepath.Join(dir, filepath.FromSlash(d.ExecutablePath))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	placeholder := "#!/bin/sh\n" +
		"echo \"" + d.ID + " has not been built yet, run build.sh first\" >&2\n" +
		"exit 1\n"
	return os.WriteFile(target, []byte(placeholder), 0o755)
}

// sanitizeProviderCode turns a plugin id's last segment into a value that
// satisfies the dns01 provider code grammar.
func sanitizeProviderCode(s string) string {
	s = strings.ToLower(s)
	if len(s) < 2 {
		s += "-x"
	}
	if len(s) > 32 {
		s = s[:32]
	}
	return s
}

// pascalCase turns a hyphen/underscore/dot separated identifier into a Go
// exported-style type name, e.g. "my-dns" -> "MyDns".
func pascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	if b.Len() == 0 {
		return "Provider"
	}
	return b.String()
}
