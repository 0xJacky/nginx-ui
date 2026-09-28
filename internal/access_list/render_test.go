package access_list

import (
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLists() (lan, office, scanners *model.AccessList, all []*model.AccessList) {
	lan = &model.AccessList{
		Model:    model.Model{ID: 1},
		Name:     "LAN",
		Slug:     "lan",
		Fallback: model.AccessFallbackDeny,
		Rules: []model.AccessRule{
			{Type: model.AccessRuleAllow, Value: "192.168.31.0/24"},
			{Type: model.AccessRuleAllow, Value: "10.8.0.0/24", Note: "WireGuard"},
			{Type: model.AccessRuleAllow, Value: "fd00:1234:5678::/48"},
		},
	}
	office = &model.AccessList{
		Model:    model.Model{ID: 2},
		Name:     "LAN + office",
		Slug:     "lan-office",
		Fallback: model.AccessFallbackDeny,
		Rules: []model.AccessRule{
			{Type: model.AccessRuleDeny, Value: "192.168.31.66", Note: "guest"},
			{Type: model.AccessRuleRef, RefID: 1},
			{Type: model.AccessRuleAllow, Value: "203.0.113.10"},
		},
	}
	scanners = &model.AccessList{
		Model:    model.Model{ID: 3},
		Name:     "Scanners",
		Slug:     "scanners",
		Fallback: model.AccessFallbackAllow,
		Rules: []model.AccessRule{
			{Type: model.AccessRuleDeny, Value: "198.51.100.0/24"},
		},
	}
	return lan, office, scanners, []*model.AccessList{lan, office, scanners}
}

func TestRenderExpandsReferencesInPlace(t *testing.T) {
	_, office, _, all := testLists()

	content, err := Render(office, all)
	require.NoError(t, err)

	want := `# Managed by Nginx UI: access list "LAN + office" (lan-office).
# Changes made to this file are overwritten when the list is saved in Nginx UI.
deny 192.168.31.66; # guest
# >>> LAN (lan)
allow 192.168.31.0/24;
allow 10.8.0.0/24; # WireGuard
allow fd00:1234:5678::/48;
# <<< LAN (lan)
allow 203.0.113.10;
deny all;
`
	assert.Equal(t, want, content)
}

func TestRenderDropsReferencedFallback(t *testing.T) {
	_, _, scanners, all := testLists()
	// scanners ends with "allow all"; referencing it must not end the rule set.
	outer := &model.AccessList{
		Model:    model.Model{ID: 9},
		Name:     "Outer",
		Slug:     "outer",
		Fallback: model.AccessFallbackDeny,
		Rules: []model.AccessRule{
			{Type: model.AccessRuleRef, RefID: scanners.ID},
			{Type: model.AccessRuleAllow, Value: "10.0.0.0/8"},
		},
	}

	content, err := Render(outer, append(all, outer))
	require.NoError(t, err)
	assert.NotContains(t, content, "allow all;")
	assert.True(t, strings.HasSuffix(content, "allow 10.0.0.0/8;\ndeny all;\n"))
}

func TestRenderNestedReferences(t *testing.T) {
	_, office, _, all := testLists()
	top := &model.AccessList{
		Model:    model.Model{ID: 10},
		Name:     "Top",
		Slug:     "top",
		Fallback: model.AccessFallbackDeny,
		Rules:    []model.AccessRule{{Type: model.AccessRuleRef, RefID: office.ID}},
	}

	content, err := Render(top, append(all, top))
	require.NoError(t, err)
	assert.Contains(t, content, "# >>> LAN + office (lan-office)\ndeny 192.168.31.66; # guest\n# >>> LAN (lan)\n")
	assert.Equal(t, 1, strings.Count(content, "deny all;"))
}

func TestRenderDetectsCycle(t *testing.T) {
	lan, _, _, all := testLists()
	// LAN now references office, which already references LAN.
	changed := *lan
	changed.Rules = append(append([]model.AccessRule{}, lan.Rules...), model.AccessRule{Type: model.AccessRuleRef, RefID: 2})

	_, err := Render(&changed, all)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LAN → LAN + office → LAN")
}

func TestRenderSanitizesNotesAndNames(t *testing.T) {
	list := &model.AccessList{
		Name:     "Bad\nname",
		Slug:     "bad",
		Fallback: model.AccessFallbackDeny,
		Rules:    []model.AccessRule{{Type: model.AccessRuleAllow, Value: "::1", Note: "a\nallow all;"}},
	}
	content, err := Render(list, nil)
	require.NoError(t, err)
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		assert.False(t, strings.HasPrefix(line, "allow all"), "note leaked a directive: %q", line)
	}
}

func TestDependents(t *testing.T) {
	_, office, _, all := testLists()
	top := &model.AccessList{Model: model.Model{ID: 10}, Rules: []model.AccessRule{{Type: model.AccessRuleRef, RefID: office.ID}}}
	all = append(all, top)

	ids := func(lists []*model.AccessList) []uint64 {
		var result []uint64
		for _, l := range lists {
			result = append(result, l.ID)
		}
		return result
	}
	assert.ElementsMatch(t, []uint64{2, 10}, ids(Dependents(1, all)))
	assert.ElementsMatch(t, []uint64{2}, ids(DirectDependents(1, all)))
	assert.Empty(t, Dependents(3, all))
}
