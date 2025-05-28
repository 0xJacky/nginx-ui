//go:generate go run .
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/uozi-tech/cosy/logger"
)

// Structure for notification function calls
type NotificationCall struct {
	Type    string
	Title   string
	Content string
	Path    string
}

// Directories to exclude
var excludeDirs = []string{
	".devcontainer", ".github", ".idea", ".pnpm-store",
	".vscode", "app", "query", "tmp", "cmd",
}

// Main function
func main() {
	logger.Init("release")
	// Start scanning from the project root
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		logger.Error("Unable to get the current file")
		return
	}

	root := filepath.Join(filepath.Dir(file), "../../")
	calls := []NotificationCall{}

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
			findNotificationCalls(path, &calls)
		}

		return nil
	})

	if err != nil {
		logger.Errorf("Error walking the path: %v\n", err)
		return
	}

	// Generate a single TS file
	generateSingleTSFile(root, calls)

	logger.Infof("Found %d notification calls\n", len(calls))
}

// Find notification function calls in Go files
func findNotificationCalls(filePath string, calls *[]NotificationCall) {
	// Parse Go code
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		logger.Errorf("Error parsing %s: %v\n", filePath, err)
		return
	}

	// Traverse the AST to find function calls
	ast.Inspect(node, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check if it's a call to the notification package
		selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		xident, ok := selExpr.X.(*ast.Ident)
		if !ok {
			return true
		}

		// Check if it's one of the functions we're interested in: notification.Info/Error/Warning/Success
		if xident.Name == "notification" {
			funcName := selExpr.Sel.Name
			if funcName == "Info" || funcName == "Error" || funcName == "Warning" || funcName == "Success" {
				// Function must have at least two parameters (title, content)
				if len(callExpr.Args) >= 2 {
					titleArg := callExpr.Args[0]
					contentArg := callExpr.Args[1]

					// Get parameter values
					title := getStringValue(titleArg)
					content := getStringValue(contentArg)

					// Ignore cases where content is a variable name or function call
					if content != "" && !isVariableOrFunctionCall(content) {
						*calls = append(*calls, NotificationCall{
							Type:    funcName,
							Title:   title,
							Content: content,
							Path:    filePath,
						})
					}
				}
			}
		}

		return true
	})
}

// Check if the string is a variable name or function call
func isVariableOrFunctionCall(s string) bool {
	// Simple check: if the string doesn't contain spaces or quotes, it might be a variable name
	if !strings.Contains(s, " ") && !strings.Contains(s, "\"") && !strings.Contains(s, "'") {
		return true
	}

	// If it looks like a function call, e.g., err.Error()
	if strings.Contains(s, "(") && strings.Contains(s, ")") {
		return true
	}

	return false
}

// Get string value from AST node
func getStringValue(expr ast.Expr) string {
	// Direct string
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		// Return string without quotes
		return strings.Trim(lit.Value, "\"")
	}

	// Recover string value from source code expression
	var str strings.Builder
	if bin, ok := expr.(*ast.BinaryExpr); ok {
		// Handle string concatenation expression
		leftStr := getStringValue(bin.X)
		rightStr := getStringValue(bin.Y)
		str.WriteString(leftStr)
		str.WriteString(rightStr)
	}

	if str.Len() > 0 {
		return str.String()
	}

	// Return empty string if unable to parse as string
	return ""
}

// Generate a single TypeScript file
func generateSingleTSFile(root string, calls []NotificationCall) {
	// Create target directory
	targetDir := filepath.Join(root, "app/src/components/Notification")
	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		logger.Errorf("Error creating directory %s: %v\n", targetDir, err)
		return
	}

	// Create file name
	tsFilePath := filepath.Join(targetDir, "notifications.ts")

	// Prepare file content
	var content strings.Builder
	content.WriteString("// Auto-generated notification texts\n")
	content.WriteString("// Extracted from Go source code notification function calls\n")
	content.WriteString("/* eslint-disable ts/no-explicit-any */\n\n")
	content.WriteString("const notifications: Record<string, { title: () => string, content: (args: any) => string }> = {\n")

	// Track used keys to avoid duplicates
	usedKeys := make(map[string]bool)

	// Organize notifications by directory
	messagesByDir := make(map[string][]NotificationCall)
	for _, call := range calls {
		dir := filepath.Dir(call.Path)
		// Extract module name from directory path
		dirParts := strings.Split(dir, "/")
		moduleName := dirParts[len(dirParts)-1]
		if strings.HasPrefix(dir, "internal/") || strings.HasPrefix(dir, "api/") {
			messagesByDir[moduleName] = append(messagesByDir[moduleName], call)
		} else {
			messagesByDir["general"] = append(messagesByDir["general"], call)
		}
	}

	// Add comments for each module and write notifications
	for module, moduleCalls := range messagesByDir {
		content.WriteString(fmt.Sprintf("\n  // %s module notifications\n", module))

		for _, call := range moduleCalls {
			// Escape quotes in title and content
			escapedTitle := strings.ReplaceAll(call.Title, "'", "\\'")
			escapedContent := strings.ReplaceAll(call.Content, "'", "\\'")

			// Use just the title as the key
			key := call.Title

			// Check if key is already used, generate unique key if necessary
			uniqueKey := key
			counter := 1
			for usedKeys[uniqueKey] {
				uniqueKey = fmt.Sprintf("%s_%d", key, counter)
				counter++
			}

			usedKeys[uniqueKey] = true

			// Write record with both title and content as functions
			content.WriteString(fmt.Sprintf("  '%s': {\n", uniqueKey))
			content.WriteString(fmt.Sprintf("    title: () => $gettext('%s'),\n", escapedTitle))
			content.WriteString(fmt.Sprintf("    content: (args: any) => $gettext('%s', args, true),\n", escapedContent))
			content.WriteString("  },\n")
		}
	}

	content.WriteString("}\n\n")
	content.WriteString("export default notifications\n")

	// Write file
	err = os.WriteFile(tsFilePath, []byte(content.String()), 0644)
	if err != nil {
		logger.Errorf("Error writing TS file %s: %v\n", tsFilePath, err)
		return
	}

	logger.Infof("Generated single TS file: %s with %d notifications\n", tsFilePath, len(calls))
}
