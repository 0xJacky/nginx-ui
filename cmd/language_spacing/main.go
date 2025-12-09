//go:generate go run .
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

type fileReport struct {
	path       string
	insertions int
}

var (
	dryRun    = flag.Bool("dry-run", false, "only report issues without modifying files")
	targetDir string
	rootDir   string
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	if err := resolvePaths(); err != nil {
		log.Fatalf("resolve paths: %v", err)
	}

	reports, totalInsertions, err := processDirectory(*dryRun)
	if err != nil {
		log.Fatalf("scan failed: %v", err)
	}

	if len(reports) == 0 {
		log.Println("No spacing issues detected.")
		return
	}

	for _, r := range reports {
		relative := r.path
		if rel, err := filepath.Rel(rootDir, r.path); err == nil {
			relative = rel
		}
		fmt.Printf("%s: inserted %d space(s)\n", relative, r.insertions)
	}

	if *dryRun {
		log.Printf("Dry run complete. %d potential insertion(s) across %d file(s).", totalInsertions, len(reports))
		return
	}

	log.Printf("Completed fixes. Inserted %d space(s) across %d file(s).", totalInsertions, len(reports))
}

func resolvePaths() error {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("unable to determine caller")
	}

	rootDir = filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	targetDir = filepath.Join(rootDir, "app/src/language")

	info, err := os.Stat(targetDir)
	if err != nil {
		return fmt.Errorf("stat language directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("language path is not a directory: %s", targetDir)
	}
	return nil
}

func processDirectory(dryRun bool) ([]fileReport, int, error) {
	reports := make([]fileReport, 0)
	totalInsertions := 0

	err := filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !isSupportedFile(path) {
			return nil
		}

		original, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		fixed, insertions := fixContent(string(original))
		if insertions == 0 {
			return nil
		}

		if !dryRun {
			if err := os.WriteFile(path, []byte(fixed), info.Mode().Perm()); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
		}

		reports = append(reports, fileReport{
			path:       path,
			insertions: insertions,
		})
		totalInsertions += insertions
		return nil
	})

	return reports, totalInsertions, err
}

func isSupportedFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".po", ".pot", ".ts":
		return true
	default:
		return false
	}
}

func fixContent(text string) (string, int) {
	runes := []rune(text)
	if len(runes) == 0 {
		return text, 0
	}

	var builder strings.Builder
	builder.Grow(len(runes) + 16)
	insertions := 0

	for i := 0; i < len(runes); i++ {
		current := runes[i]
		builder.WriteRune(current)

		if i == len(runes)-1 {
			break
		}

		next := runes[i+1]
		if needsSpace(current, next) {
			builder.WriteRune(' ')
			insertions++
		}
	}

	return builder.String(), insertions
}

func needsSpace(left, right rune) bool {
	if isHan(left) && isASCIIAlphaNum(right) {
		return true
	}
	if isASCIIAlphaNum(left) && isHan(right) {
		return true
	}
	return false
}

func isHan(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

func isASCIIAlphaNum(r rune) bool {
	return r <= unicode.MaxASCII && ((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
}
