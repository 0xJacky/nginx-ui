package template

import (
	"io"
	"io/fs"
	dirPath "path"
	"regexp"
	"slices"
	"text/template"
	"text/template/parse"

	"github.com/pkg/errors"
)

const (
	// maxTemplateSize bounds the file size of one template.
	maxTemplateSize = 256 << 10
	// maxRenderedSize bounds what one section of a plugin template renders to.
	maxRenderedSize = 1 << 20
)

// Kinds of template, the directory names below a template root.
const (
	KindConf  = "conf"
	KindBlock = "block"
)

// Kinds lists the template kinds.
var Kinds = []string{KindConf, KindBlock}

// fileNamePattern keeps a template name safe in a URL and a path.
var fileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}\.conf$`)

// variableTypes lists the types a template variable may have.
var variableTypes = []string{"string", "boolean", "select"}

// allowedFuncs lists the template functions a plugin template may call. The
// others can build unbounded output or call into the data.
var allowedFuncs = []string{
	"and", "or", "not", "eq", "ne", "lt", "le", "gt", "ge",
	"len", "index", "print", "println", "html", "js", "urlquery",
}

// IsValidFileName reports whether name may be the file name of a plugin
// template.
func IsValidFileName(name string) bool {
	return fileNamePattern.MatchString(name)
}

// TemplateFiles lists the template files of one kind below a template root,
// in name order. Entries that are not templates are left out.
func TemplateFiles(fsys fs.FS, kind string) []string {
	return templateFiles(fsys, kind)
}

// ValidateFile checks one template of a plugin the way the host reads it:
// the marker lines, the TOML header, the variable types, and a render with
// the default values whose output must be valid nginx syntax.
func ValidateFile(fsys fs.FS, kind, name string) error {
	if !IsValidFileName(name) {
		return errors.Errorf("file name %q must match %s", name, fileNamePattern)
	}
	if err := checkPluginFile(fsys, kind, name); err != nil {
		return errors.New("not a regular file in a real directory")
	}
	info, err := fs.Lstat(fsys, dirPath.Join(kind, name))
	if err != nil {
		return err
	}
	if info.Size() > maxTemplateSize {
		return errors.Errorf("the file is larger than %d bytes", maxTemplateSize)
	}

	loc := location{fsys: fsys, origin: OriginPlugin}
	header, err := readInfo(loc, kind, name)
	if err != nil {
		return err
	}
	for key, variable := range header.Variables {
		if !slices.Contains(variableTypes, variable.Type) {
			return errors.Errorf("variable %q has unknown type %q", key, variable.Type)
		}
		if variable.Type == "select" && len(variable.Mask) == 0 {
			return errors.Errorf("select variable %q needs a mask", key)
		}
	}

	if _, err = parseTemplate(loc, kind, name, header.Variables); err != nil {
		return err
	}
	return nil
}

// HasName reports whether a template header names the template.
func HasName(fsys fs.FS, kind, name string) bool {
	info, err := readInfo(location{fsys: fsys, origin: OriginPlugin}, kind, name)
	return err == nil && info.Name != ""
}

// checkRestricted rejects the template constructs a plugin template may not
// use: nested template definitions, loops and functions outside
// allowedFuncs.
func checkRestricted(t *template.Template) error {
	if len(t.Templates()) > 1 {
		return errors.New("defining templates is not allowed")
	}
	if t.Tree == nil || t.Root == nil {
		return nil
	}
	return walkRestricted(t.Root)
}

func walkRestricted(node parse.Node) error {
	switch n := node.(type) {
	case nil:
		return nil
	case *parse.ListNode:
		if n == nil {
			return nil
		}
		for _, child := range n.Nodes {
			if err := walkRestricted(child); err != nil {
				return err
			}
		}
	case *parse.ActionNode:
		return walkRestricted(n.Pipe)
	case *parse.PipeNode:
		if n == nil {
			return nil
		}
		for _, cmd := range n.Cmds {
			if err := walkRestricted(cmd); err != nil {
				return err
			}
		}
	case *parse.CommandNode:
		for _, arg := range n.Args {
			if err := walkRestricted(arg); err != nil {
				return err
			}
		}
	case *parse.IdentifierNode:
		if !slices.Contains(allowedFuncs, n.Ident) {
			return errors.Errorf("function %q is not allowed", n.Ident)
		}
	case *parse.IfNode:
		return walkBranch(&n.BranchNode)
	case *parse.WithNode:
		return walkBranch(&n.BranchNode)
	case *parse.RangeNode:
		return errors.New("range is not allowed")
	case *parse.TemplateNode:
		return errors.New("calling a template is not allowed")
	case *parse.ChainNode:
		return walkRestricted(n.Node)
	}
	return nil
}

func walkBranch(n *parse.BranchNode) error {
	if err := walkRestricted(n.Pipe); err != nil {
		return err
	}
	if err := walkRestricted(n.List); err != nil {
		return err
	}
	return walkRestricted(n.ElseList)
}

// limitedWriter fails once more than left bytes were written.
type limitedWriter struct {
	w    io.Writer
	left int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if len(p) > l.left {
		return 0, errors.Errorf("the template renders to more than %d bytes", maxRenderedSize)
	}
	l.left -= len(p)
	return l.w.Write(p)
}
