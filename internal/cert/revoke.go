package cert

import (
	"context"
	"log/slog"
	"runtime"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/go-acme/lego/v5/acme"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/go-acme/lego/v5/lego"
	legolog "github.com/go-acme/lego/v5/log"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

// RevokeCert revokes a certificate and provides log messages through channels
func RevokeCert(payload *ConfigPayload, certLogger *Logger, logChan chan string, errChan chan error) {
	lock()
	defer unlock()
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, 1024)
			runtime.Stack(buf, false)
			logger.Errorf("%s\n%s", err, buf)
		}
	}()

	// Initialize a channel writer to receive logs
	cw := NewChannelWriter()
	defer close(errChan)
	defer close(cw.Ch)

	// Hijack the logger of lego
	oldLogger := legolog.Default()
	legolog.SetDefault(slog.New(slog.NewTextHandler(cw, nil)))
	// Restore the original logger
	defer func() {
		legolog.SetDefault(oldLogger)
	}()

	// Start a goroutine to fetch and process logs from channel
	go func() {
		for msg := range cw.Ch {
			logChan <- string(msg)
		}
	}()

	// Create client for communication with CA server
	certLogger.Info(translation.C("[Nginx UI] Preparing for certificate revocation"))

	// An expired certificate is no longer trusted by anyone, so there is
	// nothing left to revoke and no reason to contact the CA.
	if certificateExpired(payload) {
		certLogger.Info(translation.C("[Nginx UI] Certificate has already expired, no revocation needed"))
		// Wait for logs to be written
		time.Sleep(2 * time.Second)
		return
	}

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

	// Create the client
	client, err := lego.NewClient(config)
	if err != nil {
		errChan <- errors.Wrap(err, "create client error")
		return
	}

	err = revoke(payload, client, certLogger)
	if err != nil {
		errChan <- err
		return
	}

	// If the revoked certificate was used for the server itself, reload server TLS certificate
	if payload.GetCertificatePath() == cSettings.ServerSettings.SSLCert &&
		payload.GetCertificateKeyPath() == cSettings.ServerSettings.SSLKey {
		certLogger.Info(translation.C("[Nginx UI] Certificate was used for server, reloading server TLS certificate"))
		ReloadServerTLSCertificate()
	}

	certLogger.Info(translation.C("[Nginx UI] Revocation completed"))

	// Wait for logs to be written
	time.Sleep(2 * time.Second)
}

// revoke implements the internal certificate revocation logic
func revoke(payload *ConfigPayload, client *lego.Client, l *Logger) error {
	l.Info(translation.C("[Nginx UI] Revoking certificate"))
	err := client.Certificate.Revoke(context.Background(), payload.Resource.Certificate)
	// Check the problem type before WrapErrorWithParams flattens the error.
	var problem *acme.ProblemDetails
	if errors.As(err, &problem) && problem.Type == acme.AlreadyRevokedErrorType {
		l.Info(translation.C("[Nginx UI] Certificate was already revoked by the CA"))
		return nil
	}
	if err != nil {
		return cosy.WrapErrorWithParams(ErrRevokeCert, err.Error())
	}

	l.Info(translation.C("[Nginx UI] Certificate successfully revoked"))
	return nil
}

// certificateExpired reports whether the leaf certificate in the payload is
// past its NotAfter date. A certificate that cannot be parsed is left to the
// CA to judge.
func certificateExpired(payload *ConfigPayload) bool {
	if payload.Resource == nil {
		return false
	}
	cert, err := certcrypto.ParsePEMCertificate(payload.Resource.Certificate)
	if err != nil {
		return false
	}
	return time.Now().After(cert.NotAfter)
}
