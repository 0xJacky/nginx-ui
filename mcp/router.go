package mcp

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
)

var writeMCPTools = map[string]struct{}{
	"nginx_config_add":    {},
	"nginx_config_enable": {},
	"nginx_config_mkdir":  {},
	"nginx_config_modify": {},
	"nginx_config_rename": {},
	"reload_nginx":        {},
	"restart_nginx":       {},
}

var readOnlyMCPTools = map[string]struct{}{
	"nginx_config_base_path": {},
	"nginx_config_get":       {},
	"nginx_config_history":   {},
	"nginx_config_list":      {},
	"nginx_status":           {},
}

var errInvalidMCPRequestBody = errors.New("invalid MCP request body")

type mcpToolCallProbe struct {
	Method string `json:"method"`
	Params struct {
		Name string `json:"name"`
	} `json:"params"`
}

func InitRouter(r *gin.Engine) {
	r.Any("/mcp", middleware.IPWhiteList(), mcpAuthRequired(), authorizeMCPToolRequest(),
		func(c *gin.Context) {
			internalmcp.ServeHTTP(c)
		})
	r.Any("/mcp_message", middleware.IPWhiteList(), mcpAuthRequired(), authorizeMCPToolRequest(),
		func(c *gin.Context) {
			internalmcp.ServeHTTP(c)
		})
}

func mcpAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Query().Has("node_secret") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Authorization failed"})
			return
		}

		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		token := authorization
		if strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
			token = strings.TrimSpace(authorization[len("Bearer "):])
		}
		if strings.HasPrefix(token, "nui_pat_") {
			principal, err := internalmcp.VerifyServiceToken(token, time.Now())
			if err != nil {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Authorization failed"})
				return
			}
			c.Set(internalmcp.ServiceTokenPrincipalKey, principal)
			c.Next()
			return
		}
		if token != "" {
			var (
				currentUser *model.User
				ok          bool
			)
			if len(token) <= 16 {
				currentUser, ok = user.GetTokenUserByShortToken(token)
			} else {
				currentUser, ok = user.GetTokenUser(token)
			}
			if ok {
				c.Set("user", currentUser)
				c.Next()
				return
			}
		}

		legacySecret := strings.TrimSpace(c.GetHeader("X-Node-Secret"))
		configuredSecret := settings.NodeSettings.Secret
		if legacySecret != "" && configuredSecret != "" &&
			len(legacySecret) == len(configuredSecret) &&
			subtle.ConstantTimeCompare([]byte(legacySecret), []byte(configuredSecret)) == 1 {
			principal := &nodeauth.Principal{
				CredentialID:         "legacy",
				ControllerInstanceID: "legacy",
				AuthMethod:           model.NodeAuthMethodLegacy,
			}
			c.Set(nodeauth.GinPrincipalKey, principal)
			c.Set("user", user.GetInitUser(c))
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Authorization failed"})
	}
}

func authorizeMCPToolRequest() gin.HandlerFunc {
	requireSecureSession := middleware.RequireSecureSession()

	return func(c *gin.Context) {
		var body []byte
		if c.Request.Body != nil {
			var err error
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"message": "Failed to read request body",
				})
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}

		requiredScope := model.MCPTokenScopeRead
		if c.Request.Method == http.MethodPost {
			var err error
			requiredScope, err = classifyMCPRequest(body)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"message": "Invalid MCP request body",
				})
				return
			}
		}
		if value, ok := c.Get(internalmcp.ServiceTokenPrincipalKey); ok {
			principal, valid := value.(*internalmcp.ServiceTokenPrincipal)
			if !valid || !principal.HasScope(requiredScope) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "MCP token scope is insufficient"})
				return
			}
			c.Next()
			return
		}

		if requiredScope == model.MCPTokenScopeWrite {
			requireSecureSession(c)
			return
		}
		c.Next()
	}
}

func classifyMCPRequest(body []byte) (string, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || body[0] != '{' {
		return "", errInvalidMCPRequestBody
	}

	var message mcpToolCallProbe
	if err := json.Unmarshal(body, &message); err != nil {
		return "", errInvalidMCPRequestBody
	}

	if message.Method != "tools/call" {
		return model.MCPTokenScopeRead, nil
	}

	if _, ok := readOnlyMCPTools[message.Params.Name]; ok {
		return model.MCPTokenScopeRead, nil
	}
	if _, ok := writeMCPTools[message.Params.Name]; ok {
		return model.MCPTokenScopeWrite, nil
	}

	// Unknown tools fail closed. This keeps newly added mutating tools behind
	// write scope and secure-session checks until they are explicitly classified.
	return model.MCPTokenScopeWrite, nil
}
