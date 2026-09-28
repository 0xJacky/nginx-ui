package access_list

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidAddress(t *testing.T) {
	valid := []string{"all", "unix:", "192.168.1.1", "192.168.1.0/24", "0.0.0.0/0", "::1", "fd00::/8", "2001:db8::1/128"}
	for _, v := range valid {
		assert.True(t, ValidAddress(v), v)
	}
	invalid := []string{"", "localhost", "192.168.1.256", "192.168.1.0/33", "fd00::/129", "10.0.0.0/8;", "all;", "1.2.3.4 5.6.7.8", "unix:/tmp/sock"}
	for _, v := range invalid {
		assert.False(t, ValidAddress(v), v)
	}
}

func TestValidSlug(t *testing.T) {
	for _, s := range []string{"lan", "lan-office", "0", "a-b-c-1"} {
		assert.True(t, ValidSlug(s), s)
	}
	for _, s := range []string{"", "-lan", "LAN", "lan_office", "lan/office", "../x", "lan.conf"} {
		assert.False(t, ValidSlug(s), s)
	}
}

func TestValidate(t *testing.T) {
	lan, office, _, all := testLists()

	require.NoError(t, Validate(office, withoutID(all, office.ID)))

	cases := map[string]struct {
		mutate func(l *model.AccessList)
		err    error
	}{
		"empty name":      {func(l *model.AccessList) { l.Name = " " }, ErrInvalidName},
		"multiline name":  {func(l *model.AccessList) { l.Name = "a\nb" }, ErrInvalidName},
		"bad slug":        {func(l *model.AccessList) { l.Slug = "Bad Slug" }, ErrInvalidSlug},
		"taken slug":      {func(l *model.AccessList) { l.Slug = lan.Slug }, ErrSlugExists},
		"bad fallback":    {func(l *model.AccessList) { l.Fallback = "maybe" }, ErrInvalidFallback},
		"bad address":     {func(l *model.AccessList) { l.Rules[0].Value = "example.com" }, ErrInvalidAddress},
		"bad type":        {func(l *model.AccessList) { l.Rules[0].Type = "permit" }, ErrInvalidRuleType},
		"missing ref":     {func(l *model.AccessList) { l.Rules[1].RefID = 99 }, ErrReferenceNotFound},
		"self ref":        {func(l *model.AccessList) { l.Rules[1].RefID = office.ID }, ErrReferenceSelf},
		"multiline notes": {func(l *model.AccessList) { l.Rules[0].Note = "a\nb" }, ErrNoteMultiline},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			changed := *office
			changed.Rules = append([]model.AccessRule{}, office.Rules...)
			tc.mutate(&changed)
			err := Validate(&changed, withoutID(all, office.ID))
			require.Error(t, err)
			assertErrCode(t, err, tc.err)
		})
	}
}

func TestWarnings(t *testing.T) {
	list := &model.AccessList{
		Fallback: model.AccessFallbackAllow,
		Rules: []model.AccessRule{
			{Type: model.AccessRuleAllow, Value: "192.168.1.1/24"},
			{Type: model.AccessRuleAllow, Value: "all"},
			{Type: model.AccessRuleAllow, Value: "10.0.0.0/8"},
		},
	}
	assert.ElementsMatch(t, []Warning{
		{Code: WarningHostBits, Rule: 1},
		{Code: WarningShadowed, Rule: 2},
		{Code: WarningAllowsAll},
	}, Warnings(list))

	assert.Equal(t, []Warning{{Code: WarningEmptyRules}}, Warnings(&model.AccessList{Fallback: model.AccessFallbackDeny}))

	_, office, _, _ := testLists()
	assert.Empty(t, Warnings(office))
}
