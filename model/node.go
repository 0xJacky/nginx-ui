package model

import (
	"net"
	"net/url"
	"time"
)

const (
	NodeAuthMethodLegacy = "legacy_secret"
	NodeAuthMethodPaired = "paired_ed25519"

	NodeCredentialStatusActive   = "active"
	NodeCredentialStatusUnpaired = "unpaired"
	NodeCredentialStatusRotating = "rotating"
	NodeCredentialStatusRevoked  = "revoked"

	NodeAuthUpgradeStatusPending       = "pending"
	NodeAuthUpgradeStatusInProgress    = "in_progress"
	NodeAuthUpgradeStatusWaitingTarget = "waiting_target"
	NodeAuthUpgradeStatusFailed        = "failed"
	NodeAuthUpgradeStatusCompleted     = "completed"
	NodeAuthUpgradeStatusPaused        = "paused"

	NodeAuthUpgradeStepQueued    = "queued"
	NodeAuthUpgradeStepRequest   = "request"
	NodeAuthUpgradeStepVerify    = "verify"
	NodeAuthUpgradeStepPersist   = "persist"
	NodeAuthUpgradeStepCompleted = "completed"

	NodeAuthUpgradeErrorTargetUnsupported      = "target_unsupported"
	NodeAuthUpgradeErrorTimeout                = "timeout"
	NodeAuthUpgradeErrorConnectionFailed       = "connection_failed"
	NodeAuthUpgradeErrorAuthenticationRejected = "authentication_rejected"
	NodeAuthUpgradeErrorTargetRejected         = "target_rejected"
	NodeAuthUpgradeErrorInvalidResponse        = "invalid_response"
	NodeAuthUpgradeErrorInvalidConfirmation    = "invalid_confirmation"
	NodeAuthUpgradeErrorPersistenceFailed      = "persistence_failed"
	NodeAuthUpgradeErrorMissingLegacySecret    = "missing_legacy_secret"
	NodeAuthUpgradeErrorInternal               = "internal"
)

type Node struct {
	Model
	Name                    string     `json:"name"`
	URL                     string     `json:"url"`
	Token                   string     `json:"-"`
	EncryptedLegacySecret   []byte     `json:"-"`
	AuthMethod              string     `json:"auth_method" gorm:"default:legacy_secret;index"`
	CredentialStatus        string     `json:"credential_status" gorm:"default:unpaired"`
	LastCredentialUseAt     *time.Time `json:"last_credential_use_at,omitempty"`
	AuthUpgradeStatus       string     `json:"auth_upgrade_status" gorm:"index"`
	AuthUpgradeStep         string     `json:"auth_upgrade_step"`
	AuthUpgradeAttemptCount uint       `json:"auth_upgrade_attempt_count"`
	AuthUpgradeAttemptedAt  *time.Time `json:"auth_upgrade_attempted_at,omitempty"`
	AuthUpgradeNextRetryAt  *time.Time `json:"auth_upgrade_next_retry_at,omitempty"`
	AuthUpgradeCompletedAt  *time.Time `json:"auth_upgrade_completed_at,omitempty"`
	AuthUpgradeErrorCode    string     `json:"auth_upgrade_error_code,omitempty"`
	AuthUpgradeError        string     `json:"auth_upgrade_error,omitempty"`
	Enabled                 bool       `json:"enabled" gorm:"default:false"`
}

func (n *Node) HasCredential() bool {
	if n == nil {
		return false
	}
	if n.AuthMethod == NodeAuthMethodLegacy {
		return len(n.EncryptedLegacySecret) != 0 || n.Token != ""
	}
	return n.CredentialStatus == NodeCredentialStatusActive || n.CredentialStatus == NodeCredentialStatusRotating
}

type NodeCredential struct {
	Model
	NodeID                      uint64     `json:"node_id" gorm:"uniqueIndex"`
	CredentialID                string     `json:"credential_id" gorm:"uniqueIndex;not null"`
	TargetInstanceID            string     `json:"target_instance_id" gorm:"index;not null"`
	PublicKey                   []byte     `json:"-" gorm:"not null"`
	EncryptedPrivateKey         []byte     `json:"-" gorm:"not null"`
	PreviousPublicKey           []byte     `json:"-"`
	EncryptedPreviousPrivateKey []byte     `json:"-"`
	PreviousValidUntil          *time.Time `json:"previous_valid_until,omitempty"`
	PendingPublicKey            []byte     `json:"-"`
	EncryptedPendingPrivateKey  []byte     `json:"-"`
	Status                      string     `json:"status" gorm:"index;not null"`
	RotatedAt                   *time.Time `json:"rotated_at,omitempty"`
	LastUsedAt                  *time.Time `json:"last_used_at,omitempty"`
	RevokedAt                   *time.Time `json:"revoked_at,omitempty" gorm:"index"`
}

type NodeControllerCredential struct {
	Model
	CredentialID         string     `json:"credential_id" gorm:"uniqueIndex;not null"`
	ControllerInstanceID string     `json:"controller_instance_id" gorm:"index;not null"`
	PublicKey            []byte     `json:"-" gorm:"not null"`
	PreviousPublicKey    []byte     `json:"-"`
	PreviousValidUntil   *time.Time `json:"previous_valid_until,omitempty"`
	PendingPublicKey     []byte     `json:"-"`
	Status               string     `json:"status" gorm:"index;not null"`
	LastUsedAt           *time.Time `json:"last_used_at,omitempty"`
	RevokedAt            *time.Time `json:"revoked_at,omitempty" gorm:"index"`
}

func (n *Node) GetUrl(uri string) (decodedUri string, err error) {
	baseUrl, err := url.Parse(n.URL)
	if err != nil {
		return
	}

	u, err := url.JoinPath(baseUrl.String(), uri)
	if err != nil {
		return
	}

	decodedUri, err = url.QueryUnescape(u)
	if err != nil {
		return
	}

	return
}

func (n *Node) GetWebSocketURL(uri string) (decodedUri string, err error) {
	baseUrl, err := url.Parse(n.URL)
	if err != nil {
		return
	}

	// Switch the scheme on the parsed URL instead of rewriting the rendered
	// string, so a host, path, or query value that happens to contain "http"
	// is left untouched.
	defaultPort := ""
	switch baseUrl.Scheme {
	case "https", "wss":
		defaultPort = "443"
		baseUrl.Scheme = "wss"
	default:
		defaultPort = "80"
		baseUrl.Scheme = "ws"
	}

	if baseUrl.Port() == "" {
		baseUrl.Host = net.JoinHostPort(baseUrl.Hostname(), defaultPort)
	}

	u, err := url.JoinPath(baseUrl.String(), uri)

	if err != nil {
		return
	}

	decodedUri, err = url.QueryUnescape(u)

	if err != nil {
		return
	}

	return
}
