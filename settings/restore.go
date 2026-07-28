package settings

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"

	cosysettings "github.com/uozi-tech/cosy/settings"
	"gopkg.in/ini.v1"
)

type restoreSettingsSection struct {
	name            string
	prototype       any
	protectedFields map[string]struct{}
}

func restoreSettingsSections() []restoreSettingsSection {
	result := []restoreSettingsSection{
		{
			name:      "app",
			prototype: cosysettings.AppSettings,
			protectedFields: map[string]struct{}{
				"JwtSecret": {},
			},
		},
		{name: "server", prototype: cosysettings.ServerSettings},
		{name: "log", prototype: cosysettings.LogSettings},
		{name: "sls", prototype: cosysettings.SLSSettings},
	}

	for name, prototype := range sections.AllFromFront() {
		result = append(result, restoreSettingsSection{name: name, prototype: prototype})
	}
	return result
}

// BuildRestoreConfig parses and validates a backup configuration. Portable
// restores copy every field except destination-owned protected settings.
func BuildRestoreConfig(backupPath, currentPath string, preserveProtected bool) ([]byte, []string, error) {
	backupConfig, err := loadRestoreConfig(backupPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load backup settings: %w", err)
	}

	var skipped []string
	if preserveProtected {
		currentConfig, err := loadRestoreConfig(currentPath)
		if err != nil {
			return nil, nil, fmt.Errorf("load current settings: %w", err)
		}

		for _, section := range restoreSettingsSections() {
			fields, err := protectedINIFields(section)
			if err != nil {
				return nil, nil, err
			}
			for _, field := range fields {
				copied, err := copyINIKey(currentConfig, backupConfig, section.name, field)
				if err != nil {
					return nil, nil, err
				}
				if copied {
					skipped = append(skipped, section.name+"."+field)
				}
			}
		}
	}

	if err := validateRestoreConfig(backupConfig); err != nil {
		return nil, nil, err
	}

	var output bytes.Buffer
	if _, err := backupConfig.WriteTo(&output); err != nil {
		return nil, nil, fmt.Errorf("serialize restored settings: %w", err)
	}
	sort.Strings(skipped)
	return output.Bytes(), skipped, nil
}

func loadRestoreConfig(configPath string) (*ini.File, error) {
	return ini.LoadSources(ini.LoadOptions{
		Loose:        false,
		AllowShadows: true,
	}, configPath)
}

func protectedINIFields(section restoreSettingsSection) ([]string, error) {
	typeOfSection := reflect.TypeOf(section.prototype)
	if typeOfSection == nil || typeOfSection.Kind() != reflect.Pointer || typeOfSection.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("settings section %q has an invalid prototype", section.name)
	}

	var fields []string
	sectionType := typeOfSection.Elem()
	for i := 0; i < sectionType.NumField(); i++ {
		field := sectionType.Field(i)
		_, manuallyProtected := section.protectedFields[field.Name]
		if field.Tag.Get("protected") != "true" && !manuallyProtected {
			continue
		}

		name := iniFieldName(field)
		if name != "" {
			fields = append(fields, name)
		}
	}
	sort.Strings(fields)
	return fields, nil
}

func iniFieldName(field reflect.StructField) string {
	name := strings.Split(field.Tag.Get("ini"), ",")[0]
	if name == "-" {
		return ""
	}
	if name != "" {
		return name
	}
	return field.Name
}

func copyINIKey(source, destination *ini.File, sectionName, keyName string) (bool, error) {
	destinationHasKey := destination.HasSection(sectionName) && destination.Section(sectionName).HasKey(keyName)
	if !source.HasSection(sectionName) {
		if destinationHasKey {
			destination.Section(sectionName).DeleteKey(keyName)
		}
		return destinationHasKey, nil
	}
	sourceSection, err := source.GetSection(sectionName)
	if err != nil {
		return false, fmt.Errorf("read current settings section %q: %w", sectionName, err)
	}
	if !sourceSection.HasKey(keyName) {
		if destinationHasKey {
			destination.Section(sectionName).DeleteKey(keyName)
		}
		return destinationHasKey, nil
	}
	sourceKey, err := sourceSection.GetKey(keyName)
	if err != nil {
		return false, fmt.Errorf("read current setting %s.%s: %w", sectionName, keyName, err)
	}

	destinationSection, err := destination.NewSection(sectionName)
	if err != nil {
		return false, fmt.Errorf("create restored settings section %q: %w", sectionName, err)
	}
	destinationSection.DeleteKey(keyName)

	values := sourceKey.ValueWithShadows()
	if len(values) == 0 {
		values = []string{sourceKey.String()}
	}
	destinationKey, err := destinationSection.NewKey(keyName, values[0])
	if err != nil {
		return false, fmt.Errorf("restore protected setting %s.%s: %w", sectionName, keyName, err)
	}
	for _, value := range values[1:] {
		if err := destinationKey.AddShadow(value); err != nil {
			return false, fmt.Errorf("restore protected setting shadow %s.%s: %w", sectionName, keyName, err)
		}
	}
	return true, nil
}

func validateRestoreConfig(config *ini.File) error {
	for _, section := range restoreSettingsSections() {
		prototypeType := reflect.TypeOf(section.prototype)
		candidate := reflect.New(prototypeType.Elem()).Interface()
		if err := config.Section(section.name).StrictMapTo(candidate); err != nil {
			return fmt.Errorf("validate restored settings section %q: %w", section.name, err)
		}
	}
	return nil
}
