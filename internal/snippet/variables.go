package snippet

import (
	"fmt"
	"regexp"
	"strings"
	texttemplate "text/template"

	"github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/uozi-tech/cosy"
)

// Variable types the config template panel can fill in.
const (
	VariableString  = "string"
	VariableBoolean = "boolean"
	VariableSelect  = "select"
)

// variableKeyPattern keeps a key usable as {{ .key }} in the content.
var variableKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

// cleanVariables checks the variables of a header and trims their labels.
func cleanVariables(vars map[string]template.Variable) (map[string]template.Variable, error) {
	cleaned := make(map[string]template.Variable, len(vars))
	for key, v := range vars {
		if !variableKeyPattern.MatchString(key) {
			return nil, cosy.WrapErrorWithParams(ErrInvalidVariable, key, "use letters, digits and underscores, starting with a letter or underscore")
		}
		v.Name = cleanDescription(v.Name)
		switch v.Type {
		case VariableString:
			if v.Value == nil {
				v.Value = ""
			}
			if _, ok := v.Value.(string); !ok {
				return nil, cosy.WrapErrorWithParams(ErrInvalidVariable, key, "the default value must be text")
			}
			v.Mask = nil
		case VariableBoolean:
			if v.Value == nil {
				v.Value = false
			}
			if _, ok := v.Value.(bool); !ok {
				return nil, cosy.WrapErrorWithParams(ErrInvalidVariable, key, "the default value must be true or false")
			}
			v.Mask = nil
		case VariableSelect:
			if len(v.Mask) == 0 {
				return nil, cosy.WrapErrorWithParams(ErrInvalidVariable, key, "a select needs at least one option")
			}
			mask := make(map[string]map[string]string, len(v.Mask))
			for option, labels := range v.Mask {
				if strings.TrimSpace(option) == "" {
					return nil, cosy.WrapErrorWithParams(ErrInvalidVariable, key, "an option has no value")
				}
				mask[option] = cleanDescription(labels)
			}
			v.Mask = mask
			value, ok := v.Value.(string)
			if _, known := mask[value]; !ok || !known {
				return nil, cosy.WrapErrorWithParams(ErrInvalidVariable, key, "the default value must be one of the options")
			}
		default:
			return nil, cosy.WrapErrorWithParams(ErrInvalidVariable, key, fmt.Sprintf("unknown type %q", v.Type))
		}
		cleaned[key] = v
	}
	return cleaned, nil
}

// checkTemplate parses the content of a snippet with variables the way the
// config template panel renders it.
func checkTemplate(vars map[string]template.Variable, content string) error {
	if len(vars) == 0 {
		return nil
	}
	if _, err := texttemplate.New("snippet").Parse(content); err != nil {
		return cosy.WrapErrorWithParams(ErrInvalidTemplate, err.Error())
	}
	return nil
}

// PreviewResult is a snippet rendered the way the config template panel
// inserts it.
type PreviewResult struct {
	Content string `json:"content"`
	// Error tells why the content does not render or does not parse.
	Error string `json:"error,omitempty"`
}

// Preview renders a snippet that may not be saved yet with the given
// variable values. Content that comes from a plugin is rendered with the
// limits of plugin templates.
func Preview(content string, vars map[string]template.Variable, fromPlugin bool) (PreviewResult, error) {
	if len(content) > maxSnippetBytes {
		return PreviewResult{}, ErrTooLarge
	}
	render := template.RenderText
	if fromPlugin {
		render = template.RenderPluginText
	}
	rendered, err := render("snippet", content, vars)
	result := PreviewResult{Content: rendered}
	if err != nil {
		result.Error = err.Error()
	}
	return result, nil
}
