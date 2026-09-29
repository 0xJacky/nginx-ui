package dns

// Field groups of a FormField.
const (
	FieldGroupCredential = "credential"
	FieldGroupSetting    = "setting"
)

// FieldUnitSeconds marks a field whose value is a number of seconds.
const FieldUnitSeconds = "seconds"

// Form is the structured credential form of one provider. Labels, help texts
// and method names are English msgids the frontend translates.
type Form struct {
	Fields  []FormField  `json:"fields,omitempty"`
	Methods []FormMethod `json:"methods,omitempty"`
}

// FormField is one input of the credential form.
type FormField struct {
	Key      string `json:"key"`
	Label    string `json:"label,omitempty"`
	Help     string `json:"help,omitempty"`
	Group    string `json:"group,omitempty"`
	Optional bool   `json:"optional,omitempty"`
	Secret   bool   `json:"secret,omitempty"`
	Default  string `json:"default,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Link     string `json:"link,omitempty"`
}

// FormMethod is one way to sign in and the credential fields it uses. Values
// are fixed credentials stored when the method is selected.
type FormMethod struct {
	Name        string            `json:"name"`
	Recommended bool              `json:"recommended,omitempty"`
	Fields      []string          `json:"fields,omitempty"`
	Values      map[string]string `json:"values,omitempty"`
}
