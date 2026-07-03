package mcp

import (
	"bytes"
	"encoding/json"
	"io"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

var sensitiveMCPTools = map[string]struct{}{
	"nginx_config_add":    {},
	"nginx_config_enable": {},
	"nginx_config_mkdir":  {},
	"nginx_config_modify": {},
	"nginx_config_rename": {},
	"reload_nginx":        {},
	"restart_nginx":       {},
}

type mcpToolCallProbe struct {
	Method string `json:"method"`
	Params struct {
		Name string `json:"name"`
	} `json:"params"`
}

func InitRouter(r *gin.Engine) {
	r.Any("/mcp", middleware.IPWhiteList(), middleware.AuthRequired(), requireSecureSessionForSensitiveMCPTools(),
		func(c *gin.Context) {
			internalmcp.ServeHTTP(c)
		})
	r.Any("/mcp_message", middleware.IPWhiteList(), middleware.AuthRequired(), requireSecureSessionForSensitiveMCPTools(),
		func(c *gin.Context) {
			internalmcp.ServeHTTP(c)
		})
}

func requireSecureSessionForSensitiveMCPTools() gin.HandlerFunc {
	requireSecureSession := middleware.RequireSecureSession()

	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(400, gin.H{
				"message": "Failed to read request body",
			})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		if !mcpRequestNeedsSecureSession(body) {
			c.Next()
			return
		}

		requireSecureSession(c)
	}
}

func mcpRequestNeedsSecureSession(body []byte) bool {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return false
	}

	if body[0] == '[' {
		var messages []mcpToolCallProbe
		if err := json.Unmarshal(body, &messages); err != nil {
			return false
		}

		for _, message := range messages {
			if mcpMessageNeedsSecureSession(message) {
				return true
			}
		}
		return false
	}

	var message mcpToolCallProbe
	if err := json.Unmarshal(body, &message); err != nil {
		return false
	}

	return mcpMessageNeedsSecureSession(message)
}

func mcpMessageNeedsSecureSession(message mcpToolCallProbe) bool {
	if message.Method != "tools/call" {
		return false
	}

	_, ok := sensitiveMCPTools[message.Params.Name]
	return ok
}
