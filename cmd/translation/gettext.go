// go run .
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/uozi-tech/cosy/logger"
)

// Directories to exclude
var excludeDirs = []string{
	".devcontainer", ".github", ".idea", ".pnpm-store",
	".vscode", "app", "query", "tmp", "cmd",
}

// Regular expression to match import statements for translation package
var importRegex = regexp.MustCompile(`import\s+\(\s*((?:.|\n)*?)\s*\)|\s*import\s+(.*?)\s+".*?(?:internal/translation|github\.com/0xJacky/Nginx-UI/internal/translation)"`)
var singleImportRegex = regexp.MustCompile(`\s*(?:(\w+)\s+)?".*?(?:internal/translation|github\.com/0xJacky/Nginx-UI/internal/translation)"`)

func main() {
	logger.Init("release")
	// Start scanning from the project root
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		logger.Error("Unable to get the current file")
		return
	}

	root := filepath.Join(filepath.Dir(file), "../../")
	calls := make(map[string]bool)

	// Scan all Go files
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded directories
		for _, dir := range excludeDirs {
			if strings.Contains(path, dir) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Only process Go files
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			findTranslationC(path, calls)
		}

		return nil
	})

	if err != nil {
		logger.Errorf("Error walking the path: %v\n", err)
		return
	}

	// Generate a single TS file
	generateSingleTSFile(root, calls)

	logger.Infof("Found %d translation messages\n", len(calls))
}

// findTranslationC finds all translation.C calls in a file and adds them to the calls map
func findTranslationC(filePath string, calls map[string]bool) {
	// Read the entire file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		logger.Errorf("Error reading file %s: %v\n", filePath, err)
		return
	}

	fileContent := string(content)

	// Find the translation package alias from import statements
	alias := findTranslationAlias(fileContent)
	if alias == "" {
		// No translation package imported, skip this file
		return
	}

	// Create regex pattern based on the alias
	pattern := fmt.Sprintf(`%s\.C\(\s*["']([^"']*(?:\\["'][^"']*)*)["']`, alias)
	cCallRegex := regexp.MustCompile(pattern)

	// Handle backtick strings separately (multi-line strings)
	backtickPattern := fmt.Sprintf(`%s\.C\(\s*\x60([^\x60]*)\x60`, alias)
	backtickRegex := regexp.MustCompile(backtickPattern)

	// Find all matches with regular quotes
	matches := cCallRegex.FindAllStringSubmatch(fileContent, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			message := match[1]
			// Clean up the message (remove escaped quotes, etc.)
			message = strings.ReplaceAll(message, "\\\"", "\"")
			message = strings.ReplaceAll(message, "\\'", "'")

			// Add to the map if not already present
			if _, exists := calls[message]; !exists {
				calls[message] = true
			}
		}
	}

	// Find all matches with backticks
	backtickMatches := backtickRegex.FindAllStringSubmatch(fileContent, -1)
	for _, match := range backtickMatches {
		if len(match) >= 2 {
			message := match[1]

			// Add to the map if not already present
			if _, exists := calls[message]; !exists {
				calls[message] = true
			}
		}
	}
}

// findTranslationAlias finds the alias for the translation package in import statements
func findTranslationAlias(fileContent string) string {
	// Default alias
	alias := "translation"

	// Find import blocks
	matches := importRegex.FindAllStringSubmatch(fileContent, -1)
	for _, match := range matches {
		if len(match) >= 3 && match[1] != "" {
			// This is a block import, search inside it
			imports := match[1]
			singleMatches := singleImportRegex.FindAllStringSubmatch(imports, -1)
			for _, singleMatch := range singleMatches {
				if len(singleMatch) >= 2 && singleMatch[1] != "" {
					// Custom alias found
					return singleMatch[1]
				}
			}
		} else if len(match) >= 3 && match[2] != "" {
			// This is a single-line import
			singleMatch := singleImportRegex.FindAllStringSubmatch(match[2], -1)
			if len(singleMatch) > 0 && len(singleMatch[0]) >= 2 && singleMatch[0][1] != "" {
				// Custom alias found
				return singleMatch[0][1]
			}
		}
	}

	return alias
}

// generateSingleTSFile generates a single TS file with all translation messages
func generateSingleTSFile(root string, calls map[string]bool) {
	outputPath := filepath.Join(root, "app/src/language/generate.ts")

	// Create the directory if it doesn't exist
	err := os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err != nil {
		logger.Errorf("Error creating directory: %v\n", err)
		return
	}

	// Create the output file
	file, err := os.Create(outputPath)
	if err != nil {
		logger.Errorf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// Write the header
	writer.WriteString("// This file is auto-generated. DO NOT EDIT MANUALLY.\n\n")
	writer.WriteString("export const msg = [\n")

	// Write each translation message
	for message := range calls {
		// Escape single quotes in the message for JavaScript
		escapedMessage := strings.ReplaceAll(message, "'", "\\'")
		writer.WriteString(fmt.Sprintf("  $gettext('%s'),\n", escapedMessage))
	}

	writer.WriteString("]\n")
	writer.Flush()

	logger.Infof("Generated TS file at %s\n", outputPath)
}
