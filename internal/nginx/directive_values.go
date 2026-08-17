package nginx

import (
	"fmt"
	"strings"
)

type DirectiveValueReplacement struct {
	Directive string
	OldValue  string
	NewValue  string
}

func DirectiveValues(content, directive string) ([]string, error) {
	tokens, err := tokenizeNginxConfig(content)
	if err != nil {
		return nil, err
	}

	values := make([]string, 0)
	statementStart := true
	for i, token := range tokens {
		switch token.value {
		case "{", "}", ";":
			statementStart = true
			continue
		}

		if statementStart && strings.EqualFold(token.value, directive) && i+2 < len(tokens) &&
			tokens[i+2].value == ";" {
			values = append(values, tokens[i+1].value)
		}
		statementStart = false
	}

	return values, nil
}

func RewriteDirectiveValues(content string, replacements []DirectiveValueReplacement) (string, int, error) {
	tokens, err := tokenizeNginxConfig(content)
	if err != nil {
		return "", 0, err
	}

	type edit struct {
		start       int
		end         int
		replacement string
	}
	edits := make([]edit, 0, len(replacements))
	statementStart := true
	for i, token := range tokens {
		switch token.value {
		case "{", "}", ";":
			statementStart = true
			continue
		}

		if !statementStart || i+2 >= len(tokens) || tokens[i+2].value != ";" {
			statementStart = false
			continue
		}

		valueToken := tokens[i+1]
		for _, replacement := range replacements {
			if strings.EqualFold(token.value, replacement.Directive) && valueToken.value == replacement.OldValue {
				rendered, renderErr := renderDirectiveValue(content[valueToken.start:valueToken.end], replacement.NewValue)
				if renderErr != nil {
					return "", 0, renderErr
				}
				edits = append(edits, edit{start: valueToken.start, end: valueToken.end, replacement: rendered})
				break
			}
		}
		statementStart = false
	}

	if len(edits) == 0 {
		return content, 0, nil
	}

	var builder strings.Builder
	last := 0
	for _, current := range edits {
		builder.WriteString(content[last:current.start])
		builder.WriteString(current.replacement)
		last = current.end
	}
	builder.WriteString(content[last:])

	return builder.String(), len(edits), nil
}

func renderDirectiveValue(original, value string) (string, error) {
	if original == "" {
		return "", fmt.Errorf("empty nginx directive value token")
	}

	switch original[0] {
	case '\'':
		return "'" + strings.ReplaceAll(value, "'", "\\'") + "'", nil
	case '"':
		escaped := strings.ReplaceAll(value, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`, nil
	default:
		if strings.ContainsAny(value, " \t\r\n#{};") {
			escaped := strings.ReplaceAll(value, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			return `"` + escaped + `"`, nil
		}
		return value, nil
	}
}
