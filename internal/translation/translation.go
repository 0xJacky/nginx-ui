package translation

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"slices"
	"sort"

	"github.com/0xJacky/Nginx-UI/app"
	"github.com/0xJacky/pofile"
	"github.com/samber/lo"
)

// Dict holds the flat built-in catalogs used by backend messages such as
// notifications. Entries with a msgctxt are left out because pofile.Dict
// cannot look them up.
var Dict map[string]pofile.Dict

// webDict holds the catalogs served to the web app. Entries that share a
// msgid are merged into one object keyed by msgctxt, with "" for the entry
// without a context, which is the shape vue3-gettext expects.
var webDict map[string]map[string]any

// languages lists the language codes of the built-in catalogs, sorted.
var languages []string

func init() {
	Dict = make(map[string]pofile.Dict)
	webDict = make(map[string]map[string]any)

	fs, err := app.GetDistFS()
	if err != nil {
		log.Fatalln("Failed to get DistFS:", err)
	}

	i18nJson, err := fs.Open("i18n.json")
	if err != nil {
		log.Fatalln("Failed to open i18n.json:", err)
	}

	defer i18nJson.Close()

	bytes, _ := io.ReadAll(i18nJson)

	i18nMap := make(map[string]string)

	_ = json.Unmarshal(bytes, &i18nMap)

	langCode := lo.MapToSlice(i18nMap, func(key string, value string) string {
		return key
	})

	for _, v := range langCode {
		handlePo(v)
	}
	sort.Strings(langCode)
	languages = langCode
}

// Languages lists the language codes the host translates its interface
// into, sorted.
func Languages() []string {
	return slices.Clone(languages)
}

// IsLanguage reports whether code is one of Languages.
func IsLanguage(code string) bool {
	_, found := slices.BinarySearch(languages, code)
	return found
}

func handlePo(langCode string) {
	fsys, err := app.GetDistFS()
	if err != nil {
		log.Fatalln("Failed to get DistFS:", err)
	}

	file, err := fsys.Open(fmt.Sprintf("src/language/%s.po", langCode))

	if err != nil {
		log.Fatalln(err)
	}

	defer file.Close()

	bytes, err := io.ReadAll(file)

	if err != nil {
		log.Fatalln(err)
	}

	p, err := pofile.ParseText(string(bytes))

	if err != nil {
		log.Fatalln(err)
	}

	Dict[langCode], webDict[langCode] = buildDicts(p)
}

// buildDicts splits the parsed entries into the flat dictionary used by
// backend messages and the context aware one served to the web app.
func buildDicts(p *pofile.Pofile) (flat pofile.Dict, web map[string]any) {
	flat = make(pofile.Dict)
	web = make(map[string]any)

	for _, item := range p.Items {
		if slices.Contains(item.Flags, "fuzzy") {
			continue
		}

		var value any
		if len(item.MsgStr) == 1 {
			value = item.MsgStr[0]
		} else if len(item.MsgStr) > 1 {
			value = slices.Clone(item.MsgStr)
		}

		if item.Msgctxt == "" {
			flat[item.MsgId] = value
		}

		existing, found := web[item.MsgId]
		contexts, isObject := existing.(map[string]any)
		switch {
		case item.Msgctxt == "" && !isObject:
			web[item.MsgId] = value
		case item.Msgctxt == "":
			contexts[""] = value
		case isObject:
			contexts[item.Msgctxt] = value
		default:
			contexts = map[string]any{item.Msgctxt: value}
			if found {
				contexts[""] = existing
			}
			web[item.MsgId] = contexts
		}
	}

	return
}

// GetTranslation returns the catalog for the web app: the built-in entries
// merged with the entries of every enabled plugin, where a built-in
// translation always wins. Plugin catalogs are kept apart, see plugin.go.
func GetTranslation(langCode string) map[string]any {
	return merged(langCode)
}
