package cluster

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/api/audit"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/transport"
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

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	payload := completePairingRequest{
		Code:                 request.Code,
		ControllerInstanceID: settings.NodeSettings.InstanceID,
		PublicKey:            base64.RawURLEncoding.EncodeToString(publicKey),
	}
	var response completePairingResponse
	if err := sendNodeJSON(node, http.MethodPost, "/api/node/pair/complete", payload, &response, false); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	if _, err := uuid.Parse(response.CredentialID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "paired node returned an invalid credential ID"})
		return
	}
	if _, err := uuid.Parse(response.TargetInstanceID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "paired node returned an invalid instance ID"})
		return
	}

	encryptedPrivateKey, err := nodeauth.EncryptPrivateCredential(
		nodeauth.SigningCredentialPurpose(response.CredentialID),
		privateKey,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	database := model.UseDB()
	err = database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("node_id = ?", node.ID).Delete(&model.NodeCredential{}).Error; err != nil {
			return err
		}
		credential := &model.NodeCredential{
			NodeID:              node.ID,
			CredentialID:        response.CredentialID,
			TargetInstanceID:    response.TargetInstanceID,
			PublicKey:           append([]byte(nil), publicKey...),
			EncryptedPrivateKey: encryptedPrivateKey,
			Status:              model.NodeCredentialStatusActive,
		}
		if err := tx.Create(credential).Error; err != nil {
			return err
		}
		return tx.Model(&model.Node{}).Where("id = ?", node.ID).Updates(map[string]any{
			"token":                   "",
			"encrypted_legacy_secret": nil,
			"auth_method":             model.NodeAuthMethodPaired,
			"credential_status":       model.NodeCredentialStatusActive,
		}).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	refreshNodeState()
	c.JSON(http.StatusOK, nodeCredentialResponse{
		CredentialID:     response.CredentialID,
		TargetInstanceID: response.TargetInstanceID,
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
	if node.AuthMethod != model.NodeAuthMethodPaired {
		c.JSON(http.StatusConflict, gin.H{"message": "node is not using paired authentication"})
		return
	}
	database := model.UseDB()
	var credential model.NodeCredential
	if err := database.Where("node_id = ? AND revoked_at IS NULL", node.ID).First(&credential).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "paired credential not found"})
		return
	}

	if len(credential.EncryptedPendingPrivateKey) == 0 {
		publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		encrypted, err := nodeauth.EncryptPrivateCredential(nodeauth.SigningCredentialPurpose(credential.CredentialID), privateKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		payload := rotateCredentialRequest{PublicKey: base64.RawURLEncoding.EncodeToString(publicKey)}
		path := "/api/node/credentials/" + url.PathEscape(credential.CredentialID) + "/rotation"
		if err := sendNodeJSON(node, http.MethodPost, path, payload, nil, true); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
			return
		}
		if err := database.Model(&credential).Updates(map[string]any{
			"pending_public_key":            publicKey,
			"encrypted_pending_private_key": encrypted,
			"status":                        model.NodeCredentialStatusRotating,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		credential.PendingPublicKey = publicKey
		credential.EncryptedPendingPrivateKey = encrypted
	}

	confirmPath := "/api/node/credentials/" + url.PathEscape(credential.CredentialID) + "/rotation/confirm"
	if err := sendNodeJSON(node, http.MethodPost, confirmPath, nil, nil, true); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	previousValidUntil := time.Now().Add(credentialRotationRecoveryGrace)
	err := database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.NodeCredential{}).Where("id = ?", credential.ID).Updates(map[string]any{
			"previous_public_key":            credential.PublicKey,
			"encrypted_previous_private_key": credential.EncryptedPrivateKey,
			"previous_valid_until":           previousValidUntil,
			"public_key":                     credential.PendingPublicKey,
			"encrypted_private_key":          credential.EncryptedPendingPrivateKey,
			"pending_public_key":             nil,
			"encrypted_pending_private_key":  nil,
			"status":                         model.NodeCredentialStatusActive,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Node{}).Where("id = ?", node.ID).
			Update("credential_status", model.NodeCredentialStatusActive).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nodeCredentialResponse{
		CredentialID:       credential.CredentialID,
		TargetInstanceID:   credential.TargetInstanceID,
		Status:             model.NodeCredentialStatusActive,
		PreviousValidUntil: &previousValidUntil,
	})
}

func sendNodeJSON(node *model.Node, method, path string, payload, response any, authenticate bool) error {
	target, err := node.GetUrl(path)
	if err != nil {
		return err
	}
	var body io.Reader
	if payload != nil {
		content, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(content)
	}
	request, err := http.NewRequest(method, target, body)
	if err != nil {
		return err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	baseTransport, err := transport.NewTransport()
	if err != nil {
		return err
	}
	roundTripper := http.RoundTripper(baseTransport)
	if authenticate {
		roundTripper = nodeauth.NewTransport(node, roundTripper)
	}
	client := &http.Client{Transport: roundTripper, Timeout: 15 * time.Second}
	httpResponse, err := client.Do(request)
	if err != nil {
		return err
	}
	defer httpResponse.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		return err
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return errors.New(strings.TrimSpace(string(responseBody)))
	}
	if response != nil && len(responseBody) != 0 {
		if err := json.Unmarshal(responseBody, response); err != nil {
			return err
		}
	}
	return nil
}

func requireSecurePairingURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return errors.New("invalid node URL")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme != "http" {
		return errors.New("pairing requires an HTTPS node URL")
	}
	hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if hostname == "localhost" {
		return nil
	}
	ip := net.ParseIP(hostname)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	return errors.New("remote pairing requires HTTPS")
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
