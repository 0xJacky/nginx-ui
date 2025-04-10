package cert

import (
	"log"
	"os"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/go-acme/lego/v4/lego"
	legolog "github.com/go-acme/lego/v4/log"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

// RevokeCert revokes a certificate and provides log messages through channels
func RevokeCert(payload *ConfigPayload, logChan chan string, errChan chan error) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()

	// Initialize a channel writer to receive logs
	cw := NewChannelWriter()
	defer close(errChan)
	defer close(cw.Ch)

	// Initialize a logger
	l := log.New(os.Stderr, "", log.LstdFlags)
	l.SetOutput(cw)

	// Hijack the logger of lego
	oldLogger := legolog.Logger
	legolog.Logger = l
	// Restore the original logger
	defer func() {
		legolog.Logger = oldLogger
	}()

	// Start a goroutine to fetch and process logs from channel
	go func() {
		for msg := range cw.Ch {
			logChan <- string(msg)
		}
	}()

	// Create client for communication with CA server
	l.Println("[INFO] [Nginx UI] Preparing for certificate revocation")
	user, err := payload.GetACMEUser()
	if err != nil {
		errChan <- errors.Wrap(err, "get ACME user error")
		return
	}

	config := lego.NewConfig(user)
	config.CADirURL = user.CADir

	// Skip TLS check if proxy is configured
	if config.HTTPClient != nil {
		t, err := transport.NewTransport(
			transport.WithProxy(user.Proxy))
		if err != nil {
			errChan <- errors.Wrap(err, "create transport error")
			return
		}
		config.HTTPClient.Transport = t
	}

	config.Certificate.KeyType = payload.GetKeyType()

	// Create the client
	client, err := lego.NewClient(config)
	if err != nil {
		errChan <- errors.Wrap(err, "create client error")
		return
	}

	revoke(payload, client, l, errChan)

	// If the revoked certificate was used for the server itself, reload server TLS certificate
	if payload.GetCertificatePath() == cSettings.ServerSettings.SSLCert &&
		payload.GetCertificateKeyPath() == cSettings.ServerSettings.SSLKey {
		l.Println("[INFO] [Nginx UI] Certificate was used for server, reloading server TLS certificate")
		ReloadServerTLSCertificate()
	}

	l.Println("[INFO] [Nginx UI] Revocation completed")

	// Wait for logs to be written
	time.Sleep(2 * time.Second)
}

// revoke implements the internal certificate revocation logic
func revoke(payload *ConfigPayload, client *lego.Client, l *log.Logger, errChan chan error) {
	l.Println("[INFO] [Nginx UI] Revoking certificate")
	err := client.Certificate.Revoke(payload.Resource.Certificate)
	if err != nil {
		errChan <- errors.Wrap(err, "revoke certificate error")
		return 
	}

	l.Println("[INFO] [Nginx UI] Certificate successfully revoked")
	return 
}
