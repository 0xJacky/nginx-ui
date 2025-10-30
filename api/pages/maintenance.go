package pages

import (
	"embed"
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
)

//go:embed *.tmpl
var tmplFS embed.FS

const maintenanceMountDir = "/etc/nginx/maintenance"

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

	// Try custom mounted HTML first (NGINX_UI_NGINX_MAINTENANCE_TEMPLATE)
	if name := strings.TrimSpace(settings.NginxSettings.MaintenanceTemplate); name != "" {
		name = filepath.Base(name)
		full := filepath.Join(maintenanceMountDir, name)

		if b, err := os.ReadFile(full); err == nil && len(b) > 0 {
			c.Data(http.StatusServiceUnavailable, "text/html; charset=utf-8", b)
			return
		}
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
