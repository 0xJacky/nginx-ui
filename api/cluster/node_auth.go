package cluster

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/api/audit"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	pairingCodeLifetime             = 10 * time.Minute
	credentialRotationRecoveryGrace = 10 * time.Minute
)

type pairingCodeResponse struct {
	Code       string    `json:"code"`
	InstanceID string    `json:"instance_id"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type completePairingRequest struct {
	Code                 string `json:"code" binding:"required"`
	ControllerInstanceID string `json:"controller_instance_id" binding:"required"`
	PublicKey            string `json:"public_key" binding:"required"`
}

type completePairingResponse struct {
	CredentialID     string `json:"credential_id"`
	TargetInstanceID string `json:"target_instance_id"`
}

type upgradeLegacyPairingRequest struct {
	ControllerInstanceID string `json:"controller_instance_id" binding:"required"`
	PublicKey            string `json:"public_key" binding:"required"`
}

type pairNodeRequest struct {
	Code string `json:"code" binding:"required"`
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

func CreatePairingCode(c *gin.Context) {
	codeBytes := make([]byte, 16)
	if _, err := rand.Read(codeBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to generate pairing code"})
		return
	}
	code := base64.RawURLEncoding.EncodeToString(codeBytes)
	codeHash := sha256.Sum256([]byte(code))
	expiresAt := time.Now().Add(pairingCodeLifetime)
	createdBy := currentUserID(c)

	database := model.UseDB()
	if database == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "database unavailable"})
		return
	}
	_ = database.Where("expires_at < ? OR used_at IS NOT NULL", time.Now()).Delete(&model.NodePairingCode{}).Error
	record := &model.NodePairingCode{
		CodeHash:  codeHash[:],
		ExpiresAt: expiresAt,
		CreatedBy: createdBy,
	}
	if err := database.Create(record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	audit.MarkSensitiveResponse(c)
	c.JSON(http.StatusCreated, pairingCodeResponse{
		Code:       code,
		InstanceID: settings.NodeSettings.InstanceID,
		ExpiresAt:  expiresAt,
	})
}

func CompletePairing(c *gin.Context) {
	audit.MarkSensitiveRequest(c)
	var request completePairingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	codeBytes, err := base64.RawURLEncoding.DecodeString(request.Code)
	if err != nil || len(codeBytes) != 16 {
		c.JSON(http.StatusForbidden, gin.H{"message": "invalid or expired pairing code"})
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
	now := time.Now()
	codeHash := sha256.Sum256([]byte(request.Code))
	credentialID := uuid.NewString()
	err = database.Transaction(func(tx *gorm.DB) error {
		var pairingCode model.NodePairingCode
		if err := tx.Where("code_hash = ? AND used_at IS NULL AND expires_at > ?", codeHash[:], now).
			First(&pairingCode).Error; err != nil {
			return errors.New("invalid or expired pairing code")
		}
		result := tx.Model(&model.NodePairingCode{}).
			Where("id = ? AND used_at IS NULL", pairingCode.ID).
			Update("used_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("pairing code was already used")
		}

		credential := &model.NodeControllerCredential{
			CredentialID:         credentialID,
			ControllerInstanceID: request.ControllerInstanceID,
			PublicKey:            append([]byte(nil), publicKey...),
			Status:               model.NodeCredentialStatusActive,
		}
		return tx.Create(credential).Error
	})
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, completePairingResponse{
		CredentialID:     credentialID,
		TargetInstanceID: settings.NodeSettings.InstanceID,
	})
}

func UpgradeLegacyPairing(c *gin.Context) {
	principal, ok := nodePrincipal(c)
	if !ok || principal.AuthMethod != model.NodeAuthMethodLegacy {
		c.JSON(http.StatusForbidden, gin.H{"message": "legacy node authentication is required"})
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
	c.JSON(http.StatusCreated, completePairingResponse{
		CredentialID:     credentialID,
		TargetInstanceID: settings.NodeSettings.InstanceID,
	})
}

func PairNode(c *gin.Context) {
	audit.MarkSensitiveRequest(c)
	node, ok := findNode(c)
	if !ok {
		return
	}
	var request pairNodeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if err := requireSecurePairingURL(node.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	result, err := nodeauth.PairNode(c.Request.Context(), node, request.Code, settings.NodeSettings.InstanceID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	refreshNodeState()
	c.JSON(http.StatusOK, nodeCredentialResponse{
		CredentialID:     result.CredentialID,
		TargetInstanceID: result.TargetInstanceID,
		Status:           model.NodeCredentialStatusActive,
	})
}

func GetNodeCredentials(c *gin.Context) {
	node, ok := findNode(c)
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
	node, ok := findNode(c)
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

func requireSecurePairingURL(rawURL string) error {
	return nodeauth.RequireSecurePairingURL(rawURL)
}

func nodePrincipal(c *gin.Context) (*nodeauth.Principal, bool) {
	value, ok := c.Get(nodeauth.GinPrincipalKey)
	if !ok {
		return nil, false
	}
	principal, ok := value.(*nodeauth.Principal)
	return principal, ok && principal != nil
}

func currentUserID(c *gin.Context) uint64 {
	value, ok := c.Get("user")
	if !ok {
		return 0
	}
	user, ok := value.(*model.User)
	if !ok {
		return 0
	}
	return user.ID
}
