package nodeauth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	credentialRotationRecoveryGrace = 10 * time.Minute
	automaticCredentialRotationAge  = 90 * 24 * time.Hour
)

var (
	ErrNodeNotPaired     = errors.New("node is not using paired authentication")
	ErrCredentialMissing = errors.New("paired credential not found")
)

type PairingResult struct {
	CredentialID     string
	TargetInstanceID string
}

type RotationResult struct {
	CredentialID       string
	TargetInstanceID   string
	PreviousValidUntil time.Time
}

type MaintenanceIssue struct {
	NodeID    uint64
	Operation string
	Err       error
}

type pairingRequest struct {
	Code                 string `json:"code,omitempty"`
	ControllerInstanceID string `json:"controller_instance_id"`
	PublicKey            string `json:"public_key"`
}

type pairingResponse struct {
	CredentialID     string `json:"credential_id"`
	TargetInstanceID string `json:"target_instance_id"`
}

type rotationRequest struct {
	PublicKey string `json:"public_key"`
}

func PairNode(ctx context.Context, node *model.Node, code, controllerInstanceID string) (*PairingResult, error) {
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("pairing code is required")
	}
	return establishRelationship(ctx, node, "/api/node/pair/complete", pairingRequest{
		Code:                 strings.TrimSpace(code),
		ControllerInstanceID: controllerInstanceID,
	}, false)
}

func UpgradeLegacyRelationship(ctx context.Context, node *model.Node, controllerInstanceID string) (*PairingResult, error) {
	if node == nil || node.AuthMethod != model.NodeAuthMethodLegacy {
		return nil, errors.New("node is not using legacy authentication")
	}
	return establishRelationship(ctx, node, "/api/node/pair/upgrade", pairingRequest{
		ControllerInstanceID: controllerInstanceID,
	}, true)
}

func establishRelationship(ctx context.Context, node *model.Node, path string, payload pairingRequest,
	authenticate bool,
) (*PairingResult, error) {
	if node == nil {
		return nil, errors.New("node is required")
	}
	if err := RequireSecurePairingURL(node.URL); err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(payload.ControllerInstanceID); err != nil {
		return nil, errors.New("invalid controller instance ID")
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	payload.PublicKey = base64.RawURLEncoding.EncodeToString(publicKey)

	var response pairingResponse
	if err := sendRelationshipJSON(ctx, node, http.MethodPost, path, payload, &response, authenticate); err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(response.CredentialID); err != nil {
		return nil, errors.New("paired node returned an invalid credential ID")
	}
	if _, err := uuid.Parse(response.TargetInstanceID); err != nil {
		return nil, errors.New("paired node returned an invalid instance ID")
	}

	encryptedPrivateKey, err := EncryptPrivateCredential(
		SigningCredentialPurpose(response.CredentialID),
		privateKey,
	)
	if err != nil {
		return nil, err
	}
	database := model.UseDB()
	if database == nil {
		return nil, errors.New("node authentication database is unavailable")
	}
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
		return nil, err
	}
	return &PairingResult{
		CredentialID:     response.CredentialID,
		TargetInstanceID: response.TargetInstanceID,
	}, nil
}

func RotateRelationshipCredential(ctx context.Context, node *model.Node) (*RotationResult, error) {
	if node == nil || node.AuthMethod != model.NodeAuthMethodPaired {
		return nil, ErrNodeNotPaired
	}
	database := model.UseDB()
	if database == nil {
		return nil, errors.New("node authentication database is unavailable")
	}
	var credential model.NodeCredential
	if err := database.Where("node_id = ? AND revoked_at IS NULL", node.ID).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCredentialMissing
		}
		return nil, err
	}

	if len(credential.EncryptedPendingPrivateKey) == 0 {
		publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		encrypted, err := EncryptPrivateCredential(SigningCredentialPurpose(credential.CredentialID), privateKey)
		if err != nil {
			return nil, err
		}
		path := "/api/node/credentials/" + url.PathEscape(credential.CredentialID) + "/rotation"
		if err := sendRelationshipJSON(ctx, node, http.MethodPost, path,
			rotationRequest{PublicKey: base64.RawURLEncoding.EncodeToString(publicKey)}, nil, true); err != nil {
			return nil, err
		}
		if err := database.Model(&credential).Updates(map[string]any{
			"pending_public_key":            publicKey,
			"encrypted_pending_private_key": encrypted,
			"status":                        model.NodeCredentialStatusRotating,
		}).Error; err != nil {
			return nil, err
		}
		credential.PendingPublicKey = publicKey
		credential.EncryptedPendingPrivateKey = encrypted
	}

	confirmPath := "/api/node/credentials/" + url.PathEscape(credential.CredentialID) + "/rotation/confirm"
	if err := sendRelationshipJSON(ctx, node, http.MethodPost, confirmPath, nil, nil, true); err != nil {
		return nil, err
	}
	now := time.Now()
	previousValidUntil := now.Add(credentialRotationRecoveryGrace)
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
			"rotated_at":                     now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Node{}).Where("id = ?", node.ID).
			Update("credential_status", model.NodeCredentialStatusActive).Error
	})
	if err != nil {
		return nil, err
	}
	return &RotationResult{
		CredentialID:       credential.CredentialID,
		TargetInstanceID:   credential.TargetInstanceID,
		PreviousValidUntil: previousValidUntil,
	}, nil
}

func CredentialRotationDue(credential *model.NodeCredential, now time.Time) bool {
	if credential == nil || credential.RevokedAt != nil {
		return false
	}
	if credential.Status == model.NodeCredentialStatusRotating {
		return true
	}
	if credential.Status != model.NodeCredentialStatusActive {
		return false
	}
	reference := credential.CreatedAt
	if credential.RotatedAt != nil {
		reference = *credential.RotatedAt
	}
	return !reference.IsZero() && !reference.Add(automaticCredentialRotationAge).After(now)
}

func MaintainRelationships(ctx context.Context, controllerInstanceID string, now time.Time) []MaintenanceIssue {
	database := model.UseDB()
	if database == nil {
		return []MaintenanceIssue{{Operation: "load", Err: errors.New("node authentication database is unavailable")}}
	}
	var nodes []model.Node
	if err := database.Where("enabled = ?", true).Find(&nodes).Error; err != nil {
		return []MaintenanceIssue{{Operation: "load", Err: err}}
	}

	issues := make([]MaintenanceIssue, 0)
	for index := range nodes {
		node := &nodes[index]
		switch node.AuthMethod {
		case model.NodeAuthMethodLegacy:
			if len(node.EncryptedLegacySecret) == 0 {
				continue
			}
			if _, err := UpgradeLegacyRelationship(ctx, node, controllerInstanceID); err != nil {
				issues = append(issues, MaintenanceIssue{NodeID: node.ID, Operation: "upgrade", Err: err})
			}
		case model.NodeAuthMethodPaired:
			var credential model.NodeCredential
			if err := database.Where("node_id = ? AND revoked_at IS NULL", node.ID).First(&credential).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					issues = append(issues, MaintenanceIssue{NodeID: node.ID, Operation: "load", Err: err})
				}
				continue
			}
			if !CredentialRotationDue(&credential, now) {
				continue
			}
			if _, err := RotateRelationshipCredential(ctx, node); err != nil {
				issues = append(issues, MaintenanceIssue{NodeID: node.ID, Operation: "rotate", Err: err})
			}
		}
	}
	return issues
}

func sendRelationshipJSON(ctx context.Context, node *model.Node, method, path string, payload, response any,
	authenticate bool,
) error {
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
	request, err := http.NewRequestWithContext(ctx, method, target, body)
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
		roundTripper = NewTransport(node, roundTripper)
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
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = http.StatusText(httpResponse.StatusCode)
		}
		return fmt.Errorf("node returned %s: %s", httpResponse.Status, message)
	}
	if response != nil && len(responseBody) != 0 {
		if err := json.Unmarshal(responseBody, response); err != nil {
			return err
		}
	}
	return nil
}

func RequireSecurePairingURL(rawURL string) error {
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
