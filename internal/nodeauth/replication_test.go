package nodeauth

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplicatedFromRoundTrip(t *testing.T) {
	for _, value := range []string{"", "11111111-1111-4111-8111-111111111111", `a"b\c`} {
		parsed, ok := parseSFString(formatSFString(value))
		require.True(t, ok, value)
		require.Equal(t, value, parsed)
	}
}

func TestParseReplicatedFromRejectsNonSFStrings(t *testing.T) {
	for _, raw := range []string{
		"origin",    // token, not a string
		`"origin`,   // unterminated
		`"ori"gin"`, // trailing characters
		`"a\nb"`,    // invalid escape
		"\"a\tb\"",  // control character
		`"a", "b"`,  // a list, not a single string
		`?1`,        // boolean
		"\"café\"",  // non-ASCII
	} {
		header := http.Header{}
		header.Set(ReplicatedFromHeader, raw)
		_, ok := parseReplicatedFrom(header)
		require.False(t, ok, raw)
	}

	header := http.Header{}
	header.Add(ReplicatedFromHeader, `"a"`)
	header.Add(ReplicatedFromHeader, `"b"`)
	_, ok := parseReplicatedFrom(header)
	require.False(t, ok, "repeated field lines")

	header = http.Header{}
	header.Set(ReplicatedFromHeader, ` "origin" `)
	origin, ok := parseReplicatedFrom(header)
	require.True(t, ok)
	require.Equal(t, "origin", origin)
}

func TestWithPrincipalRecordsReplicatedFrom(t *testing.T) {
	request, err := http.NewRequest(http.MethodPost, "http://node.example/api/sites/a", nil)
	require.NoError(t, err)
	request.Header.Set(ReplicatedFromHeader, `"origin"`)
	original := &Principal{AuthMethod: "legacy"}

	request = WithPrincipal(request, original)

	principal, ok := PrincipalFromRequest(request)
	require.True(t, ok)
	require.True(t, principal.Replicated)
	require.Equal(t, "origin", principal.ReplicatedFrom)
	require.True(t, IsReplicated(request.Context()))
	require.False(t, original.Replicated, "the caller's principal is not mutated")
}
