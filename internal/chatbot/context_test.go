package chatbot

import (
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

func TestRegex(t *testing.T) {
	content := `
server {
    listen 80;
    listen [::]:80;
    server_name _;
    include error_json;
}
`
	pattern := regexp.MustCompile(`(?m)^\s*include\s+([^;]+);`)
	matches := pattern.FindAllStringSubmatch(content, -1)

	assert.Equal(t, 1, len(matches))
	assert.Equal(t, "error_json", matches[0][1])
}
