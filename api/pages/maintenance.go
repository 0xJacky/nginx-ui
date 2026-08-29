package pages

import (
	"embed"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	candidates := make([]string, 0, 2)
	if site := sanitizeMaintenanceFileName(siteName); site != "" {
		candidates = append(candidates, filepath.Join(dir, site+"."+templateName))
	}
	candidates = append(candidates, filepath.Join(dir, templateName))

	for _, candidate := range candidates {
		if content, err := os.ReadFile(candidate); err == nil && len(content) > 0 {
			return content
		}
	}

	return nil
}

// sanitizeMaintenanceFileName reduces the input to a single path element so that
// neither the configured template name nor a spoofed site header can escape the
// maintenance directory.
func sanitizeMaintenanceFileName(name string) string {
	name = strings.TrimSpace(name)
	name = name[strings.LastIndexAny(name, `/\`)+1:]
	if name == "." || name == ".." || strings.ContainsRune(name, 0) {
		return ""
	}
	return name
}
