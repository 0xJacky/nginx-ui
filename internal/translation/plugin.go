package translation

import (
	"bufio"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/0xJacky/pofile"
)

// Catalog holds the entries of one plugin, keyed by language code.
type Catalog map[string]pofile.Dict

var (
	pluginMutex sync.RWMutex
	// pluginCatalogs holds the catalogs of the enabled plugins by plugin id.
	pluginCatalogs = map[string]Catalog{}
	// mergedCache holds the merged catalog of each language until the plugin
	// catalogs change.
	mergedCache = map[string]map[string]any{}
	// generation changes with every SetPluginCatalogs, so a merge computed
	// from older catalogs is never cached.
	generation uint64
)

// SetPluginCatalogs replaces the catalogs of every plugin. A plugin missing
// from catalogs has its entries removed.
func SetPluginCatalogs(catalogs map[string]Catalog) {
	next := make(map[string]Catalog, len(catalogs))
	for pluginID, catalog := range catalogs {
		if len(catalog) > 0 {
			next[pluginID] = catalog
		}
	}

	pluginMutex.Lock()
	defer pluginMutex.Unlock()
	pluginCatalogs = next
	mergedCache = map[string]map[string]any{}
	generation++
}

// PluginIDs lists the plugins whose entries are merged right now.
func PluginIDs() []string {
	pluginMutex.RLock()
	defer pluginMutex.RUnlock()
	ids := make([]string, 0, len(pluginCatalogs))
	for pluginID := range pluginCatalogs {
		ids = append(ids, pluginID)
	}
	sort.Strings(ids)
	return ids
}

// merged returns the web catalog of a language. Without plugin entries it is
// the built-in catalog itself.
func merged(langCode string) map[string]any {
	pluginMutex.RLock()
	if dict, ok := mergedCache[langCode]; ok {
		pluginMutex.RUnlock()
		return dict
	}
	if len(pluginCatalogs) == 0 {
		pluginMutex.RUnlock()
		return webDict[langCode]
	}
	ids := make([]string, 0, len(pluginCatalogs))
	for pluginID := range pluginCatalogs {
		ids = append(ids, pluginID)
	}
	sort.Strings(ids)
	computedFor := generation
	layers := make([]pofile.Dict, 0, len(ids))
	for _, pluginID := range ids {
		if dict := pluginCatalogs[pluginID][langCode]; len(dict) > 0 {
			layers = append(layers, dict)
		}
	}
	pluginMutex.RUnlock()

	builtin := webDict[langCode]
	if len(layers) == 0 {
		return builtin
	}

	result := make(map[string]any, len(builtin))
	for key, value := range builtin {
		result[key] = value
	}
	// The lowest plugin id comes first and keeps a key a later plugin
	// declares too. A key the host translates is never replaced.
	for _, layer := range layers {
		for key, value := range layer {
			if translated(result[key]) {
				continue
			}
			result[key] = value
		}
	}

	pluginMutex.Lock()
	if generation == computedFor {
		mergedCache[langCode] = result
	}
	pluginMutex.Unlock()
	return result
}

// translated reports whether a catalog value carries a translation. An empty
// msgstr marks an entry nobody translated yet.
func translated(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return v != ""
	case []string:
		for _, form := range v {
			if form != "" {
				return true
			}
		}
		return false
	default:
		return true
	}
}

// ParsePO parses a gettext catalog the way the built-in catalogs are read,
// and rejects what the parser would silently misread: invalid UTF-8, a
// missing header entry and an entry without a msgstr.
func ParsePO(data []byte) (dict pofile.Dict, err error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("the file is not valid UTF-8")
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if err = checkPOHeader(text); err != nil {
		return nil, err
	}

	// The parser indexes its input without checking it, so a malformed file
	// must not take the host down.
	defer func() {
		if recovered := recover(); recovered != nil {
			dict, err = nil, fmt.Errorf("malformed catalog: %v", recovered)
		}
	}()
	parsed, err := pofile.ParseText(text)
	if err != nil {
		return nil, err
	}
	for _, item := range parsed.Items {
		if len(item.MsgStr) == 0 {
			return nil, fmt.Errorf("msgid %q has no msgstr", item.MsgId)
		}
	}
	return parsed.ToDict(), nil
}

// checkPOHeader requires the first entry to be the header: an empty msgid
// followed by its msgstr.
func checkPOHeader(text string) error {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), len(text)+1)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "msgctxt") {
			continue
		}
		if line != `msgid ""` {
			return fmt.Errorf(`the first entry must be the header (msgid "")`)
		}
		for scanner.Scan() {
			next := strings.TrimSpace(scanner.Text())
			if next == "" {
				continue
			}
			if !strings.HasPrefix(next, "msgstr") {
				return fmt.Errorf(`the first entry must be the header (msgid "")`)
			}
			return nil
		}
		return fmt.Errorf("the header entry has no msgstr")
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("the file has no entry")
}
