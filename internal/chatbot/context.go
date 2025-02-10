package chatbot

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy/logger"
	"os"
	"regexp"
	"strings"
)

type includeContext struct {
	Paths    []string
	PathsMap map[string]bool
}

func IncludeContext(filename string) (includes []string) {
	c := &includeContext{
		Paths:    make([]string, 0),
		PathsMap: make(map[string]bool),
	}

	c.extractIncludes(filename)

	return c.Paths
}

// extractIncludes extracts all include statements from the given nginx configuration file.
func (c *includeContext) extractIncludes(filename string) {
	if !helper.FileExists(filename) {
		logger.Error("File does not exist: ", filename)
		return
	}

	if !helper.IsUnderDirectory(filename, nginx.GetConfPath()) {
		logger.Error("File is not under the nginx conf path: ", filename)
		return
	}

	// Read the file content
	content, err := os.ReadFile(filename)
	if err != nil {
		logger.Error(err)
		return
	}

	// Find all include statements
	pattern := regexp.MustCompile(`(?m)^\s*include\s+([^;]+);`)
	matches := pattern.FindAllStringSubmatch(string(content), -1)
	for _, match := range matches {
		if len(match) > 1 {
			// Resolve the path of the included file
			includePath := match[1]

			// to avoid infinite loop
			if c.PathsMap[includePath] {
				continue
			}

			c.push(includePath)

			// Recursively extract includes from the included file
			c.extractIncludes(includePath)
		}
	}

	return
}

func (c *includeContext) push(path string) {
	c.Paths = append(c.Paths, path)
	c.PathsMap[path] = true
}

// getConfigIncludeContext returns the context of the given filename.
func getConfigIncludeContext(filename string) (multiContent []openai.ChatMessagePart) {
	multiContent = make([]openai.ChatMessagePart, 0)

	if !helper.IsUnderDirectory(filename, nginx.GetConfPath()) {
		return
	}

	includes := IncludeContext(filename)
	logger.Debug(includes)
	var sb strings.Builder
	for _, include := range includes {
		text, _ := os.ReadFile(nginx.GetConfPath(include))

		if len(text) == 0 {
			continue
		}

		sb.WriteString("The Content of ")
		sb.WriteString(include)
		sb.WriteString(",")
		sb.WriteString(string(text))

		multiContent = append(multiContent, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeText,
			Text: sb.String(),
		})

		sb.Reset()
	}
	return
}
