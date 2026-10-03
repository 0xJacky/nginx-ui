package cert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/samber/lo"
	"github.com/uozi-tech/cosy/logger"
)

type SyncCertificatePayload struct {
	Name                  string             `json:"name"`
	SSLCertificatePath    string             `json:"ssl_certificate_path"`
	SSLCertificateKeyPath string             `json:"ssl_certificate_key_path"`
	SSLCertificate        string             `json:"ssl_certificate"`
	SSLCertificateKey     string             `json:"ssl_certificate_key"`
	KeyType               certcrypto.KeyType `json:"key_type"`
	// Delegated marks a certificate the sender issued for this node and keeps
	// renewing, so a record of this node for the same files stops renewing.
	Delegated bool `json:"delegated,omitempty"`
}

// SyncToRemoteServer pushes the certificate files to the nodes configured on
// the certificate and to the sync nodes of every site and stream that loads
// it, so a renewal reaches each node that serves the certificate.
func SyncToRemoteServer(c *model.Cert) (err error) {
	if c.SSLCertificatePath == "" || c.SSLCertificateKeyPath == "" {
		return
	}
	if c.IsDelegated() {
		return syncDelegated(c)
	}

	nodeIDs := lo.Uniq(append(append([]uint64{}, c.SyncNodeIds...), referencingNodeIDs(c)...))
	if len(nodeIDs) == 0 {
		return
	}

	nginxConfPath := nginx.GetConfPath()
	for _, path := range []string{c.SSLCertificatePath, c.SSLCertificateKeyPath} {
		if helper.IsUnderDirectory(path, nginxConfPath) {
			continue
		}
		// Files outside the configuration directory are managed on each node
		// by other means. Only an explicit sync target makes that an error.
		if len(c.SyncNodeIds) == 0 {
			return nil
		}
		return e.NewWithParams(50006, ErrPathIsNotUnderTheNginxConfDir.Error(), path, nginxConfPath)
	}

	payload, err := newSyncPayload(c.Name, c.SSLCertificatePath, c.SSLCertificateKeyPath, c.GetKeyType())
	if err != nil {
		return
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}

	q := query.Node
	nodes, _ := q.Where(q.ID.In(nodeIDs...)).Find()
	for _, node := range nodes {
		go func() {
			err := deploy(node, c, payloadBytes)
			if err != nil {
				logger.Error(err)
			}
		}()
	}

	return
}

// newSyncPayload reads a certificate pair from the Nginx target filesystem
// into the body accepted by the /api/cert_sync endpoint of a node.
func newSyncPayload(name, certPath, keyPath string, keyType certcrypto.KeyType) (*SyncCertificatePayload, error) {
	certBytes, err := nginx.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	keyBytes, err := nginx.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	return &SyncCertificatePayload{
		Name:                  name,
		SSLCertificatePath:    certPath,
		SSLCertificateKeyPath: keyPath,
		SSLCertificate:        string(certBytes),
		SSLCertificateKey:     string(keyBytes),
		KeyType:               keyType,
	}, nil
}

type SyncNotificationPayload struct {
	StatusCode int    `json:"status_code"`
	CertName   string `json:"cert_name"`
	NodeName   string `json:"node_name"`
	Response   string `json:"response"`
}

func deploy(node *model.Node, c *model.Cert, payloadBytes []byte) error {
	_, err := deployWithReply(node, c, payloadBytes)
	return err
}

// deployWithReply sends a certificate to a node and returns the reply of the
// node, which names the record the node keeps the certificate in.
func deployWithReply(node *model.Node, c *model.Cert, payloadBytes []byte) (respBody []byte, err error) {
	client, err := nodeauth.NewHTTPClient(node, 0)
	if err != nil {
		return
	}
	url, err := node.GetUrl("/api/cert_sync")
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	notificationPayload := &SyncNotificationPayload{
		StatusCode: resp.StatusCode,
		CertName:   c.Name,
		NodeName:   node.Name,
		Response:   string(respBody),
	}

	if resp.StatusCode != http.StatusOK {
		notification.Error("Sync Certificate Error",
			"Sync Certificate %{cert_name} to %{node_name} failed", notificationPayload)
		return respBody, fmt.Errorf("node %s answered %s", node.Name, resp.Status)
	}

	notification.Success("Sync Certificate Success",
		"Sync Certificate %{cert_name} to %{node_name} successfully", notificationPayload)

	return
}
