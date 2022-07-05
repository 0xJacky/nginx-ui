package tool

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/0xJacky/Nginx-UI/server/tool/nginx"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// MyUser You'll need a user or account type that implements acme.User
type MyUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *MyUser) GetEmail() string {
	return u.Email
}
func (u MyUser) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *MyUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

func AutoCert() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("[AutoCert] Recover", err)
		}
	}()
	log.Println("[AutoCert] Start")
	autoCertList := model.GetAutoCertList()
	for i := range autoCertList {
		domain := autoCertList[i].Domain
		key, err := GetCertInfo(domain)
		if err != nil {
			log.Println("GetCertInfo Err", err)
			// 获取证书信息失败，本次跳过
			continue
		}
		// 未到一个月
		if time.Now().Before(key.NotBefore.AddDate(0, 1, 0)) {
			continue
		}
		// 过一个月了，重新申请证书
		err = IssueCert(domain)
		if err != nil {
			log.Println(err)
		}
	}
}

func GetCertInfo(domain string) (key *x509.Certificate, err error) {

	var response *http.Response

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).DialContext,
			DisableKeepAlives: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 5 * time.Second,
	}

	response, err = client.Get("https://" + domain)

	if err != nil {
		err = errors.Wrap(err, "get cert info error")
		return
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
			return
		}
	}(response.Body)

	key = response.TLS.PeerCertificates[0]

	return
}

func IssueCert(domain string) error {
	// Create a user. New accounts need an email and private key to start.
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return errors.Wrap(err, "issue cert generate key error")
	}

	myUser := MyUser{
		Email: settings.ServerSettings.Email,
		key:   privateKey,
	}

	config := lego.NewConfig(&myUser)

	if settings.ServerSettings.Demo {
		config.CADirURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
	}
	config.Certificate.KeyType = certcrypto.RSA2048

	// A client facilitates communication with the CA server.
	client, err := lego.NewClient(config)
	if err != nil {
		return errors.Wrap(err, "issue cert new client error")
	}

	err = client.Challenge.SetHTTP01Provider(
		http01.NewProviderServer("",
			settings.ServerSettings.HTTPChallengePort,
		),
	)
	if err != nil {
		return errors.Wrap(err, "issue cert challenge fail")
	}

	// New users will need to register
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		log.Println(err)
		return errors.Wrap(err, "issue cert register fail")
	}
	myUser.Registration = reg

	request := certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	}
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return errors.Wrap(err, "issue cert fail to obtain")
	}
	saveDir := nginx.GetNginxConfPath("ssl/" + domain)
	if _, err := os.Stat(saveDir); os.IsNotExist(err) {
		err = os.Mkdir(saveDir, 0755)
		if err != nil {
			return errors.Wrap(err, "issue cert fail to create")
		}
	}

	// Each certificate comes back with the cert bytes, the bytes of the client's
	// private key, and a certificate URL. SAVE THESE TO DISK.
	err = ioutil.WriteFile(filepath.Join(saveDir, "fullchain.cer"),
		certificates.Certificate, 0644)
	if err != nil {
		log.Println(err)
		return errors.Wrap(err, "issue cert write fullchain.cer fail")
	}
	err = ioutil.WriteFile(filepath.Join(saveDir, domain+".key"),
		certificates.PrivateKey, 0644)
	if err != nil {
		log.Println(err)
		return errors.Wrap(err, "issue cert write key fail")
	}

	nginx.ReloadNginx()

	return nil
}
