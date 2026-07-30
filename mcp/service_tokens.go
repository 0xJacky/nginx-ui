package mcp

import (
	"errors"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/api/audit"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createServiceTokenRequest struct {
	Name      string     `json:"name" binding:"required"`
	Scopes    []string   `json:"scopes" binding:"required,min=1"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type serviceTokenResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatorID  uint64     `json:"creator_id"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	Token      string     `json:"token,omitempty"`
}

func InitManagementRouter(r *gin.RouterGroup) {
	initServiceTokenManagementRouter(r.Group("service_tokens"))
	// Keep the original endpoint as a compatibility alias for existing clients.
	initServiceTokenManagementRouter(r.Group("mcp/tokens"))
}

func initServiceTokenManagementRouter(group *gin.RouterGroup) {
	admin := group.Group("", middleware.RequireInteractiveUser())
	admin.GET("", ListServiceTokens)
	mutations := admin.Group("", middleware.RequireSecureSession())
	mutations.POST("", CreateServiceToken)
	mutations.POST("/:id/rotate", RotateServiceToken)
	mutations.DELETE("/:id", RevokeServiceToken)
}

func ListServiceTokens(c *gin.Context) {
	var records []model.MCPServiceToken
	if err := model.UseDB().Order("created_at DESC").Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	result := make([]serviceTokenResponse, 0, len(records))
	for _, record := range records {
		result = append(result, newServiceTokenResponse(&record, ""))
	}
	c.JSON(http.StatusOK, result)
}

func CreateServiceToken(c *gin.Context) {
	var request createServiceTokenRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	record, rawToken, err := internalmcp.CreateServiceToken(
		request.Name,
		request.Scopes,
		request.ExpiresAt,
		currentMCPUserID(c),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	audit.MarkSensitiveResponse(c)
	c.JSON(http.StatusCreated, newServiceTokenResponse(record, rawToken))
}

func RotateServiceToken(c *gin.Context) {
	record, rawToken, err := internalmcp.RotateServiceToken(c.Param("id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"message": err.Error()})
		return
	}
	audit.MarkSensitiveResponse(c)
	c.JSON(http.StatusOK, newServiceTokenResponse(record, rawToken))
}

func RevokeServiceToken(c *gin.Context) {
	err := internalmcp.RevokeServiceToken(c.Param("id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"message": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func newServiceTokenResponse(record *model.MCPServiceToken, rawToken string) serviceTokenResponse {
	return serviceTokenResponse{
		ID:         record.PublicID,
		Name:       record.Name,
		Scopes:     append([]string(nil), record.Scopes...),
		ExpiresAt:  record.ExpiresAt,
		RevokedAt:  record.RevokedAt,
		CreatorID:  record.CreatorID,
		LastUsedAt: record.LastUsedAt,
		CreatedAt:  record.CreatedAt,
		Token:      rawToken,
	}
}

func currentMCPUserID(c *gin.Context) uint64 {
	value, ok := c.Get("user")
	if !ok {
		return 0
	}
	currentUser, ok := value.(*model.User)
	if !ok {
		return 0
	}
	return currentUser.ID
}
