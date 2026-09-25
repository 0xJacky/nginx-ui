package translation

import (
	"testing"

	"github.com/0xJacky/pofile"
	"github.com/stretchr/testify/assert"
)

const contextCatalog = `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"
"Plural-Forms: nplurals=1; plural=0;\n"

msgid "Maintenance"
msgstr "维护模式"

msgctxt "preference group"
msgid "Maintenance"
msgstr "维护"

msgctxt "action"
msgid "Save"
msgstr "保存"

msgid "Save"
msgstr "保存更改"

msgid "Plain"
msgstr "普通"

#, fuzzy
msgid "Fuzzy"
msgstr "模糊"

msgid "%{count} item"
msgid_plural "%{count} items"
msgstr[0] "%{count} 项"

msgid "%{count} file"
msgid_plural "%{count} files"
msgstr[0] "%{count} Datei"
msgstr[1] "%{count} Dateien"
`

func TestBuildDicts(t *testing.T) {
	p, err := pofile.ParseText(contextCatalog)
	assert.NoError(t, err)

	flat, web := buildDicts(p)

	// The flat catalog only carries entries without a context, whatever the
	// order they appear in, and drops fuzzy ones.
	assert.Equal(t, "维护模式", flat["Maintenance"])
	assert.Equal(t, "保存更改", flat["Save"])
	assert.Equal(t, "普通", flat["Plain"])
	assert.NotContains(t, flat, "Fuzzy")

	// A msgid with contexts becomes an object with "" for the plain entry,
	// regardless of which entry came first.
	assert.Equal(t, map[string]any{"": "维护模式", "preference group": "维护"}, web["Maintenance"])
	assert.Equal(t, map[string]any{"": "保存更改", "action": "保存"}, web["Save"])

	// Entries without a context stay plain strings.
	assert.Equal(t, "普通", web["Plain"])
	assert.NotContains(t, web, "Fuzzy")

	// A single plural form stays a string, several forms become a list.
	assert.Equal(t, "%{count} 项", web["%{count} item"])
	assert.Equal(t, "%{count} 项", flat["%{count} item"])
	assert.Equal(t, []string{"%{count} Datei", "%{count} Dateien"}, web["%{count} file"])
	assert.Equal(t, []string{"%{count} Datei", "%{count} Dateien"}, flat["%{count} file"])
}

func TestBuildDictsContextOnly(t *testing.T) {
	// Real catalogs always start with a header entry.
	p, err := pofile.ParseText("msgid \"\"\nmsgstr \"\"\n\"Content-Type: text/plain; charset=UTF-8\\n\"\n\nmsgctxt \"menu\"\nmsgid \"Open\"\nmsgstr \"打开\"\n")
	assert.NoError(t, err)

	flat, web := buildDicts(p)

	assert.NotContains(t, flat, "Open")
	assert.Equal(t, map[string]any{"menu": "打开"}, web["Open"])
}
