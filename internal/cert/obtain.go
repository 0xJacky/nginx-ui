package cert

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/pkg/errors"
	"log"
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

	payload.WriteFile(l, errChan)
}
