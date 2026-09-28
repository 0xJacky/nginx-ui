package access_list

import (
	"sort"
	"strings"
)

// span is a half-open byte range of the source text.
type span struct {
	start int
	end   int
}

// removeSpans deletes the given ranges. A range that is the only thing on its
// line takes the whole line with it, so removing a directive does not leave a
// blank line behind.
func removeSpans(src string, spans []span) string {
	expanded := make([]span, 0, len(spans))
	for _, s := range spans {
		expanded = append(expanded, expandToLine(src, s))
	}
	sort.Slice(expanded, func(i, j int) bool { return expanded[i].start > expanded[j].start })

	for i, s := range expanded {
		// Overlapping ranges can only come from the same line; skip the inner one.
		if i > 0 && s.end > expanded[i-1].start {
			continue
		}
		src = src[:s.start] + src[s.end:]
	}
	return src
}

func expandToLine(src string, s span) span {
	lineStart := strings.LastIndexByte(src[:s.start], '\n') + 1
	lineEnd := strings.IndexByte(src[s.end:], '\n')
	if lineEnd < 0 {
		lineEnd = len(src)
	} else {
		lineEnd += s.end
	}

	prefixBlank := strings.TrimSpace(src[lineStart:s.start]) == ""
	suffixBlank := strings.TrimSpace(src[s.end:lineEnd]) == ""
	switch {
	case prefixBlank && suffixBlank:
		end := lineEnd
		if end < len(src) {
			end++
		}
		return span{start: lineStart, end: end}
	case suffixBlank:
		// Keep the rest of the line, drop the trailing spaces.
		start := s.start
		for start > lineStart && (src[start-1] == ' ' || src[start-1] == '\t') {
			start--
		}
		return span{start: start, end: lineEnd}
	default:
		end := s.end
		for end < lineEnd && (src[end] == ' ' || src[end] == '\t') {
			end++
		}
		return span{start: s.start, end: end}
	}
}

// insertLines adds lines to a block. With a nil anchor they become the first
// lines of the body, otherwise they follow the line of the anchor directive.
// root marks the whole text as the body, which is how a location body is kept
// by the basic mode editor.
func insertLines(src string, block *statement, root bool, anchor *statement, lines []string) string {
	indent := childIndent(src, block, root)

	if root && anchor == nil {
		var b strings.Builder
		for _, line := range lines {
			b.WriteString(line)
			b.WriteByte('\n')
		}
		return b.String() + src
	}

	pos := block.BodyStart
	bodyEnd := block.BodyEnd
	if root {
		bodyEnd = len(src)
	}
	if anchor != nil {
		pos = anchor.End
	}

	nl := strings.IndexByte(src[pos:], '\n')
	if nl >= 0 && pos+nl < bodyEnd {
		at := pos + nl + 1
		var b strings.Builder
		for _, line := range lines {
			b.WriteString(indent)
			b.WriteString(line)
			b.WriteByte('\n')
		}
		return src[:at] + b.String() + src[at:]
	}

	// The block continues on the same line, for example "server { listen 80; }".
	if root {
		prefix := "\n"
		if src == "" || strings.HasSuffix(src, "\n") {
			prefix = ""
		}
		return src + prefix + strings.Join(lines, "\n") + "\n"
	}
	return src[:pos] + " " + strings.Join(lines, " ") + src[pos:]
}

// childIndent guesses the indentation of a new directive inside block.
func childIndent(src string, block *statement, root bool) string {
	if len(block.Children) > 0 {
		if indent, ok := lineIndent(src, block.Children[0].Start); ok {
			return indent
		}
	}
	if root {
		return ""
	}
	parent, _ := lineIndent(src, block.Start)
	return parent + indentUnit(src)
}

// lineIndent returns the whitespace in front of offset when nothing else
// precedes it on its line.
func lineIndent(src string, offset int) (string, bool) {
	lineStart := strings.LastIndexByte(src[:offset], '\n') + 1
	prefix := src[lineStart:offset]
	if strings.TrimSpace(prefix) != "" {
		return "", false
	}
	return prefix, true
}

func indentUnit(src string) string {
	if strings.Contains(src, "\n\t") {
		return "\t"
	}
	return "    "
}
