package cert

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/pkg/errors"
	"log"
)

func renew(payload *ConfigPayload, client *lego.Client, l *log.Logger, errChan chan error) {
	if payload.Resource == nil {
		errChan <- errors.New("resource is nil")
		return
	}

	options := &certificate.RenewOptions{
		Bundle:     true,
		MustStaple: payload.MustStaple,
	}

	cert, err := client.Certificate.RenewWithOptions(payload.Resource.GetResource(), options)
	if err != nil {
		errChan <- errors.Wrap(err, "renew cert error")
		return
	}

	payload.Resource = &model.CertificateResource{
		Resource:          cert,
		PrivateKey:        cert.PrivateKey,
		Certificate:       cert.Certificate,
		IssuerCertificate: cert.IssuerCertificate,
		CSR:               cert.CSR,
	}

	payload.WriteFile(l, errChan)

	l.Println("[INFO] [Nginx UI] Certificate renewed successfully")
}
