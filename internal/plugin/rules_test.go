package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every rule the linter and the conformance runner report is explained in the
// plugin development guide of every language, under a heading of its name.
func TestEveryRuleIsDocumented(t *testing.T) {
	for _, page := range []string{"plugin/rules.md", "zh_CN/plugin/rules.md", "zh_TW/plugin/rules.md"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "docs", filepath.FromSlash(page)))
		require.NoError(t, err)

		headings := map[string]bool{}
		for _, line := range strings.Split(string(raw), "\n") {
			if name, ok := strings.CutPrefix(line, "### "); ok {
				headings[strings.TrimSpace(name)] = true
			}
		}
		for _, rule := range AllRules {
			assert.True(t, headings[rule], "%s has no section for %s", page, rule)
		}
		assert.Len(t, headings, len(AllRules), "%s documents rules the code does not have", page)
	}
}
