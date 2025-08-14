package cert

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/uozi-tech/cosy/logger"
)

type SyncCertificatePayload struct {
	Name                  string             `json:"name"`
	SSLCertificatePath    string             `json:"ssl_certificate_path"`
	SSLCertificateKeyPath string             `json:"ssl_certificate_key_path"`
	SSLCertificate        string             `json:"ssl_certificate"`
	SSLCertificateKey     string             `json:"ssl_certificate_key"`
	KeyType               certcrypto.KeyType `json:"key_type"`
}

func SyncToRemoteServer(c *model.Cert) (err error) {
	if c.SSLCertificatePath == "" || c.SSLCertificateKeyPath == "" || len(c.SyncNodeIds) == 0 {
		return
	}

	nginxConfPath := nginx.GetConfPath()
	if !helper.IsUnderDirectory(c.SSLCertificatePath, nginxConfPath) {
		return e.NewWithParams(50006, ErrPathIsNotUnderTheNginxConfDir.Error(), c.SSLCertificatePath, nginxConfPath)
	}

	if !helper.IsUnderDirectory(c.SSLCertificateKeyPath, nginxConfPath) {
		return e.NewWithParams(50006, ErrPathIsNotUnderTheNginxConfDir.Error(), c.SSLCertificateKeyPath, nginxConfPath)
	}

	certBytes, err := os.ReadFile(c.SSLCertificatePath)
	if err != nil {
		return
	}
	keyBytes, err := os.ReadFile(c.SSLCertificateKeyPath)
	if err != nil {
		return
	}

	payload := &SyncCertificatePayload{
		Name:                  c.Name,
		SSLCertificatePath:    c.SSLCertificatePath,
		SSLCertificateKeyPath: c.SSLCertificateKeyPath,
		SSLCertificate:        string(certBytes),
		SSLCertificateKey:     string(keyBytes),
		KeyType:               c.KeyType,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}

	q := query.Node
	nodes, _ := q.Where(q.ID.In(c.SyncNodeIds...)).Find()
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

type SyncNotificationPayload struct {
	StatusCode int    `json:"status_code"`
	CertName   string `json:"cert_name"`
	NodeName   string `json:"node_name"`
	Response   string `json:"response"`
}

func deploy(node *model.Node, c *model.Cert, payloadBytes []byte) (err error) {
	t, err := transport.NewTransport()
	if err != nil {
		return
	}
	client := http.Client{
		Transport: t,
	}
	url, err := node.GetUrl("/api/cert_sync")
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return
	}
	req.Header.Set("X-Node-Secret", node.Token)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
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
		return
	}

	notification.Success("Sync Certificate Success",
		"Sync Certificate %{cert_name} to %{node_name} successfully", notificationPayload)

	return
}
