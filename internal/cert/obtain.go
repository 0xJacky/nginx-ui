package cert

import (
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/pkg/errors"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func obtain(payload *ConfigPayload, client *lego.Client, l *log.Logger, errChan chan error) {
	request := certificate.ObtainRequest{
		Domains: payload.ServerName,
		Bundle:  true,
	}

	l.Println("[INFO] [Nginx UI] Obtaining certificate")
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		errChan <- errors.Wrap(err, "obtain certificate error")
		return
	}
	payload.Resource = &model.CertificateResource{
		Resource:          certificates,
		PrivateKey:        certificates.PrivateKey,
		Certificate:       certificates.Certificate,
		IssuerCertificate: certificates.IssuerCertificate,
		CSR:               certificates.CSR,
	}
	name := strings.Join(payload.ServerName, "_")
	saveDir := nginx.GetConfPath("ssl/" + name + "_" + string(payload.KeyType))
	if _, err = os.Stat(saveDir); os.IsNotExist(err) {
		err = os.MkdirAll(saveDir, 0755)
		if err != nil {
			errChan <- errors.Wrap(err, "mkdir error")
			return
		}
	}

	// Each certificate comes back with the cert bytes, the bytes of the client's
	// private key, and a certificate URL. SAVE THESE TO DISK.
	l.Println("[INFO] [Nginx UI] Writing certificate to disk")
	err = os.WriteFile(filepath.Join(saveDir, "fullchain.cer"),
		certificates.Certificate, 0644)

	if err != nil {
		errChan <- errors.Wrap(err, "write fullchain.cer error")
		return
	}

	l.Println("[INFO] [Nginx UI] Writing certificate private key to disk")
	err = os.WriteFile(filepath.Join(saveDir, "private.key"),
		certificates.PrivateKey, 0644)

	if err != nil {
		errChan <- errors.Wrap(err, "write private.key error")
		return
	}
}
