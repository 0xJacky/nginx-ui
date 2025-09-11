//go:generate go run .
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ulikunitz/xz"
)

type License struct {
	Name    string `json:"name"`
	License string `json:"license"`
	URL     string `json:"url"`
	Version string `json:"version"`
}

type ComponentInfo struct {
	Backend  []License `json:"backend"`
	Frontend []License `json:"frontend"`
}

func main() {
	log.Println("Generating license information...")

	var info ComponentInfo

	// Generate backend licenses
	backendLicenses, err := generateBackendLicenses()
	if err != nil {
		log.Printf("Error generating backend licenses: %v", err)
	} else {
		info.Backend = backendLicenses
		log.Printf("INFO: Backend license collection completed: %d components", len(backendLicenses))
	}

	// Generate frontend licenses
	frontendLicenses, err := generateFrontendLicenses()
	if err != nil {
		log.Printf("Error generating frontend licenses: %v", err)
	} else {
		info.Frontend = frontendLicenses
		log.Printf("INFO: Frontend license collection completed: %d components", len(frontendLicenses))
	}

	log.Println("INFO: Serializing license data to JSON...")
	// Marshal to JSON
	jsonData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}
	log.Printf("INFO: JSON size: %d bytes", len(jsonData))

	log.Println("INFO: Compressing license data with xz...")
	// Compress with xz
	var compressed bytes.Buffer
	writer, err := xz.NewWriter(&compressed)
	if err != nil {
		log.Fatalf("Error creating xz writer: %v", err)
	}

	_, err = writer.Write(jsonData)
	if err != nil {
		log.Fatalf("Error writing compressed data: %v", err)
	}

	err = writer.Close()
	if err != nil {
		log.Fatalf("Error closing xz writer: %v", err)
	}
	log.Printf("INFO: Compressed size: %d bytes (%.1f%% of original)",
		compressed.Len(), float64(compressed.Len())/float64(len(jsonData))*100)

	// Write compressed data to file
	outputPath := "internal/license/licenses.xz"
	log.Printf("INFO: Writing compressed data to %s", outputPath)
	err = os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err != nil {
		log.Fatalf("Error creating output directory: %v", err)
	}

	err = os.WriteFile(outputPath, compressed.Bytes(), 0644)
	if err != nil {
		log.Fatalf("Error writing output file: %v", err)
	}

	log.Printf("SUCCESS: License data generated successfully!")
	log.Printf("  - Backend components: %d", len(info.Backend))
	log.Printf("  - Frontend components: %d", len(info.Frontend))
	log.Printf("  - Total components: %d", len(info.Backend)+len(info.Frontend))
	log.Printf("  - Compressed size: %d bytes", compressed.Len())
	log.Printf("  - Output file: %s", outputPath)
}

func generateBackendLicenses() ([]License, error) {
	var licenses []License

	log.Println("INFO: Collecting backend Go modules...")

	// Read go.mod file directly
	goModPath := "go.mod"
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod: %v", err)
	}

	// Parse go.mod content to extract dependencies
	depMap := make(map[string]string) // path -> version
	lines := strings.Split(string(data), "\n")
	inRequireBlock := false
	inReplaceBlock := false
	
	replaceMap := make(map[string]string) // original -> replacement

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Handle require block
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}
		if strings.HasPrefix(line, "replace (") {
			inReplaceBlock = true
			continue
		}
		if line == ")" {
			inRequireBlock = false
			inReplaceBlock = false
			continue
		}

		// Parse replace directives
		if inReplaceBlock || strings.HasPrefix(line, "replace ") {
			if strings.Contains(line, "=>") {
				parts := strings.Split(line, "=>")
				if len(parts) == 2 {
					original := strings.TrimSpace(parts[0])
					replacement := strings.TrimSpace(parts[1])
					
					// Remove "replace " prefix if present
					original = strings.TrimPrefix(original, "replace ")
					
					// Extract module path (before version if present)
					if strings.Contains(original, " ") {
						original = strings.Fields(original)[0]
					}
					if strings.Contains(replacement, " ") {
						replacement = strings.Fields(replacement)[0]
					}
					
					replaceMap[original] = replacement
				}
			}
			continue
		}

		// Parse dependencies in require block or single require line
		if inRequireBlock || strings.HasPrefix(line, "require ") {
			// Remove "require " prefix if present
			line = strings.TrimPrefix(line, "require ")
			
			// Remove comments
			if idx := strings.Index(line, "//"); idx != -1 {
				line = line[:idx]
			}
			line = strings.TrimSpace(line)
			
			if line == "" {
				continue
			}

			// Parse "module version" format
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				path := parts[0]
				version := parts[1]
				
				if path == "" {
					continue
				}
				
				// Apply replacements if they exist
				if replacement, exists := replaceMap[path]; exists {
					path = replacement
				}
				
				depMap[path] = version
			}
		}
	}

	// Convert map to slice
	var allMods []struct {
		Path    string `json:"Path"`
		Version string `json:"Version"`
	}

	for path, version := range depMap {
		allMods = append(allMods, struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
		}{
			Path:    path,
			Version: version,
		})
	}

	// Add Go language itself
	goLicense := License{
		Name:    "Go Programming Language",
		Version: getGoVersion(),
		URL:     "https://golang.org",
		License: "BSD-3-Clause",
	}
	licenses = append(licenses, goLicense)

	log.Printf("INFO: Found %d backend dependencies (+ Go language)", len(allMods))

	// Process modules in parallel
	const maxWorkers = 64
	jobs := make(chan struct {
		Path    string
		Version string
		Index   int
	}, len(allMods))

	results := make(chan License, len(allMods))

	// Progress tracking
	var processed int32
	var mu sync.Mutex

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				license := License{
					Name:    job.Path,
					Version: job.Version,
					URL:     fmt.Sprintf("https://%s", job.Path),
				}

				// Try to get license info from various sources
				licenseText := tryGetLicenseFromGit(job.Path)
				if licenseText == "" {
					licenseText = detectCommonLicense(job.Path)
				}
				license.License = licenseText

				mu.Lock()
				processed++
				currentCount := processed
				mu.Unlock()

				log.Printf("INFO: [%d/%d] Backend: %s -> %s", currentCount, len(allMods), job.Path, licenseText)
				results <- license
			}
		}(i)
	}

	// Send jobs
	go func() {
		for i, mod := range allMods {
			jobs <- struct {
				Path    string
				Version string
				Index   int
			}{
				Path:    mod.Path,
				Version: mod.Version,
				Index:   i,
			}
		}
		close(jobs)
	}()

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for license := range results {
		licenses = append(licenses, license)
	}

	return licenses, nil
}

func generateFrontendLicenses() ([]License, error) {
	var licenses []License

	log.Println("INFO: Collecting frontend npm packages...")

	// Read package.json
	packagePath := "app/package.json"
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("package.json not found at %s", packagePath)
	}

	data, err := os.ReadFile(packagePath)
	if err != nil {
		return nil, err
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	log.Printf("INFO: Found %d frontend dependencies", len(pkg.Dependencies))

	// Convert map to slice for easier parallel processing
	var packages []struct {
		Name    string
		Version string
		Index   int
	}

	i := 0
	for name, version := range pkg.Dependencies {
		packages = append(packages, struct {
			Name    string
			Version string
			Index   int
		}{
			Name:    name,
			Version: version,
			Index:   i,
		})
		i++
	}

	// Process packages in parallel
	const maxWorkers = 64
	jobs := make(chan struct {
		Name    string
		Version string
		Index   int
	}, len(packages))

	results := make(chan License, len(packages))

	// Progress tracking
	var processed int32
	var mu sync.Mutex

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				license := License{
					Name:    job.Name,
					Version: job.Version,
					URL:     fmt.Sprintf("https://www.npmjs.com/package/%s", job.Name),
				}

				// Try to get license info
				licenseText := tryGetNpmLicense(job.Name)
				if licenseText == "" {
					licenseText = "Unknown"
				}
				license.License = licenseText

				mu.Lock()
				processed++
				currentCount := processed
				mu.Unlock()

				log.Printf("INFO: [%d/%d] Frontend: %s -> %s", currentCount, len(packages), job.Name, licenseText)
				results <- license
			}
		}(i)
	}

	// Send jobs
	go func() {
		for _, pkg := range packages {
			jobs <- pkg
		}
		close(jobs)
	}()

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for license := range results {
		licenses = append(licenses, license)
	}

	return licenses, nil
}

func tryGetLicenseFromGitHub(modulePath string) string {
	// Extract GitHub info from module path
	if !strings.HasPrefix(modulePath, "github.com/") {
		return ""
	}

	parts := strings.Split(modulePath, "/")
	if len(parts) < 3 {
		return ""
	}

	owner := parts[1]
	repo := parts[2]

	// Try common license files and branches
	licenseFiles := []string{"LICENSE", "LICENSE.txt", "LICENSE.md", "COPYING", "COPYING.txt", "License", "license"}
	branches := []string{"master", "main", "HEAD"}

	for _, branch := range branches {
		for _, file := range licenseFiles {
			url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, file)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					continue
				}

				// Extract license type from content
				content := string(body)
				licenseType := extractLicenseType(content)
				if licenseType != "Custom" {
					return licenseType
				}
			}
		}
	}

	return ""
}

func tryGetLicenseFromGit(modulePath string) string {
	// Try different Git hosting platforms
	if strings.HasPrefix(modulePath, "github.com/") {
		return tryGetLicenseFromGitHub(modulePath)
	}

	if strings.HasPrefix(modulePath, "gitlab.com/") {
		return tryGetLicenseFromGitLab(modulePath)
	}

	if strings.HasPrefix(modulePath, "gitee.com/") {
		return tryGetLicenseFromGitee(modulePath)
	}

	if strings.HasPrefix(modulePath, "bitbucket.org/") {
		return tryGetLicenseFromBitbucket(modulePath)
	}

	// Try to use go.mod info or pkg.go.dev API
	return tryGetLicenseFromPkgGoDev(modulePath)
}

func tryGetLicenseFromGitLab(modulePath string) string {
	parts := strings.Split(modulePath, "/")
	if len(parts) < 3 {
		return ""
	}

	owner := parts[1]
	repo := parts[2]

	// GitLab raw file URL format
	licenseFiles := []string{"LICENSE", "LICENSE.txt", "LICENSE.md", "COPYING", "License"}
	branches := []string{"master", "main"}

	for _, branch := range branches {
		for _, file := range licenseFiles {
			url := fmt.Sprintf("https://gitlab.com/%s/%s/-/raw/%s/%s", owner, repo, branch, file)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					continue
				}

				content := string(body)
				licenseType := extractLicenseType(content)
				if licenseType != "Custom" {
					return licenseType
				}
			}
		}
	}

	return ""
}

func tryGetLicenseFromGitee(modulePath string) string {
	parts := strings.Split(modulePath, "/")
	if len(parts) < 3 {
		return ""
	}

	owner := parts[1]
	repo := parts[2]

	// Gitee raw file URL format
	licenseFiles := []string{"LICENSE", "LICENSE.txt", "LICENSE.md", "COPYING"}
	branches := []string{"master", "main"}

	for _, branch := range branches {
		for _, file := range licenseFiles {
			url := fmt.Sprintf("https://gitee.com/%s/%s/raw/%s/%s", owner, repo, branch, file)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					continue
				}

				content := string(body)
				licenseType := extractLicenseType(content)
				if licenseType != "Custom" {
					return licenseType
				}
			}
		}
	}

	return ""
}

func tryGetLicenseFromBitbucket(modulePath string) string {
	parts := strings.Split(modulePath, "/")
	if len(parts) < 3 {
		return ""
	}

	owner := parts[1]
	repo := parts[2]

	// Bitbucket raw file URL format
	licenseFiles := []string{"LICENSE", "LICENSE.txt", "LICENSE.md", "COPYING"}
	branches := []string{"master", "main"}

	for _, branch := range branches {
		for _, file := range licenseFiles {
			url := fmt.Sprintf("https://bitbucket.org/%s/%s/raw/%s/%s", owner, repo, branch, file)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					continue
				}

				content := string(body)
				licenseType := extractLicenseType(content)
				if licenseType != "Custom" {
					return licenseType
				}
			}
		}
	}

	return ""
}

func tryGetLicenseFromPkgGoDev(modulePath string) string {
	// Try to get license info from pkg.go.dev API
	url := fmt.Sprintf("https://api.deps.dev/v3alpha/systems/go/packages/%s", modulePath)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	var apiResponse struct {
		Package struct {
			License []struct {
				Type string `json:"type"`
			} `json:"license"`
		} `json:"package"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return ""
	}

	if len(apiResponse.Package.License) > 0 {
		return apiResponse.Package.License[0].Type
	}

	return ""
}

func tryGetNpmLicense(packageName string) string {
	// Try to get license from npm registry
	url := fmt.Sprintf("https://registry.npmjs.org/%s/latest", packageName)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	var pkg struct {
		License interface{} `json:"license"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return ""
	}

	switch v := pkg.License.(type) {
	case string:
		return v
	case map[string]interface{}:
		if t, ok := v["type"].(string); ok {
			return t
		}
	}

	return ""
}

func extractLicenseType(content string) string {
	content = strings.ToUpper(content)

	licensePatterns := map[string]*regexp.Regexp{
		"MIT":        regexp.MustCompile(`MIT\s+LICENSE`),
		"Apache-2.0": regexp.MustCompile(`APACHE\s+LICENSE.*VERSION\s+2\.0`),
		"GPL-3.0":    regexp.MustCompile(`GNU\s+GENERAL\s+PUBLIC\s+LICENSE.*VERSION\s+3`),
		"BSD-3":      regexp.MustCompile(`BSD\s+3-CLAUSE`),
		"BSD-2":      regexp.MustCompile(`BSD\s+2-CLAUSE`),
		"ISC":        regexp.MustCompile(`ISC\s+LICENSE`),
		"AGPL-3.0":   regexp.MustCompile(`GNU\s+AFFERO\s+GENERAL\s+PUBLIC\s+LICENSE`),
	}

	for license, pattern := range licensePatterns {
		if pattern.MatchString(content) {
			return license
		}
	}

	return "Custom"
}

func detectCommonLicense(modulePath string) string {
	// Common patterns for detecting license types based on module paths
	commonLicenses := map[string]string{
		"golang.org/x":               "BSD-3-Clause",
		"google.golang.org":          "Apache-2.0",
		"gopkg.in":                   "Various",
		"go.uber.org":                "MIT",
		"go.etcd.io":                 "Apache-2.0",
		"go.mongodb.org":             "Apache-2.0",
		"go.opentelemetry.io":        "Apache-2.0",
		"k8s.io":                     "Apache-2.0",
		"sigs.k8s.io":                "Apache-2.0",
		"cloud.google.com":           "Apache-2.0",
		"go.opencensus.io":           "Apache-2.0",
		"contrib.go.opencensus.io":   "Apache-2.0",
		"github.com/golang/":         "BSD-3-Clause",
		"github.com/google/":         "Apache-2.0",
		"github.com/grpc-ecosystem/": "Apache-2.0",
		"github.com/prometheus/":     "Apache-2.0",
		"github.com/coreos/":         "Apache-2.0",
		"github.com/etcd-io/":        "Apache-2.0",
		"github.com/go-kit/":         "MIT",
		"github.com/sirupsen/":       "MIT",
		"github.com/stretchr/":       "MIT",
		"github.com/spf13/":          "Apache-2.0",
		"github.com/gorilla/":        "BSD-3-Clause",
		"github.com/gin-gonic/":      "MIT",
		"github.com/labstack/":       "MIT",
		"github.com/julienschmidt/":  "BSD-2-Clause",
	}

	for prefix, license := range commonLicenses {
		if strings.HasPrefix(modulePath, prefix) {
			return license
		}
	}

	return "Unknown"
}

func getGoVersion() string {
	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err != nil {
		return "Unknown"
	}

	// Parse "go version go1.21.5 darwin/amd64" to extract "go1.21.5"
	parts := strings.Fields(string(output))
	if len(parts) >= 3 {
		return parts[2] // "go1.21.5"
	}

	return "Unknown"
}
