package access_list

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStatementsOffsets(t *testing.T) {
	src := "server {\n  listen 80; # inline\n  location = \"/a b\" {\n    return 200 \"}\";\n  }\n}\n"
	roots, err := parseStatements(src)
	require.NoError(t, err)
	require.Len(t, roots, 1)

	server := roots[0]
	assert.Equal(t, "server", server.Name)
	assert.Equal(t, "}", src[server.BodyEnd:server.BodyEnd+1])
	require.Len(t, server.Children, 2)
	assert.Equal(t, "listen 80;", src[server.Children[0].Start:server.Children[0].End])

	location := server.Children[1]
	assert.Equal(t, []string{"=", "/a b"}, location.Args)
	assert.Equal(t, "return 200 \"}\";", src[location.Children[0].Start:location.Children[0].End])
}

func TestParseStatementsComments(t *testing.T) {
	src := "# about a\na 1;\n\n# detached\n\nb 2; # inline\nc 3;\n"
	roots, err := parseStatements(src)
	require.NoError(t, err)
	require.Len(t, roots, 3)
	require.Len(t, roots[0].Comments, 1)
	assert.Equal(t, "# about a", roots[0].Comments[0].Text)
	assert.Empty(t, roots[1].Comments, "a blank line detaches a comment")
	assert.Empty(t, roots[2].Comments, "an inline comment belongs to the line above")
}

func TestParseStatementsSpecialBlocks(t *testing.T) {
	src := "server {\n  set $x ${host}a;\n  content_by_lua_block {\n    local t = { a = \"{\" }\n  }\n  listen 80;\n}\n"
	roots, err := parseStatements(src)
	require.NoError(t, err)
	names := []string{}
	for _, st := range roots[0].Children {
		names = append(names, st.Name)
	}
	assert.Equal(t, []string{"set", "content_by_lua_block", "listen"}, names)
}

func TestParseStatementsErrors(t *testing.T) {
	for _, src := range []string{"server {", "server }", "listen 80", "a \"unterminated;", "}"} {
		_, err := parseStatements(src)
		assert.Error(t, err, src)
	}
}

func TestServerBlocksInsideHTTP(t *testing.T) {
	roots, err := parseStatements("http {\n server { listen 80; }\n server { listen 81; }\n}\n")
	require.NoError(t, err)
	assert.Len(t, serverBlocks(roots), 2)
}

func TestRemoveSpansKeepsSurroundingText(t *testing.T) {
	src := "a;\n    b;\nc; d;\n"
	assert.Equal(t, "a;\nc; d;\n", removeSpans(src, []span{{start: 7, end: 9}}))
	assert.Equal(t, "a;\n    b;\nc;\n", removeSpans(src, []span{{start: 13, end: 15}}))
}

func TestInsertLinesSingleLineBlock(t *testing.T) {
	out, err := Apply("server { listen 80; }\n", []Change{{Server: 0, Mode: ModeList, Slug: "lan"}})
	require.NoError(t, err)
	states, err := State(out)
	require.NoError(t, err)
	assert.Equal(t, "lan", states[0].Slug)
}
