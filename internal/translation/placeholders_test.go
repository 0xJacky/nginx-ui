package translation

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/0xJacky/pofile"
)

// placeholderPattern matches the %{name} interpolation syntax used by the
// front-end and back-end translators.
var placeholderPattern = regexp.MustCompile(`%\{[^}]*\}`)

// brokenPlaceholderPattern matches a placeholder whose percent sign was
// replaced by a look-alike character or separated from its brace, e.g.
// "％{name}" (full-width), "٪{name}" (Arabic) or "% {name}".
var brokenPlaceholderPattern = regexp.MustCompile(`[％﹪٪]\{|%\s+\{`)

// TestCatalogPlaceholders checks that every translated string of every
// built-in catalog only uses the placeholders its source string provides, and
// that none of them is mangled by the translator.
func TestCatalogPlaceholders(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "app", "src", "language", "*.po"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no .po catalogs found")
	}

	for _, file := range files {
		name := filepath.Base(file)
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		catalog, err := pofile.ParseText(string(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		for _, item := range catalog.Items {
			if item.MsgId == "" || slices.Contains(item.Flags, "fuzzy") {
				continue
			}
			allowed := placeholderPattern.FindAllString(item.MsgId+" "+item.MsgIdPlural, -1)
			for _, msgstr := range item.MsgStr {
				if msgstr == "" {
					continue
				}
				if brokenPlaceholderPattern.MatchString(msgstr) {
					t.Errorf("%s: msgid %q has a malformed placeholder in %q", name, item.MsgId, msgstr)
				}
				for _, placeholder := range placeholderPattern.FindAllString(msgstr, -1) {
					if !slices.Contains(allowed, placeholder) {
						t.Errorf("%s: msgid %q: %q uses %s, which the source string does not provide (%s)",
							name, item.MsgId, msgstr, placeholder, strings.Join(allowed, ", "))
					}
				}
			}
		}
	}
}
