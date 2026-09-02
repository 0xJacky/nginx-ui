package pages

import (
	"embed"
	"html/template"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
)

//go:embed *.tmpl
var tmplFS embed.FS

// maintenanceSiteHeader carries the site name injected by the generated
// maintenance nginx configuration, so a per-site template can be selected.
const maintenanceSiteHeader = "X-Maintenance-Site"

// MaintenancePageData maintenance page data structure
type MaintenancePageData struct {
	Title                string `json:"title"`
	Message              string `json:"message"`
	Description          string `json:"description"`
	ICPNumber            string `json:"icp_number"`
	PublicSecurityNumber string `json:"public_security_number"`
}

const (
	Title       = "System Maintenance"
	Message     = "We are currently performing system maintenance to improve your experience."
	Description = "Please check back later. Thank you for your understanding and patience."
)

// MaintenancePage returns a maintenance page
func MaintenancePage(c *gin.Context) {
	// Prepare template data
	data := MaintenancePageData{
		Title:                Title,
		Message:              Message,
		Description:          Description,
		ICPNumber:            settings.NodeSettings.ICPNumber,
		PublicSecurityNumber: settings.NodeSettings.PublicSecurityNumber,
	}

	// Check User-Agent
	userAgent := c.GetHeader("User-Agent")
	isBrowser := len(userAgent) > 0 && (contains(userAgent, "Mozilla") ||
		contains(userAgent, "Chrome") ||
		contains(userAgent, "Safari") ||
		contains(userAgent, "Edge") ||
		contains(userAgent, "Firefox") ||
		contains(userAgent, "Opera"))

	if !isBrowser {
		c.JSON(http.StatusServiceUnavailable, data)
		return
	}

	// Try custom mounted HTML first (NGINX_UI_NGINX_MAINTENANCE_TEMPLATE),
	// preferring the template dedicated to the requesting site.
	if content := readMaintenanceTemplate(c.GetHeader(maintenanceSiteHeader)); content != nil {
		c.Data(http.StatusServiceUnavailable, "text/html; charset=utf-8", content)
		return
	}

	// Fallback: embedded template
	tmpl, err := template.ParseFS(tmplFS, "maintenance.tmpl")
	if err != nil {
		c.String(http.StatusInternalServerError, "503 Service Unavailable")
		return
	}

	// Set content type
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusServiceUnavailable)

	// Render template
	err = tmpl.Execute(c.Writer, data)
	if err != nil {
		c.String(http.StatusInternalServerError, "503 Service Unavailable")
		return
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// readMaintenanceTemplate loads the custom maintenance page from the configured
// maintenance directory. The site specific file "<site>.<template>" wins, and the
// shared "<template>" file is used when it does not exist. A nil result means the
// caller should fall back to the built-in generic maintenance page.
func readMaintenanceTemplate(siteName string) []byte {
	templateName := sanitizeMaintenanceFileName(settings.NginxSettings.MaintenanceTemplate)
	if templateName == "" {
		return nil
	}

	dir := settings.NginxSettings.GetMaintenanceDir()
	names := make([]string, 0, 2)
	if site := sanitizeMaintenanceFileName(siteName); site != "" {
		names = append(names, site+"."+templateName)
	}
	names = append(names, templateName)

	for _, name := range names {
		// filepath.IsLocal rejects absolute paths and any ".." traversal, so the
		// join below can never escape dir even if sanitizeMaintenanceFileName
		// were bypassed.
		if !filepath.IsLocal(name) {
			continue
		}
		candidate := filepath.Join(dir, name)
		// The maintenance directory lives next to the nginx configuration, so
		// in host_via_ssh with SFTP access it is on the host rather than in
		// the container. nginx.ReadFile follows the configured target.
		if content, err := nginx.ReadFile(candidate); err == nil && len(content) > 0 {
			return content
		}
	}

	return nil
}

// maintenanceFileNamePattern allows only characters that are safe in a single
// path segment on every supported OS, closing off traversal and NUL-byte tricks.
var maintenanceFileNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// sanitizeMaintenanceFileName reduces the input to a single, safe path element so
// that neither the configured template name nor a spoofed site header can escape
// the maintenance directory.
func sanitizeMaintenanceFileName(name string) string {
	name = strings.TrimSpace(name)
	name = name[strings.LastIndexAny(name, `/\`)+1:]
	if name == "." || name == ".." || !maintenanceFileNamePattern.MatchString(name) {
		return ""
	}
	return name
}
