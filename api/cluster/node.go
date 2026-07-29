package cluster

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/api/audit"
	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalCluster "github.com/0xJacky/Nginx-UI/internal/cluster"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
)

type nodeMutationRequest struct {
	Name         string  `json:"name" binding:"required"`
	URL          string  `json:"url" binding:"required"`
	Enabled      bool    `json:"enabled"`
	LegacySecret *string `json:"legacy_secret"`
	Token        *string `json:"token"`
}

type nodeResponse struct {
	ID                  uint64     `json:"id"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	Name                string     `json:"name"`
	URL                 string     `json:"url"`
	Enabled             bool       `json:"enabled"`
	AuthMethod          string     `json:"auth_method"`
	HasCredential       bool       `json:"has_credential"`
	CredentialStatus    string     `json:"credential_status"`
	LastCredentialUseAt *time.Time `json:"last_credential_use_at,omitempty"`
	// LegacySecret only ever carries the redaction sentinel, which tells the
	// edit form a secret is stored without putting it in a list response.
	LegacySecret string `json:"legacy_secret,omitempty"`
	analytic.NodeStat
	analytic.NodeInfo
}

func newNodeResponse(node *model.Node) nodeResponse {
	analyticNode := analytic.GetNode(node)
	response := nodeResponse{
		ID:                  node.ID,
		CreatedAt:           node.CreatedAt,
		UpdatedAt:           node.UpdatedAt,
		Name:                node.Name,
		URL:                 node.URL,
		Enabled:             node.Enabled,
		AuthMethod:          node.AuthMethod,
		HasCredential:       node.HasCredential(),
		CredentialStatus:    node.CredentialStatus,
		LastCredentialUseAt: node.LastCredentialUseAt,
	}
	if len(node.EncryptedLegacySecret) != 0 {
		response.LegacySecret = settings.RedactedSensitiveValue
	}
	if analyticNode != nil {
		response.NodeStat = analyticNode.NodeStat
		response.NodeInfo = analyticNode.NodeInfo
	}
	return response
}

func GetNode(c *gin.Context) {
	node, ok := findNode(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, newNodeResponse(node))
}

func GetNodeList(c *gin.Context) {
	core := cosy.Core[model.Node](c).SetFussy("name")
	if c.Query("enabled") != "" {
		core.GormScope(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("enabled = ?", cast.ToInt(cast.ToBool(c.Query("enabled"))))
		})
	}
	core.SetTransformer(func(node *model.Node) any {
		return newNodeResponse(node)
	})
	core.List()
}

func AddNode(c *gin.Context) {
	audit.MarkSensitiveRequest(c)
	var request nodeMutationRequest
	if !cosy.BindAndValid(c, &request) {
		return
	}
	normalizedURL, err := validateNodeURL(request.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	legacySecret := mutationLegacySecret(request)
	authMethod := model.NodeAuthMethodPaired
	credentialStatus := model.NodeCredentialStatusUnpaired
	if legacySecret != "" {
		authMethod = model.NodeAuthMethodLegacy
		credentialStatus = model.NodeCredentialStatusActive
	}
	node := &model.Node{
		Name:             request.Name,
		URL:              normalizedURL,
		Enabled:          request.Enabled,
		AuthMethod:       authMethod,
		CredentialStatus: credentialStatus,
	}
	database := model.UseDB()
	err = database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(node).Error; err != nil {
			return err
		}
		if legacySecret == "" {
			return nil
		}
		encrypted, err := nodeauth.EncryptPrivateCredential(
			nodeauth.LegacyCredentialPurpose(node.ID),
			[]byte(legacySecret),
		)
		if err != nil {
			return err
		}
		node.EncryptedLegacySecret = encrypted
		return tx.Model(node).Update("encrypted_legacy_secret", encrypted).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	refreshNodeState()
	c.JSON(http.StatusCreated, newNodeResponse(node))
}

func EditNode(c *gin.Context) {
	audit.MarkSensitiveRequest(c)
	node, ok := findNode(c)
	if !ok {
		return
	}
	var request nodeMutationRequest
	if !cosy.BindAndValid(c, &request) {
		return
	}
	normalizedURL, err := validateNodeURL(request.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	updates := map[string]any{
		"name":    request.Name,
		"url":     normalizedURL,
		"enabled": request.Enabled,
	}
	legacySecret := mutationLegacySecret(request)
	database := model.UseDB()
	err = database.Transaction(func(tx *gorm.DB) error {
		if legacySecret != "" {
			encrypted, err := nodeauth.EncryptPrivateCredential(
				nodeauth.LegacyCredentialPurpose(node.ID),
				[]byte(legacySecret),
			)
			if err != nil {
				return err
			}
			updates["token"] = ""
			updates["encrypted_legacy_secret"] = encrypted
			updates["auth_method"] = model.NodeAuthMethodLegacy
			updates["credential_status"] = model.NodeCredentialStatusActive
			if err := tx.Unscoped().Where("node_id = ?", node.ID).Delete(&model.NodeCredential{}).Error; err != nil {
				return err
			}
		}
		return tx.Model(node).Updates(updates).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if err := database.First(node, node.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	refreshNodeState()
	c.JSON(http.StatusOK, newNodeResponse(node))
}

func DeleteNode(c *gin.Context) {
	node, ok := findNode(c)
	if !ok {
		return
	}
	err := model.UseDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("node_id = ?", node.ID).Delete(&model.NodeCredential{}).Error; err != nil {
			return err
		}
		return tx.Delete(node).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	refreshNodeState()
	c.Status(http.StatusNoContent)
}

func LoadNodeFromSettings(c *gin.Context) {
	if err := settings.ReloadCluster(); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	internalCluster.RegisterPredefinedNodes(context.Background())
	refreshNodeState()
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func findNode(c *gin.Context) (*model.Node, bool) {
	id := cast.ToUint64(c.Param("id"))
	if id == 0 || model.UseDB() == nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "node not found"})
		return nil, false
	}
	var node model.Node
	if err := model.UseDB().First(&node, id).Error; err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"message": err.Error()})
		return nil, false
	}
	return &node, true
}

// GetNodeSecret reveals the shared secret stored for a node so an administrator
// can read or copy it back out. Revealing it costs a verified secure session,
// the same bar protected settings are held to — the route's own middleware only
// demands one from users who have two-factor authentication enabled.
func GetNodeSecret(c *gin.Context) {
	if verified, _ := c.Get(middleware.SecureSessionVerifiedKey); verified != true {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"message": "Two-factor authentication is required to reveal the node secret",
		})
		return
	}
	node, ok := findNode(c)
	if !ok {
		return
	}
	if len(node.EncryptedLegacySecret) == 0 {
		c.JSON(http.StatusOK, gin.H{"value": ""})
		return
	}
	secret, err := nodeauth.DecryptPrivateCredential(
		nodeauth.LegacyCredentialPurpose(node.ID),
		node.EncryptedLegacySecret,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	audit.MarkSensitiveResponse(c)
	c.JSON(http.StatusOK, gin.H{"value": string(secret)})
}

func refreshNodeState() {
	cache.InvalidateNodeCache()
	analytic.ReloadNodesStatus()
}

// mutationLegacySecret reads the submitted secret, treating the redaction
// sentinel the edit form echoes back as "keep the stored value" rather than as
// a literal new secret.
func mutationLegacySecret(request nodeMutationRequest) string {
	if request.LegacySecret != nil {
		return sanitizeSubmittedSecret(*request.LegacySecret)
	}
	if request.Token != nil {
		return sanitizeSubmittedSecret(*request.Token)
	}
	return ""
}

func sanitizeSubmittedSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == settings.RedactedSensitiveValue {
		return ""
	}
	return value
}

func validateNodeURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return "", errors.New("invalid node URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("node URL must use HTTP or HTTPS")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("node URL must not contain credentials, query parameters, or fragments")
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.RawPath = strings.TrimSuffix(parsed.RawPath, "/")
	return parsed.String(), nil
}
