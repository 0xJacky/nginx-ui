package cluster

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/api/audit"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const credentialRotationRecoveryGrace = 10 * time.Minute

type upgradeLegacyPairingRequest struct {
	ControllerInstanceID string `json:"controller_instance_id" binding:"required"`
	PublicKey            string `json:"public_key" binding:"required"`
}

type upgradeLegacyPairingResponse struct {
	CredentialID     string `json:"credential_id"`
	TargetInstanceID string `json:"target_instance_id"`
	Confirmation     string `json:"confirmation"`
}

type rotateCredentialRequest struct {
	PublicKey string `json:"public_key" binding:"required"`
}

type nodeCredentialResponse struct {
	CredentialID       string     `json:"credential_id"`
	TargetInstanceID   string     `json:"target_instance_id"`
	Status             string     `json:"status"`
	LastUsedAt         *time.Time `json:"last_used_at,omitempty"`
	PreviousValidUntil *time.Time `json:"previous_valid_until,omitempty"`
}

type controllerCredentialResponse struct {
	CredentialID         string     `json:"credential_id"`
	ControllerInstanceID string     `json:"controller_instance_id"`
	Status               string     `json:"status"`
	LastUsedAt           *time.Time `json:"last_used_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	RevokedAt            *time.Time `json:"revoked_at,omitempty"`
	PreviousValidUntil   *time.Time `json:"previous_valid_until,omitempty"`
}

// UpgradeLegacyPairing migrates a controller that still authenticates with the
// shared node secret onto its own key pair.
//
// The request is authenticated by the middleware like any other node request,
// which for a shared-secret controller means a signature derived from that
// secret — knowledge of the secret is therefore already proven, along with the
// nonce and freshness the signature envelope carries. What this handler adds is
// the return direction: it signs the credential it issues so the controller can
// tell it reached the node that holds the same secret.
func UpgradeLegacyPairing(c *gin.Context) {
	audit.MarkSensitiveRequest(c)
	principal, ok := nodePrincipal(c)
	if !ok || principal.AuthMethod != model.NodeAuthMethodLegacy {
		c.JSON(http.StatusForbidden, gin.H{"message": "legacy node authentication is required"})
		return
	}
	secret := strings.TrimSpace(settings.NodeSettings.Secret)
	if secret == "" {
		c.JSON(http.StatusForbidden, gin.H{"message": "node secret is not configured"})
		return
	}
	var request upgradeLegacyPairingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if _, err := uuid.Parse(request.ControllerInstanceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid controller instance ID"})
		return
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(request.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid Ed25519 public key"})
		return
	}

	database := model.UseDB()
	if database == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "database unavailable"})
		return
	}
	credentialID := uuid.NewString()
	now := time.Now()
	err = database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.NodeControllerCredential{}).
			Where("controller_instance_id = ? AND revoked_at IS NULL", request.ControllerInstanceID).
			Updates(map[string]any{
				"revoked_at": now,
				"status":     model.NodeCredentialStatusRevoked,
			}).Error; err != nil {
			return err
		}
		return tx.Create(&model.NodeControllerCredential{
			CredentialID:         credentialID,
			ControllerInstanceID: request.ControllerInstanceID,
			PublicKey:            append([]byte(nil), publicKey...),
			Status:               model.NodeCredentialStatusActive,
		}).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	confirmation, err := nodeauth.SignUpgradeConfirmation(
		[]byte(secret), request.ControllerInstanceID, publicKey, credentialID, settings.NodeSettings.InstanceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, upgradeLegacyPairingResponse{
		CredentialID:     credentialID,
		TargetInstanceID: settings.NodeSettings.InstanceID,
		Confirmation:     confirmation,
	})
}

func GetNodeCredentials(c *gin.Context) {
	node, ok := findNode(c, false)
	if !ok {
		return
	}
	var credential model.NodeCredential
	err := model.UseDB().Where("node_id = ?", node.ID).First(&credential).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, []nodeCredentialResponse{})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, []nodeCredentialResponse{{
		CredentialID:       credential.CredentialID,
		TargetInstanceID:   credential.TargetInstanceID,
		Status:             credential.Status,
		LastUsedAt:         credential.LastUsedAt,
		PreviousValidUntil: credential.PreviousValidUntil,
	}})
}

func ListControllerCredentials(c *gin.Context) {
	var credentials []model.NodeControllerCredential
	if err := model.UseDB().Unscoped().Order("created_at DESC").Find(&credentials).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	result := make([]controllerCredentialResponse, 0, len(credentials))
	for _, credential := range credentials {
		result = append(result, controllerCredentialResponse{
			CredentialID:         credential.CredentialID,
			ControllerInstanceID: credential.ControllerInstanceID,
			Status:               credential.Status,
			LastUsedAt:           credential.LastUsedAt,
			CreatedAt:            credential.CreatedAt,
			RevokedAt:            credential.RevokedAt,
			PreviousValidUntil:   credential.PreviousValidUntil,
		})
	}
	c.JSON(http.StatusOK, result)
}

func RevokeControllerCredential(c *gin.Context) {
	credentialID := c.Param("credential_id")
	now := time.Now()
	result := model.UseDB().Model(&model.NodeControllerCredential{}).
		Where("credential_id = ? AND revoked_at IS NULL", credentialID).
		Updates(map[string]any{
			"revoked_at": now,
			"status":     model.NodeCredentialStatusRevoked,
		})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": result.Error.Error()})
		return
	}
	if result.RowsAffected != 1 {
		c.JSON(http.StatusNotFound, gin.H{"message": "credential not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func BeginControllerCredentialRotation(c *gin.Context) {
	principal, ok := nodePrincipal(c)
	if !ok || principal.CredentialID != c.Param("credential_id") {
		c.JSON(http.StatusForbidden, gin.H{"message": "credential does not own this rotation"})
		return
	}
	var request rotateCredentialRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(request.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid Ed25519 public key"})
		return
	}
	result := model.UseDB().Model(&model.NodeControllerCredential{}).
		Where("credential_id = ? AND revoked_at IS NULL", principal.CredentialID).
		Updates(map[string]any{
			"pending_public_key": publicKey,
			"status":             model.NodeCredentialStatusRotating,
		})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": result.Error.Error()})
		return
	}
	if result.RowsAffected != 1 {
		c.JSON(http.StatusNotFound, gin.H{"message": "credential not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func ConfirmControllerCredentialRotation(c *gin.Context) {
	principal, ok := nodePrincipal(c)
	if !ok || principal.CredentialID != c.Param("credential_id") {
		c.JSON(http.StatusForbidden, gin.H{"message": "credential does not own this rotation"})
		return
	}
	database := model.UseDB()
	err := database.Transaction(func(tx *gorm.DB) error {
		var credential model.NodeControllerCredential
		if err := tx.Where("credential_id = ? AND revoked_at IS NULL", principal.CredentialID).First(&credential).Error; err != nil {
			return err
		}
		if len(credential.PendingPublicKey) == 0 {
			if credential.Status == model.NodeCredentialStatusActive {
				return nil
			}
			return errors.New("credential has no pending rotation")
		}
		if len(credential.PendingPublicKey) != ed25519.PublicKeySize {
			return errors.New("credential pending key is invalid")
		}
		previousValidUntil := time.Now().Add(credentialRotationRecoveryGrace)
		return tx.Model(&credential).Updates(map[string]any{
			"previous_public_key":  credential.PublicKey,
			"previous_valid_until": previousValidUntil,
			"public_key":           credential.PendingPublicKey,
			"pending_public_key":   nil,
			"status":               model.NodeCredentialStatusActive,
		}).Error
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func RotateNodeCredential(c *gin.Context) {
	node, ok := findNode(c, false)
	if !ok {
		return
	}
	result, err := nodeauth.RotateRelationshipCredential(c.Request.Context(), node)
	if err != nil {
		if errors.Is(err, nodeauth.ErrNodeNotPaired) {
			c.JSON(http.StatusConflict, gin.H{"message": err.Error()})
			return
		}
		if errors.Is(err, nodeauth.ErrCredentialMissing) {
			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nodeCredentialResponse{
		CredentialID:       result.CredentialID,
		TargetInstanceID:   result.TargetInstanceID,
		Status:             model.NodeCredentialStatusActive,
		PreviousValidUntil: &result.PreviousValidUntil,
	})
}

func nodePrincipal(c *gin.Context) (*nodeauth.Principal, bool) {
	value, ok := c.Get(nodeauth.GinPrincipalKey)
	if !ok {
		return nil, false
	}
	principal, ok := value.(*nodeauth.Principal)
	return principal, ok && principal != nil
}
