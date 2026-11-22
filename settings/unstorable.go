package settings

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"github.com/uozi-tech/cosy/settings"
	"gopkg.in/ini.v1"
)

// A value app.ini cannot store is refused when the change being saved set
// it. It can also reach the settings without such a change:
//
//   - An environment variable overrides a stored value. The override stays in
//     effect, app.ini keeps what it stores for that setting, and the rest of
//     the change is saved.
//   - app.ini itself stores a value it cannot write back, such as a quoted
//     value ending in a backslash. Nothing can be saved until the operator
//     corrects it, so every change is refused and rolled back.
//
// The helpers below find, keep and describe such values.

func unstorableValueError(key string) error {
	return cosy.WrapErrorWithParams(ErrUnstorableValue, key)
}

// splitSettingKey splits section.Key at the last dot: key names never hold
// one, a hand-edited section name may.
func splitSettingKey(key string) (section, name string) {
	i := strings.LastIndex(key, ".")
	if i < 0 {
		return "", key
	}
	return key[:i], key[i+1:]
}

type settingsTarget struct {
	name string
	ptr  any
}

// settingsTargets lists every settings struct written to app.ini, by section.
func settingsTargets() []settingsTarget {
	targets := []settingsTarget{
		{"app", settings.AppSettings},
		{"server", settings.ServerSettings},
		{"log", settings.LogSettings},
		{"sls", settings.SLSSettings},
	}
	for name, ptr := range sections.AllFromFront() {
		targets = append(targets, settingsTarget{name, ptr})
	}
	return targets
}

// settingsSnapshot holds copies of every settings struct taken before a change.
type settingsSnapshot struct {
	targets []settingsTarget
	copies  []reflect.Value
}

func snapshotSettings() *settingsSnapshot {
	snapshot := &settingsSnapshot{targets: settingsTargets()}
	for _, target := range snapshot.targets {
		snapshot.copies = append(snapshot.copies, deepCopy(reflect.ValueOf(target.ptr).Elem()))
	}
	return snapshot
}

// restore puts the copies back into the live settings.
func (s *settingsSnapshot) restore() {
	for i, target := range s.targets {
		reflect.ValueOf(target.ptr).Elem().Set(deepCopy(s.copies[i]))
	}
}

// document returns the ini document the settings had before the change.
func (s *settingsSnapshot) document() *ini.File {
	doc := ini.Empty(confLoadOptions)
	for i, target := range s.targets {
		_ = doc.Section(target.name).ReflectFrom(s.copies[i].Addr().Interface())
	}
	return doc
}

// deepCopy copies a value, including the slices, maps and arrays reachable
// through its fields, so edits of their elements do not reach the copy.
// Settings structs hold no pointers or interfaces, which would stay shared;
// TestSettingsStructsHoldNoSharedReferences keeps it that way.
func deepCopy(value reflect.Value) reflect.Value {
	copied := reflect.New(value.Type()).Elem()
	copied.Set(value)
	cloneReferences(copied)
	return copied
}

func cloneReferences(value reflect.Value) {
	switch value.Kind() {
	case reflect.Slice:
		if !value.IsNil() {
			cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
			reflect.Copy(cloned, value)
			for i := 0; i < cloned.Len(); i++ {
				cloneReferences(cloned.Index(i))
			}
			value.Set(cloned)
		}
	case reflect.Map:
		if !value.IsNil() {
			cloned := reflect.MakeMapWithSize(value.Type(), value.Len())
			iter := value.MapRange()
			for iter.Next() {
				cloned.SetMapIndex(iter.Key(), iter.Value())
			}
			value.Set(cloned)
		}
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			cloneReferences(value.Index(i))
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if field := value.Field(i); field.CanSet() {
				cloneReferences(field)
			}
		}
	}
}

// changedSince reports whether the change being saved set key, which conf
// holds as reflected from the live settings. Without a previous document
// nothing was changed.
func changedSince(previous, conf *ini.File, key string) bool {
	if previous == nil {
		return false
	}
	section, name := splitSettingKey(key)
	_, field, ok := settingsField(section, name)
	if !ok {
		// Keys no setting maps to are only ever copied from app.ini.
		return false
	}
	if !previous.Section(section).HasKey(name) {
		return true
	}
	before, current := previous.Section(section).Key(name), conf.Section(section).Key(name)
	if before.Value() != current.Value() {
		return true
	}
	// A list stored as repeated keys is reflected whole. A single value field
	// keeps any further copies app.ini has of its key, which are not the
	// change's.
	if strings.Contains(field.Tag.Get("ini"), "allowshadow") {
		return !slices.Equal(before.ValueWithShadows(), current.ValueWithShadows())
	}
	return false
}

// keepStoredValues makes conf carry what app.ini stores for keys, so the rest
// of the document can be saved, and logs each key. The returned function puts
// the live values of those settings back once the saved file has been mapped
// onto the settings again. When app.ini cannot store its own value either,
// encodeConf refuses the document afterwards.
func keepStoredValues(conf *ini.File, keys []string) (restoreLive func()) {
	stored, err := ini.LoadSources(confLoadOptions, settings.ConfPath)
	var restores []func()
	for _, key := range keys {
		section, name := splitSettingKey(key)
		if err == nil && storedValueUnstorable(stored, section, name) {
			logger.Error(describeStoredUnstorable(stored, key))
		} else {
			logger.Error(describeUnstorable(conf, key))
		}
		restores = append(restores, keepLiveField(section, name))
		if err != nil {
			continue
		}
		if _, copyErr := copyINIKey(stored, conf, section, name); copyErr != nil {
			logger.Error(copyErr)
		}
	}
	return func() {
		for _, restore := range restores {
			restore()
		}
	}
}

func storedValueUnstorable(stored *ini.File, section, name string) bool {
	storedSection, err := stored.GetSection(section)
	if err != nil || !storedSection.HasKey(name) {
		return false
	}
	return !keyReadsBack(section, storedSection.Key(name))
}

// keepLiveField returns a function that sets the field behind section.name
// back to its current value.
func keepLiveField(section, name string) func() {
	field, _, ok := settingsField(section, name)
	if !ok {
		return func() {}
	}
	saved := deepCopy(field)
	return func() { field.Set(deepCopy(saved)) }
}

func settingsField(section, name string) (reflect.Value, reflect.StructField, bool) {
	for _, target := range settingsTargets() {
		if target.name != section {
			continue
		}
		value := reflect.ValueOf(target.ptr).Elem()
		for i := 0; i < value.NumField(); i++ {
			if iniFieldName(value.Type().Field(i)) == name {
				return value.Field(i), value.Type().Field(i), true
			}
		}
	}
	return reflect.Value{}, reflect.StructField{}, false
}

// reportUnstorableSettings logs every value that cannot be stored, so an
// operator sees it at startup rather than on the first failing save: first
// those app.ini stores itself, then those only the live settings hold.
func reportUnstorableSettings() {
	reported := map[string]bool{}
	if settings.Conf != nil {
		for _, key := range unstorableKeys(settings.Conf) {
			logger.Error(describeStoredUnstorable(settings.Conf, key))
			reported[key] = true
		}
	}

	doc := ini.Empty(confLoadOptions)
	for _, target := range settingsTargets() {
		_ = doc.Section(target.name).ReflectFrom(target.ptr)
	}
	for _, key := range unstorableKeys(doc) {
		if !reported[key] {
			logger.Error(describeUnstorable(doc, key))
		}
	}
}

// describeUnstorable explains a live value app.ini cannot store while it
// keeps its stored value for the setting.
func describeUnstorable(conf *ini.File, key string) string {
	section, name := splitSettingKey(key)
	origin := ""
	if vars := settingEnvVars(section, name); len(vars) > 0 {
		origin = " (set by environment variable " + strings.Join(vars, ", ") + ")"
	}
	return fmt.Sprintf("Setting %s%s cannot be stored in app.ini: %s. "+
		"It stays in effect, but app.ini keeps its stored value for it and changes to it cannot be saved until the value is corrected.",
		key, origin, unstorableHint(conf.Section(section).Key(name)))
}

// describeStoredUnstorable explains a value app.ini stores but cannot write
// back, which blocks every save until it is corrected.
func describeStoredUnstorable(conf *ini.File, key string) string {
	section, name := splitSettingKey(key)
	return fmt.Sprintf("app.ini (%s) stores a value for %s that cannot be written back: %s. "+
		"No settings change can be saved until it is corrected in app.ini.",
		settings.ConfPath, key, unstorableHint(conf.Section(section).Key(name)))
}

// UnstorableStoredValueMessage explains err, an ErrUnstorableValue for a
// value app.ini stores itself, to an operator who has to correct it before
// Nginx UI can start.
func UnstorableStoredValueMessage(err error) string {
	key := "app.ini"
	var cErr *cosy.Error
	if errors.As(err, &cErr) && len(cErr.Params) > 0 {
		key = cErr.Params[0]
	}

	hint := "app.ini cannot represent it exactly"
	section, name := splitSettingKey(key)
	if settings.Conf != nil {
		if storedSection, sectionErr := settings.Conf.GetSection(section); sectionErr == nil && storedSection.HasKey(name) {
			hint = unstorableHint(storedSection.Key(name))
		}
	}
	return fmt.Sprintf("app.ini (%s) stores a value for %s that cannot be written back: %s. "+
		"Correct it in app.ini before Nginx UI can start.", settings.ConfPath, key, hint)
}

// migrationUnstorableMessage names the migrated values app.ini cannot store.
func migrationUnstorableMessage(migrated *ini.File) string {
	var reasons []string
	for _, key := range unstorableKeys(migrated) {
		section, name := splitSettingKey(key)
		reasons = append(reasons, key+": "+unstorableHint(migrated.Section(section).Key(name)))
	}
	return strings.Join(reasons, "; ") + ". Correct it in app.ini before Nginx UI can start."
}

func unstorableHint(key *ini.Key) string {
	values := append([]string{key.Value()}, key.ValueWithShadows()...)
	switch {
	case slices.ContainsFunc(values, iniReferencePattern.MatchString):
		return "it contains a %(name)s reference, which app.ini replaces with another setting"
	case slices.ContainsFunc(values, func(value string) bool {
		return strings.HasSuffix(strings.TrimRightFunc(value, unicode.IsSpace), `\`)
	}):
		return "it ends with a backslash, which app.ini reads as a line continuation"
	case slices.ContainsFunc(values, func(value string) bool { return strings.Contains(value, `"""`) }):
		return `it contains triple quotes (""") together with a line break`
	default:
		return "app.ini cannot represent it exactly"
	}
}

// settingEnvVars returns the environment variables that are set and override
// section.name: Nginx UI's prefixed ones and cosy's unprefixed ones.
func settingEnvVars(section, name string) []string {
	_, field, ok := settingsField(section, name)
	if !ok {
		return nil
	}
	envName := strings.Split(field.Tag.Get("env"), ",")[0]
	if envName == "" {
		envName = toEnvName(field.Name)
	}

	candidates := []string{strings.ToUpper(section) + "_" + envName}
	for _, target := range settingsTargets() {
		if target.name != section {
			continue
		}
		for prefix, ptr := range envPrefixMap {
			if ptr == target.ptr {
				candidates = append(candidates, EnvPrefix+prefix+"_"+envName)
			}
		}
	}

	var set []string
	for _, candidate := range candidates {
		if _, ok := os.LookupEnv(candidate); ok {
			set = append(set, candidate)
		}
	}
	slices.Sort(set)
	return slices.Compact(set)
}

// toEnvName mirrors the field name conversion of github.com/caarlos0/env,
// which the settings are parsed from the environment with.
func toEnvName(input string) string {
	var output []rune
	for i, c := range input {
		if c == '_' {
			continue
		}
		if len(output) > 0 && unicode.IsUpper(c) && len(input) > i+1 {
			peek := rune(input[i+1])
			if unicode.IsLower(peek) || unicode.IsLower(rune(input[i-1])) {
				output = append(output, '_')
			}
		}
		output = append(output, unicode.ToUpper(c))
	}
	return string(output)
}
