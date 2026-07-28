package model

import (
	"net/url"
	"strings"
	"time"
)

const (
	NodeAuthMethodLegacy = "legacy_secret"
	NodeAuthMethodPaired = "paired_ed25519"

	NodeCredentialStatusActive   = "active"
	NodeCredentialStatusUnpaired = "unpaired"
	NodeCredentialStatusRotating = "rotating"
	NodeCredentialStatusRevoked  = "revoked"
)

type Node struct {
	Model
	Name                  string     `json:"name"`
	URL                   string     `json:"url"`
	Token                 string     `json:"-"`
	EncryptedLegacySecret []byte     `json:"-"`
	AuthMethod            string     `json:"auth_method" gorm:"default:legacy_secret;index"`
	CredentialStatus      string     `json:"credential_status" gorm:"default:unpaired"`
	LastCredentialUseAt   *time.Time `json:"last_credential_use_at,omitempty"`
	Enabled               bool       `json:"enabled" gorm:"default:false"`
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

type NodePairingCode struct {
	Model
	CodeHash  []byte     `json:"-" gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time  `json:"expires_at" gorm:"index;not null"`
	UsedAt    *time.Time `json:"used_at,omitempty" gorm:"index"`
	CreatedBy uint64     `json:"created_by" gorm:"index;not null"`
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

	defaultPort := ""
	if baseUrl.Port() == "" {
		switch baseUrl.Scheme {
		default:
			fallthrough
		case "http":
			defaultPort = "80"
		case "https":
			defaultPort = "443"
		}

		baseUrl.Host = baseUrl.Hostname() + ":" + defaultPort
	}

	u, err := url.JoinPath(baseUrl.String(), uri)

	if err != nil {
		return
	}

	decodedUri, err = url.QueryUnescape(u)

	if err != nil {
		return
	}
	// http will be replaced with ws, https will be replaced with wss
	decodedUri = strings.ReplaceAll(decodedUri, "http", "ws")
	return
}
