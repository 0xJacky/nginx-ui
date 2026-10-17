package config

import (
	"bytes"
	"encoding/pem"
	"path/filepath"
	"strings"
	"unicode"
)

// ParsePEMData consumes the entire PEM stream. pem.Decode alone skips arbitrary
// text before a block, which would allow directives before or after a real key.
// Hash comments and whitespace between blocks are safe in both PEM and Nginx.
func ParsePEMData(content []byte) ([]*pem.Block, error) {
	if err := ValidateDataContent(content); err != nil {
		return nil, err
	}
	var blocks []*pem.Block
	for len(bytes.TrimSpace(content)) > 0 {
		content = bytes.TrimSpace(content)
		if content[0] == '#' {
			if newline := bytes.IndexByte(content, '\n'); newline >= 0 {
				content = content[newline+1:]
				continue
			}
			break
		}
		const begin = "-----BEGIN "
		if !bytes.HasPrefix(content, []byte(begin)) {
			return nil, ErrConfigDataInvalid
		}
		lineEnd := bytes.IndexByte(content, '\n')
		if lineEnd < 0 {
			return nil, ErrConfigDataInvalid
		}
		line := strings.TrimSuffix(string(content[:lineEnd]), "\r")
		if !strings.HasSuffix(line, "-----") {
			return nil, ErrConfigDataInvalid
		}
		kind := strings.TrimSuffix(strings.TrimPrefix(line, begin), "-----")
		end := []byte("-----END " + kind + "-----")
		endOffset := bytes.Index(content[lineEnd+1:], end)
		if endOffset < 0 {
			return nil, ErrConfigDataInvalid
		}
		endOffset += lineEnd + 1 + len(end)
		encoded := content[:endOffset]
		if bytes.Contains(encoded[lineEnd+1:], []byte(begin)) {
			return nil, ErrConfigDataInvalid
		}
		block, remaining := pem.Decode(encoded)
		if block == nil || block.Type != kind || len(bytes.TrimSpace(remaining)) != 0 {
			return nil, ErrConfigDataInvalid
		}
		blocks = append(blocks, block)
		content = content[endOffset:]
	}
	return blocks, nil
}

// ValidateDataFileContent validates the format as well as the text encoding.
// A data extension must not turn an arbitrary directive stream into trusted data,
// including when an existing include or glob already references the file.
func ValidateDataFileContent(path string, content []byte) error {
	if err := ValidateDataContent(content); err != nil {
		return err
	}
	if strings.EqualFold(filepath.Ext(path), ".htpasswd") {
		return validateHTPasswd(content)
	}
	_, err := ParsePEMData(content)
	return err
}

func validateHTPasswd(content []byte) error {
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, _, ok := strings.Cut(line, ":")
		if !ok || name == "" {
			return ErrConfigDataInvalid
		}
		// The colon must remain part of the first unquoted Nginx token. Passwords
		// may contain literal punctuation, including braces and semicolons.
		if strings.ContainsAny(name, "\\\"'{};#") || strings.ContainsFunc(name, unicode.IsSpace) {
			return ErrConfigDataInvalid
		}
	}
	return nil
}
