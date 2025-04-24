package cert

import (
	"log"
	"os"
	"runtime"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	legolog "github.com/go-acme/lego/v4/log"
	dnsproviders "github.com/go-acme/lego/v4/providers/dns"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

const (
	HTTP01 = "http01"
	DNS01  = "dns01"
)

func IssueCert(payload *ConfigPayload, certLogger *Logger, errChan chan error) {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, 1024)
			runtime.Stack(buf, false)
			logger.Error(err)
		}
	}()

	// initial a channelWriter to receive logs
	cw := NewChannelWriter()
	defer close(errChan)
	defer close(cw.Ch)

	// initial a logger
	l := log.New(os.Stderr, "", log.LstdFlags)
	l.SetOutput(cw)

	// Hijack the (logger) of lego
	legolog.Logger = l
	// Restore the original logger, fix #876
	defer func() {
		legolog.Logger = log.New(os.Stderr, "", log.LstdFlags)
	}()

	certLogger.Info(translation.C("[Nginx UI] Preparing lego configurations"))
	user, err := payload.GetACMEUser()
	if err != nil {
		errChan <- errors.Wrap(err, "issue cert get acme user error")
		return
	}

	certLogger.Info(translation.C("[Nginx UI] ACME User: %{name}, Email: %{email}, CA Dir: %{caDir}", map[string]any{
		"name":  user.Name,
		"email": user.Email,
		"caDir": user.CADir,
	}))

	// Start a goroutine to fetch and process logs from channel
	go func() {
		for msg := range cw.Ch {
			certLogger.Info(translation.C(string(msg)))
		}
	}()

	config := lego.NewConfig(user)

	config.CADirURL = user.CADir

	// Skip TLS check
	if config.HTTPClient != nil {
		t, err := transport.NewTransport(
			transport.WithProxy(user.Proxy))
		if err != nil {
			return
		}
		config.HTTPClient.Transport = t
	}

	config.Certificate.KeyType = payload.GetKeyType()

	certLogger.Info(translation.C("[Nginx UI] Creating client facilitates communication with the CA server"))
	// A client facilitates communication with the CA server.
	client, err := lego.NewClient(config)
	if err != nil {
		errChan <- errors.Wrap(err, "issue cert new client error")
		return
	}

	switch payload.ChallengeMethod {
	default:
		fallthrough
	case HTTP01:
		certLogger.Info(translation.C("[Nginx UI] Setting HTTP01 challenge provider"))
		err = client.Challenge.SetHTTP01Provider(
			http01.NewProviderServer("",
				settings.CertSettings.HTTPChallengePort,
			),
		)
	case DNS01:
		d := query.DnsCredential
		dnsCredential, err := d.FirstByID(payload.DNSCredentialID)
		if err != nil {
			errChan <- errors.Wrap(err, "get dns credential error")
			return
		}

		certLogger.Info(translation.C("[Nginx UI] Setting DNS01 challenge provider"))
		code := dnsCredential.Config.Code
		pConfig, ok := dns.GetProvider(code)
		if !ok {
			errChan <- errors.Wrap(err, "provider not found")
			return
		}
		certLogger.Info(translation.C("[Nginx UI] Setting environment variables"))
		if dnsCredential.Config.Configuration != nil {
			err = pConfig.SetEnv(*dnsCredential.Config.Configuration)
			if err != nil {
				errChan <- errors.Wrap(err, "set env error")
				logger.Error(err)
				break
			}
			defer func() {
				pConfig.CleanEnv()
				certLogger.Info(translation.C("[Nginx UI] Environment variables cleaned"))
			}()
			provider, err := dnsproviders.NewDNSChallengeProviderByName(code)
			if err != nil {
				errChan <- errors.Wrap(err, "new dns challenge provider error")
				logger.Error(err)
				break
			}
			challengeOptions := make([]dns01.ChallengeOption, 0)

			if len(settings.CertSettings.RecursiveNameservers) > 0 {
				challengeOptions = append(challengeOptions,
					dns01.AddRecursiveNameservers(settings.CertSettings.RecursiveNameservers),
				)
			}

			err = client.Challenge.SetDNS01Provider(provider, challengeOptions...)
		} else {
			errChan <- errors.Wrap(err, "environment configuration is empty")
			return
		}
	}

	if err != nil {
		errChan <- errors.Wrap(err, "challenge error")
		return
	}

	// fix #407
	if payload.LegoDisableCNAMESupport {
		err = os.Setenv("LEGO_DISABLE_CNAME_SUPPORT", "true")
		if err != nil {
			errChan <- errors.Wrap(err, "set env flag to disable lego CNAME support error")
			return
		}
		defer func() {
			_ = os.Unsetenv("LEGO_DISABLE_CNAME_SUPPORT")
		}()
	}

	// Backup current certificate and key if RevokeOld is true
	var oldResource *model.CertificateResource

	if payload.RevokeOld && payload.Resource != nil && payload.Resource.Certificate != nil {
		certLogger.Info(translation.C("[Nginx UI] Backing up current certificate for later revocation"))

		// Save a copy of the old certificate and key
		oldResource = &model.CertificateResource{
			Resource:    payload.Resource.Resource,
			Certificate: payload.Resource.Certificate,
			PrivateKey:  payload.Resource.PrivateKey,
		}
	}

	if time.Now().Sub(payload.NotBefore).Hours()/24 <= 21 &&
		payload.Resource != nil && payload.Resource.Certificate != nil {
		renew(payload, client, certLogger, errChan)
	} else {
		obtain(payload, client, certLogger, errChan)
	}

	certLogger.Info(translation.C("[Nginx UI] Reloading nginx"))

	nginx.Reload()

	certLogger.Info(translation.C("[Nginx UI] Finished"))

	if payload.GetCertificatePath() == cSettings.ServerSettings.SSLCert &&
		payload.GetCertificateKeyPath() == cSettings.ServerSettings.SSLKey {
		ReloadServerTLSCertificate()
	}

	// Revoke old certificate if requested and we have a backup
	if payload.RevokeOld && oldResource != nil && len(oldResource.Certificate) > 0 {
		certLogger.Info(translation.C("[Nginx UI] Revoking old certificate"))

		// Create a payload for revocation using old certificate
		revokePayload := &ConfigPayload{
			CertID:          payload.CertID,
			ServerName:      payload.ServerName,
			ChallengeMethod: payload.ChallengeMethod,
			DNSCredentialID: payload.DNSCredentialID,
			ACMEUserID:      payload.ACMEUserID,
			KeyType:         payload.KeyType,
			Resource:        oldResource,
		}

		// Revoke the old certificate
		revoke(revokePayload, client, certLogger, errChan)
	}

	// Wait log to be written
	time.Sleep(2 * time.Second)
}
