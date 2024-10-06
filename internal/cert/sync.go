package cert

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v4/certcrypto"
	"io"
	"net/http"
	"os"
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
		return fmt.Errorf("ssl_certificate_path: %s is not under the nginx conf path: %s",
			c.SSLCertificatePath, nginxConfPath)
	}

	if !helper.IsUnderDirectory(c.SSLCertificateKeyPath, nginxConfPath) {
		return fmt.Errorf("ssl_certificate_key_path: %s is not under the nginx conf path: %s",
			c.SSLCertificateKeyPath, nginxConfPath)
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

	q := query.Environment
	envs, _ := q.Where(q.ID.In(c.SyncNodeIds...)).Find()
	for _, env := range envs {
		go func() {
			err := deploy(env, c, payloadBytes)
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
	EnvName    string `json:"env_name"`
	RespBody   string `json:"resp_body"`
}

func deploy(env *model.Environment, c *model.Cert, payloadBytes []byte) (err error) {
	client := http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: settings.ServerSettings.InsecureSkipVerify},
		},
	}
	url, err := env.GetUrl("/api/cert_sync")
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return
	}
	req.Header.Set("X-Node-Secret", env.Token)
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
		EnvName:    env.Name,
		RespBody:   string(respBody),
	}

	notificationPayloadBytes, err := json.Marshal(notificationPayload)
	if err != nil {
		return
	}

	if resp.StatusCode != http.StatusOK {
		notification.Error("Sync Certificate Error", string(notificationPayloadBytes))
		return
	}

	notification.Success("Sync Certificate Success", string(notificationPayloadBytes))

	return
}
