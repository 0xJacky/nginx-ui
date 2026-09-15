package dns

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/model"
)

// DDNS matches configured names against what the provider reports, and providers
// report an internationalized label as punycode.
func TestIndexRecordsByNameCanonicalizesIDN(t *testing.T) {
	t.Parallel()

	index := indexRecordsByName([]Record{
		{ID: "1", Type: "A", Name: "例"},
		{ID: "2", Type: "A", Name: "xn--fsq"},
		{ID: "3", Type: "A", Name: "WWW"},
	})

	// Both spellings land under one key, so a name given either way resolves.
	require.Len(t, index["xn--fsq"], 2)
	require.Len(t, index["www"], 1)
}

func TestIndexRecordsByNameAndTypeCanonicalizesIDN(t *testing.T) {
	t.Parallel()

	index := indexRecordsByNameAndType([]Record{{ID: "1", Type: "a", Name: "例"}})
	require.Equal(t, "1", index["xn--fsq"]["A"].ID)
}

func TestContainsTargetForNameMatchesAcrossSpellings(t *testing.T) {
	t.Parallel()

	targets := []model.DDNSRecordTarget{{ID: "1", Name: "xn--fsq", Type: "A"}}

	require.True(t, containsTargetForName(targets, "例", "A"))
	require.True(t, containsTargetForName(targets, "xn--fsq", "A"))
	require.True(t, containsTargetForName(targets, "例", "a"))
	require.False(t, containsTargetForName(targets, "例", "AAAA"))
	require.False(t, containsTargetForName(targets, "other", "A"))
}

func TestCollectUniqueLowercaseNamesCanonicalizesIDN(t *testing.T) {
	t.Parallel()

	names := collectUniqueLowercaseNames([]model.DDNSRecordTarget{
		{ID: "1", Name: "例", Type: "A"},
		{ID: "2", Name: "xn--fsq", Type: "AAAA"},
		{ID: "3", Name: "WWW", Type: "A"},
	})

	// The two spellings of one name must not be treated as two names.
	require.Equal(t, []string{"xn--fsq", "www"}, names)
}
