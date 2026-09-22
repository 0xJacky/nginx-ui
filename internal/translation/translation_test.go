package translation

import (
	"slices"
	"testing"

	"github.com/0xJacky/pofile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestLanguagesFollowTheBuiltinCatalogs(t *testing.T) {
	languages := Languages()
	require.NotEmpty(t, languages)
	assert.True(t, slices.IsSorted(languages))
	for _, code := range []string{"en", "zh_CN", "de_DE", "ja_JP"} {
		assert.True(t, IsLanguage(code), code)
	}
	for _, code := range []string{"", "xx_XX", "zh-CN", "../en"} {
		assert.False(t, IsLanguage(code), code)
	}
}

func TestParsePO(t *testing.T) {
	dict, err := ParsePO([]byte("msgid \"\"\nmsgstr \"\"\n\"Language: de_DE\\n\"\n\nmsgid \"Save\"\nmsgstr \"Speichern\"\n\n" +
		"msgid \"one file\"\nmsgid_plural \"%d files\"\nmsgstr[0] \"eine Datei\"\nmsgstr[1] \"%d Dateien\"\n"))
	require.NoError(t, err)
	assert.Equal(t, "Speichern", dict["Save"])
	assert.Equal(t, []string{"eine Datei", "%d Dateien"}, dict["one file"])

	for name, body := range map[string]string{
		"empty":          "",
		"garbage":        "not a catalog",
		"no header":      "msgid \"Save\"\nmsgstr \"Speichern\"\n",
		"header only id": "msgid \"\"\n",
		"no msgstr":      "msgid \"\"\nmsgstr \"\"\n\nmsgid \"Save\"\n",
		"invalid utf-8":  "msgid \"\"\nmsgstr \"\"\n\nmsgid \"\xff\"\nmsgstr \"x\"\n",
	} {
		_, err := ParsePO([]byte(body))
		assert.Error(t, err, name)
	}
}

// builtinEntry returns a msgid the built-in catalog of a language
// translates.
func builtinEntry(t *testing.T, lang string) (string, string) {
	t.Helper()
	for key, value := range webDict[lang] {
		if text, ok := value.(string); ok && text != "" && key != "" {
			return key, text
		}
	}
	t.Skipf("the %s catalog has no translated entry", lang)
	return "", ""
}

func TestPluginCatalogsNeverOverrideTheHost(t *testing.T) {
	t.Cleanup(func() { SetPluginCatalogs(nil) })
	key, builtin := builtinEntry(t, "zh_CN")

	SetPluginCatalogs(map[string]Catalog{
		"io.github.b.plugin": {"zh_CN": pofile.Dict{key: "plugin b", "Plugin only": "B", "Shared": "from b"}},
		"io.github.a.plugin": {"zh_CN": pofile.Dict{"Shared": "from a"}, "ja_JP": pofile.Dict{"Plugin only": "A"}},
	})
	merged := GetTranslation("zh_CN")
	assert.Equal(t, builtin, merged[key], "a built-in translation wins")
	assert.Equal(t, "B", merged["Plugin only"])
	assert.Equal(t, "from a", merged["Shared"], "the lowest plugin id wins")
	assert.Equal(t, "A", GetTranslation("ja_JP")["Plugin only"])
	assert.NotContains(t, Dict["zh_CN"], "Plugin only", "the built-in catalog is left alone")
	assert.Equal(t, []string{"io.github.a.plugin", "io.github.b.plugin"}, PluginIDs())

	// A second read is served from the cache and equals the first.
	assert.Equal(t, merged["Shared"], GetTranslation("zh_CN")["Shared"])

	SetPluginCatalogs(nil)
	assert.NotContains(t, GetTranslation("zh_CN"), "Plugin only")
	assert.Empty(t, PluginIDs())
}

func TestPluginCatalogsFillUntranslatedEntries(t *testing.T) {
	t.Cleanup(func() { SetPluginCatalogs(nil) })
	var lang, key string
	for _, code := range Languages() {
		for k, value := range webDict[code] {
			if text, ok := value.(string); ok && text == "" && k != "" {
				lang, key = code, k
				break
			}
		}
		if key != "" {
			break
		}
	}
	if key == "" {
		t.Skip("every built-in entry is translated")
	}

	SetPluginCatalogs(map[string]Catalog{"io.github.a.plugin": {lang: pofile.Dict{key: "filled"}}})
	assert.Equal(t, "filled", GetTranslation(lang)[key])
}
