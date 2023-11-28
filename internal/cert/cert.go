package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	lego_log "github.com/go-acme/lego/v4/log"
	dns_providers "github.com/go-acme/lego/v4/providers/dns"
	"github.com/go-acme/lego/v4/registration"
	"github.com/pkg/errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	HTTP01 = "http01"
	DNS01  = "dns01"
)

// MyUser You'll need a user or account type that implements acme.User
type MyUser struct {
	Email        string
	Registration *registration.Resource
	Key          crypto.PrivateKey
}

func (u *MyUser) GetEmail() string {
	return u.Email
}
func (u *MyUser) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *MyUser) GetPrivateKey() crypto.PrivateKey {
	return u.Key
}

type ConfigPayload struct {
	ServerName      []string `json:"server_name"`
	ChallengeMethod string   `json:"challenge_method"`
	DNSCredentialID int      `json:"dns_credential_id"`
}

type channelWriter struct {
	ch chan []byte
}

func (cw *channelWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	temp := make([]byte, n)
	copy(temp, p)
	cw.ch <- temp
	return n, nil
}

func IssueCert(payload *ConfigPayload, logChan chan string, errChan chan error) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()

    defer close(logChan)
    defer close(errChan)

	// Use a channel to receive lego log
	logChannel := make(chan []byte, 1024)
	defer close(logChannel)

	domain := payload.ServerName

	// Create a user. New accounts need an email and private key to start.
	logChan <- "Generating private key for registering account"
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		errChan <- errors.Wrap(err, "issue cert generate key error")
		return
	}

	logChan <- "Preparing lego configurations"
	myUser := MyUser{
		Email: settings.ServerSettings.Email,
		Key:   privateKey,
	}

	// Hijack the (logger) of lego
	cw := &channelWriter{ch: logChannel}
	multiWriter := io.MultiWriter(os.Stderr, cw)
	l := log.New(os.Stderr, "", log.LstdFlags)
	l.SetOutput(multiWriter)
	lego_log.Logger = l

	// Start a goroutine to fetch and process logs from channel
	go func() {
		for msg := range logChannel {
			logChan <- string(msg)
		}
	}()

	config := lego.NewConfig(&myUser)

	if settings.ServerSettings.Demo {
		config.CADirURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
	}

	if settings.ServerSettings.CADir != "" {
		config.CADirURL = settings.ServerSettings.CADir
		if config.HTTPClient != nil {
			config.HTTPClient.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
	}

	config.Certificate.KeyType = certcrypto.RSA2048

	logChan <- "Creating client facilitates communication with the CA server"
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
		logChan <- "Using HTTP01 challenge provider"
		err = client.Challenge.SetHTTP01Provider(
			http01.NewProviderServer("",
				settings.ServerSettings.HTTPChallengePort,
			),
		)
	case DNS01:
		d := query.DnsCredential
		dnsCredential, err := d.FirstByID(payload.DNSCredentialID)
		if err != nil {
			errChan <- errors.Wrap(err, "get dns credential error")
			return
		}

		logChan <- "Using DNS01 challenge provider"
		code := dnsCredential.Config.Code
		pConfig, ok := dns.GetProvider(code)

		if !ok {
			errChan <- errors.Wrap(err, "provider not found")
		}
		logChan <- "Setting environment variables"
		if dnsCredential.Config.Configuration != nil {
			err = pConfig.SetEnv(*dnsCredential.Config.Configuration)
			if err != nil {
				break
			}
			defer func() {
				logChan <- "Cleaning environment variables"
				pConfig.CleanEnv()
			}()
			provider, err := dns_providers.NewDNSChallengeProviderByName(code)
			if err != nil {
				break
			}
			err = client.Challenge.SetDNS01Provider(provider)
		} else {
			errChan <- errors.Wrap(err, "environment configuration is empty")
			return
		}

	}

	if err != nil {
		errChan <- errors.Wrap(err, "fail to challenge")
		return
	}

	// New users will need to register
	logChan <- "Registering user"
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		errChan <- errors.Wrap(err, "fail to register")
		return
	}
	myUser.Registration = reg

	request := certificate.ObtainRequest{
		Domains: domain,
		Bundle:  true,
	}

	logChan <- "Obtaining certificate"
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		errChan <- errors.Wrap(err, "fail to obtain")
		return
	}
	name := strings.Join(domain, "_")
	saveDir := nginx.GetConfPath("ssl/" + name)
	if _, err = os.Stat(saveDir); os.IsNotExist(err) {
		err = os.MkdirAll(saveDir, 0755)
		if err != nil {
			errChan <- errors.Wrap(err, "fail to mkdir")
			return
		}
	}

	// Each certificate comes back with the cert bytes, the bytes of the client's
	// private key, and a certificate URL. SAVE THESE TO DISK.
	logChan <- "Writing certificate to disk"
	err = os.WriteFile(filepath.Join(saveDir, "fullchain.cer"),
		certificates.Certificate, 0644)

	if err != nil {
		errChan <- errors.Wrap(err, "error issue cert write fullchain.cer")
		return
	}

	logChan <- "Writing certificate private key to disk"
	err = os.WriteFile(filepath.Join(saveDir, "private.key"),
		certificates.PrivateKey, 0644)

	if err != nil {
		errChan <- errors.Wrap(err, "fail to write key")
		return
	}

	logChan <- "Reloading nginx"

	nginx.Reload()

	logChan <- "Finished"
}
