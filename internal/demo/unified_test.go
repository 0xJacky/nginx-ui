package demo_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// allowedDemoReaders are the files permitted to read settings.NodeSettings.Demo
// directly. Everything else must go through a provider slot filled by
// internal/demo, so that fabricated behaviour lives in one place instead of
// growing back as per-module branches.
//
// The four api/ entries are refusals, not fabrication. A refusal expressed as a
// flag branch fails loudly when it is wrong; fabrication expressed the same way
// fails silently, which is why only refusals are grandfathered here.
var allowedDemoReaders = map[string]string{
	"internal/demo/demo.go":       "the single reader the rest of the tree delegates to",
	"internal/middleware/demo.go": "RejectInDemo/DemoReadOnly route guards",
	"api/terminal/pty.go":         "refuses the PTY upgrade",
	"api/user/otp.go":             "refuses TOTP enrollment",
	"api/user/user.go":            "refuses editing the initial user",
	"internal/upgrader/binary.go": "refuses to install a self-upgrade",
	"settings/node.go":            "declares the field",
	"settings/server_v1.go":       "migrates the legacy key",
	"api/settings/settings.go":    "exposes the flag to the frontend",
	"api/public/layout.go":        "exposes the flag to the frontend before login",
}

func TestNoDemoBranchesOutsideDemoPackage(t *testing.T) {
	root := repoRoot(t)

	var offenders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if _, ok := allowedDemoReaders[filepath.ToSlash(rel)]; ok {
			return nil
		}

		if readsDemoFlag(t, path) {
			offenders = append(offenders, filepath.ToSlash(rel))
		}
		return nil
	})
	require.NoError(t, err)

	require.Empty(t, offenders,
		"these files read settings.NodeSettings.Demo directly.\n"+
			"Fabricated demo behaviour belongs behind a provider slot filled by internal/demo, "+
			"not behind an inline flag check. If the file genuinely refuses an action rather than "+
			"faking one, add it to allowedDemoReaders with a reason.")
}

// readsDemoFlag reports whether the file contains a NodeSettings.Demo selector.
func readsDemoFlag(t *testing.T, path string) bool {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		// A file this package cannot parse is not this test's problem.
		return false
	}

	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Demo" {
			return true
		}
		inner, ok := sel.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if inner.Sel.Name == "NodeSettings" || inner.Sel.Name == "Node" {
			found = true
			return false
		}
		return true
	})
	return found
}

func skipDir(name string) bool {
	switch name {
	case ".git", ".go", "node_modules", ".claude", "app", "docs", "dist", "testdata":
		return true
	}
	return false
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for range 10 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "reached the filesystem root without finding go.mod")
		dir = parent
	}
	t.Fatal("could not locate the repository root")
	return ""
}
