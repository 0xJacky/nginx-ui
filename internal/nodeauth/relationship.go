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
	ErrNodeNotPaired       = errors.New("node is not using paired authentication")
	ErrCredentialMissing   = errors.New("paired credential not found")
	ErrLegacySecretMissing = errors.New("node has no legacy secret to upgrade from")
	// ErrRelationshipUnsupported reports a target that does not expose the
	// handshake endpoint, which is how a node running an older release presents
	// itself. Such a node keeps authenticating with the shared secret until it
	// is upgraded, and the next maintenance pass picks it up on its own.
	ErrRelationshipUnsupported = errors.New("target node does not support paired authentication yet")
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

type upgradeRequest struct {
	ControllerInstanceID string `json:"controller_instance_id"`
	PublicKey            string `json:"public_key"`
}

type pairingResponse struct {
	CredentialID     string `json:"credential_id"`
	TargetInstanceID string `json:"target_instance_id"`
	Confirmation     string `json:"confirmation,omitempty"`
}

type rotationRequest struct {
	PublicKey string `json:"public_key"`
}

// UpgradeLegacyRelationship migrates a node that still authenticates with the
// shared secret onto its own key pair.
//
// The request rides the ordinary authenticated transport, which now signs with
// the shared secret rather than sending it, so this exchange needs no transport
// guarantee of its own: nothing reusable is exposed, and the target's answer is
// authenticated before the new credential is stored.
func UpgradeLegacyRelationship(ctx context.Context, node *model.Node, controllerInstanceID string) (*PairingResult, error) {
	if node == nil {
		return nil, errors.New("node is required")
	}
	if len(node.EncryptedLegacySecret) == 0 {
		return nil, ErrLegacySecretMissing
	}
	if _, err := uuid.Parse(controllerInstanceID); err != nil {
		return nil, errors.New("invalid controller instance ID")
	}
	secret, err := DecryptPrivateCredential(LegacyCredentialPurpose(node.ID), node.EncryptedLegacySecret)
	if err != nil {
		return nil, err
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	var response pairingResponse
	if err := sendRelationshipJSON(ctx, node, http.MethodPost, "/api/node/pair/upgrade", upgradeRequest{
		ControllerInstanceID: controllerInstanceID,
		PublicKey:            base64.RawURLEncoding.EncodeToString(publicKey),
	}, &response, true); err != nil {
		return nil, err
	}
	if err := validatePairingResponse(&response); err != nil {
		return nil, err
	}
	if err := VerifyUpgradeConfirmation(secret, controllerInstanceID, publicKey,
		response.CredentialID, response.TargetInstanceID, response.Confirmation); err != nil {
		return nil, fmt.Errorf("target node did not confirm the upgrade: %w", err)
	}
	return persistRelationship(node, &response, publicKey, privateKey)
}

func validatePairingResponse(response *pairingResponse) error {
	if _, err := uuid.Parse(response.CredentialID); err != nil {
		return errors.New("paired node returned an invalid credential ID")
	}
	if _, err := uuid.Parse(response.TargetInstanceID); err != nil {
		return errors.New("paired node returned an invalid instance ID")
	}
	return nil
}

func persistRelationship(node *model.Node, response *pairingResponse,
	publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey,
) (*PairingResult, error) {
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
		// The encrypted legacy secret is kept on purpose. It stops travelling on
		// the wire the moment auth_method flips to paired, and keeping it lets a
		// node that loses its controller credential (a restored backup, a
		// revoked relationship) be re-upgraded automatically instead of waiting
		// for someone to notice and pair it by hand.
		return tx.Model(&model.Node{}).Where("id = ?", node.ID).Updates(map[string]any{
			"token":             "",
			"auth_method":       model.NodeAuthMethodPaired,
			"credential_status": model.NodeCredentialStatusActive,
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
			// A node that has not been upgraded yet simply keeps using the shared
			// secret. That is an expected state during a rolling upgrade, not a
			// fault worth reporting on every pass.
			if _, err := UpgradeLegacyRelationship(ctx, node, controllerInstanceID); err != nil &&
				!errors.Is(err, ErrRelationshipUnsupported) {
				issues = append(issues, MaintenanceIssue{NodeID: node.ID, Operation: "upgrade", Err: err})
			}
		case model.NodeAuthMethodPaired:
			var credential model.NodeCredential
			if err := database.Where("node_id = ? AND revoked_at IS NULL", node.ID).First(&credential).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					issues = append(issues, MaintenanceIssue{NodeID: node.ID, Operation: "load", Err: err})
					continue
				}
				// The credential is gone while the node still trusts us through
				// its retained secret, so re-run the handshake instead of
				// leaving the relationship broken until someone pairs by hand.
				if len(node.EncryptedLegacySecret) == 0 {
					continue
				}
				if _, err := UpgradeLegacyRelationship(ctx, node, controllerInstanceID); err != nil &&
					!errors.Is(err, ErrRelationshipUnsupported) {
					issues = append(issues, MaintenanceIssue{NodeID: node.ID, Operation: "recover", Err: err})
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
	if httpResponse.StatusCode == http.StatusNotFound || httpResponse.StatusCode == http.StatusMethodNotAllowed {
		return ErrRelationshipUnsupported
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
