package settings

import (
	"bytes"
	"regexp"
	"slices"

	"gopkg.in/ini.v1"
)

// confLoadOptions are the options cosy loads app.ini with.
var confLoadOptions = ini.LoadOptions{
	Loose:        true,
	AllowShadows: true,
}

// iniReferencePattern matches the %(name)s references ini.v1 expands in values.
var iniReferencePattern = regexp.MustCompile(`%\([^)]+\)s`)

// encodeConf serializes conf for app.ini and refuses a document that would not
// read back exactly as it is.
//
// ini.v1 cannot store every string. A value with a line break is wrapped in
// triple quotes but has no escape for a triple quote inside it, so the value
// ends early and the rest of it is read as further keys and sections. A
// trailing backslash continues the value onto the next line and swallows the
// key written there. The loader keeps the first copy of a key, so either way
// one setting could add, replace or remove others, protected ones included.
// Every writer of app.ini goes through here, so the check covers them all.
func encodeConf(conf *ini.File) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := conf.WriteTo(&buf); err != nil {
		return nil, err
	}
	if !readsBackAs(conf, buf.Bytes()) {
		key := "app.ini"
		if keys := unstorableKeys(conf); len(keys) > 0 {
			key = keys[0]
		}
		return nil, unstorableValueError(key)
	}
	return buf.Bytes(), nil
}

// readsBackAs reports whether encoded parses into the same sections, keys and
// values, shadows included and in the same order, as conf, and whether the
// settings mapped from it see those values unchanged.
func readsBackAs(conf *ini.File, encoded []byte) bool {
	reloaded, err := ini.LoadSources(confLoadOptions, encoded)
	if err != nil {
		return false
	}

	want, got := conf.Sections(), reloaded.Sections()
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if want[i].Name() != got[i].Name() || !slices.Equal(want[i].KeyStrings(), got[i].KeyStrings()) {
			return false
		}
		for _, name := range want[i].KeyStrings() {
			wantKey, gotKey := want[i].Key(name), got[i].Key(name)
			if wantKey.Value() != gotKey.Value() ||
				!slices.Equal(wantKey.ValueWithShadows(), gotKey.ValueWithShadows()) ||
				hasReference(wantKey) {
				return false
			}
		}
	}
	return true
}

// hasReference reports whether a value of key contains a %(name)s reference.
// MapTo reads values through ini.v1's interpolation, which replaces such a
// reference with the value of another key of the same section or of the
// DEFAULT section. The setting would then not hold what was stored, could
// read another setting, and its meaning would depend on the rest of the file,
// so the reference itself is refused instead of comparing expanded values.
func hasReference(key *ini.Key) bool {
	if iniReferencePattern.MatchString(key.Value()) {
		return true
	}
	return slices.ContainsFunc(key.ValueWithShadows(), iniReferencePattern.MatchString)
}

// unstorableKeys names, as section.Key, every key whose values do not read
// back on their own.
func unstorableKeys(conf *ini.File) []string {
	var keys []string
	for _, section := range conf.Sections() {
		for _, key := range section.Keys() {
			if !keyReadsBack(section.Name(), key) {
				keys = append(keys, section.Name()+"."+key.Name())
			}
		}
	}
	return keys
}

func keyReadsBack(section string, key *ini.Key) bool {
	single := ini.Empty(confLoadOptions)
	copied, err := single.Section(section).NewKey(key.Name(), key.Value())
	if err != nil {
		return false
	}
	// AddShadow skips the copy of the value the key already holds.
	for _, value := range key.ValueWithShadows() {
		_ = copied.AddShadow(value)
	}

	var buf bytes.Buffer
	if _, err := single.WriteTo(&buf); err != nil {
		return false
	}
	return readsBackAs(single, buf.Bytes())
}
