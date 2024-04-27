package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/tencentcloud"
	"github.com/go-acme/lego/v4/registration"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTencentCloudDNS(t *testing.T) {
	domain := []string{"test.jackyu.cn"}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Println(err)
		return
	}

	myUser := User{
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

	// A client facilitates communication with the CA server.
	client, err := lego.NewClient(config)
	if err != nil {
		log.Println(err)
		return
	}

	provider, err := tencentcloud.NewDNSProvider()

	if err != nil {
		log.Println(err)
		return
	}

	err = client.Challenge.SetDNS01Provider(
		provider,
	)

	if err != nil {
		log.Println(err)
		return
	}

	// New users will need to register
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		log.Println(err)
		return
	}
	myUser.Registration = reg

	request := certificate.ObtainRequest{
		Domains: domain,
		Bundle:  true,
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		log.Println(err)
		return
	}
	name := strings.Join(domain, "_")
	saveDir := "tmp/" + name
	if _, err = os.Stat(saveDir); os.IsNotExist(err) {
		err = os.MkdirAll(saveDir, 0755)
		if err != nil {
			return
		}
	}

	// Each certificate comes back with the cert bytes, the bytes of the client's
	// private key, and a certificate URL. SAVE THESE TO DISK.
	err = os.WriteFile(filepath.Join(saveDir, "fullchain.cer"),
		certificates.Certificate, 0644)

	if err != nil {
		log.Println(err)
		return
	}

	err = os.WriteFile(filepath.Join(saveDir, "private.key"),
		certificates.PrivateKey, 0644)

	if err != nil {
		log.Println(err)
		return
	}
}
