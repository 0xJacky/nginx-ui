package cert

import (
	"log"
	"os"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	legolog "github.com/go-acme/lego/v4/log"
	dnsproviders "github.com/go-acme/lego/v4/providers/dns"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy/logger"
)

const (
	HTTP01 = "http01"
	DNS01  = "dns01"
)

func IssueCert(payload *ConfigPayload, logChan chan string, errChan chan error) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()

	// initial a channelWriter to receive logs
	cw := NewChannelWriter()
	defer close(errChan)

	// initial a logger
	l := log.New(os.Stderr, "", log.LstdFlags)
	l.SetOutput(cw)

	// Hijack the (logger) of lego
	legolog.Logger = l

	l.Println("[INFO] [PrimeWaf] Preparing lego configurations")
	user, err := payload.GetACMEUser()
	if err != nil {
		errChan <- errors.Wrap(err, "issue cert get acme user error")
		return
	}
	l.Printf("[INFO] [PrimeWaf] ACME User: %s, Email: %s, CA Dir: %s\n", user.Name, user.Email, user.CADir)

	// Start a goroutine to fetch and process logs from channel
	go func() {
		for msg := range cw.Ch {
			logChan <- string(msg)
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

	l.Println("[INFO] [PrimeWaf] Creating client facilitates communication with the CA server")
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
		l.Println("[INFO] [PrimeWaf] Setting HTTP01 challenge provider")
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

		l.Println("[INFO] [PrimeWaf] Setting DNS01 challenge provider")
		code := dnsCredential.Config.Code
		pConfig, ok := dns.GetProvider(code)
		if !ok {
			errChan <- errors.Wrap(err, "provider not found")
			return
		}
		l.Println("[INFO] [PrimeWaf] Setting environment variables")
		if dnsCredential.Config.Configuration != nil {
			err = pConfig.SetEnv(*dnsCredential.Config.Configuration)
			if err != nil {
				errChan <- errors.Wrap(err, "set env error")
				logger.Error(err)
				break
			}
			defer func() {
				pConfig.CleanEnv()
				l.Println("[INFO] [PrimeWaf] Environment variables cleaned")
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

	if time.Now().Sub(payload.NotBefore).Hours()/24 <= 21 &&
		payload.Resource != nil && payload.Resource.Certificate != nil {
		renew(payload, client, l, errChan)
	} else {
		obtain(payload, client, l, errChan)
	}

	l.Println("[INFO] [PrimeWaf] Reloading nginx")

	nginx.Reload()

	l.Println("[INFO] [PrimeWaf] Finished")

	// Wait log to be written
	time.Sleep(2 * time.Second)
}
