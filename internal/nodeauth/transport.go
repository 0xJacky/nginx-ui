package nodeauth

import (
	"crypto/ed25519"
	"errors"
	"net/http"
	"net/url"
	"time"

	internalTransport "github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-resty/resty/v2"
	"gorm.io/gorm"
)

type authenticatedTransport struct {
	base   http.RoundTripper
	nodeID uint64
}

func NewTransport(node *model.Node, base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &authenticatedTransport{base: base, nodeID: node.ID}
}

func (transport *authenticatedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	database := model.UseDB()
	if database == nil {
		return nil, errors.New("node authentication database is unavailable")
	}

	var node model.Node
	if err := database.First(&node, transport.nodeID).Error; err != nil {
		return nil, err
	}

	now := time.Now()
	if err := applyNodeAuthentication(request, database, &node, now); err != nil {
		return nil, err
	}

	response, err := transport.base.RoundTrip(request)
	if node.AuthMethod == model.NodeAuthMethodPaired {
		_ = database.Model(&model.NodeCredential{}).
			Where("node_id = ?", node.ID).
			Update("last_used_at", now).Error
	}
	_ = database.Model(&model.Node{}).
		Where("id = ?", node.ID).
		Update("last_credential_use_at", now).Error
	return response, err
}

func applyNodeAuthentication(request *http.Request, database *gorm.DB, node *model.Node, now time.Time) error {
	switch node.AuthMethod {
	case "", model.NodeAuthMethodLegacy:
		if err := applyLegacyAuthentication(request, node); err != nil {
			return err
		}
	case model.NodeAuthMethodPaired:
		if err := applyPairedAuthentication(request, database, node, now); err != nil {
			return err
		}
	default:
		return errors.New("unsupported node authentication method")
	}
	return nil
}

// applyLegacyAuthentication preserves the wire protocol used by nodes older
// than v2.5.0. Those nodes only understand X-Node-Secret; once the target is
// upgraded, the relationship maintenance job replaces this legacy credential
// with a dedicated Ed25519 key pair.
func applyLegacyAuthentication(request *http.Request, node *model.Node) error {
	if len(node.EncryptedLegacySecret) == 0 {
		return errors.New("legacy node credential is unavailable")
	}
	secret, err := DecryptPrivateCredential(LegacyCredentialPurpose(node.ID), node.EncryptedLegacySecret)
	if err != nil {
		return err
	}
	request.Header.Del(signatureInputHeader)
	request.Header.Del(signatureHeader)
	request.Header.Del(contentDigestHeader)
	request.Header.Del(CredentialIDHeader)
	request.Header.Del(TargetInstanceHeader)
	request.Header.Set("X-Node-Secret", string(secret))
	return nil
}

func NewHTTPClient(node *model.Node, timeout time.Duration) (*http.Client, error) {
	base, err := internalTransport.NewTransport()
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: NewTransport(node, base),
		Timeout:   timeout,
	}, nil
}

func NewRestyClient(node *model.Node) *resty.Client {
	base, _ := internalTransport.NewTransport()
	return resty.New().SetTransport(NewTransport(node, base))
}

func SignWebSocketHeaders(node *model.Node, rawURL string, header http.Header) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	request := &http.Request{
		Method: http.MethodGet,
		URL:    parsedURL,
		Header: header.Clone(),
		Body:   http.NoBody,
	}
	database := model.UseDB()
	if database == nil {
		return errors.New("node authentication database is unavailable")
	}
	var currentNode model.Node
	if err := database.First(&currentNode, node.ID).Error; err != nil {
		return err
	}
	if err := applyNodeAuthentication(request, database, &currentNode, time.Now()); err != nil {
		return err
	}
	for _, name := range []string{
		"X-Node-Secret",
		contentDigestHeader,
		CredentialIDHeader,
		TargetInstanceHeader,
		signatureInputHeader,
		signatureHeader,
	} {
		header.Del(name)
		for _, value := range request.Header.Values(name) {
			header.Add(name, value)
		}
	}
	CloseStagedBody(request)
	return nil
}

func applyPairedAuthentication(request *http.Request, database *gorm.DB, node *model.Node, now time.Time) error {
	var credential model.NodeCredential
	if err := database.Where(
		"node_id = ? AND revoked_at IS NULL AND status IN ?",
		node.ID,
		[]string{model.NodeCredentialStatusActive, model.NodeCredentialStatusRotating},
	).First(&credential).Error; err != nil {
		return errors.New("paired node credential is unavailable")
	}
	privateKey, err := DecryptPrivateCredential(
		SigningCredentialPurpose(credential.CredentialID),
		credential.EncryptedPrivateKey,
	)
	if err != nil {
		return err
	}
	return SignRequestWithKey(
		request,
		credential.CredentialID,
		credential.TargetInstanceID,
		ed25519.PrivateKey(privateKey),
		now,
	)
}
