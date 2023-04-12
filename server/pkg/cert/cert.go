package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	dns2 "github.com/0xJacky/Nginx-UI/server/pkg/cert/dns"
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns"
	"github.com/go-acme/lego/v4/registration"
	"github.com/pkg/errors"
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
	ServerName      []string    `json:"server_name"`
	ChallengeMethod string      `json:"challenge_method"`
	Config          dns2.Config `json:"config"`
}

func IssueCert(payload *ConfigPayload, logChan chan string, errChan chan error) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Issue Cert recover", err)
		}
	}()

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
	case HTTP01:
		logChan <- "Using HTTP01 challenge provider"
		err = client.Challenge.SetHTTP01Provider(
			http01.NewProviderServer("",
				settings.ServerSettings.HTTPChallengePort,
			),
		)
	case DNS01:
		code := payload.Config.Code
		pConfig, ok := dns2.GetProvider(code)

		if !ok {
			errChan <- errors.Wrap(err, "provider not found")
		}
		logChan <- "Setting environment variables"
		if payload.Config.Configuration != nil {
			err = pConfig.SetEnv(*payload.Config.Configuration)
			if err != nil {
				break
			}
			defer func() {
				logChan <- "Cleaning environment variables"
				pConfig.CleanEnv()
			}()
			provider, err := dns.NewDNSChallengeProviderByName(code)
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

	close(errChan)
	logChan <- "Reloading nginx"

	nginx.Reload()

	logChan <- "Finished"

	close(logChan)
}
