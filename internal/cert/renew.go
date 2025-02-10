package cert

import (
	"log"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/pkg/errors"
)

func renew(payload *ConfigPayload, client *lego.Client, l *log.Logger, errChan chan error) {
	if payload.Resource == nil {
		errChan <- ErrPayloadResourceIsNil
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

	l.Println("[INFO] [PrimeWaf] Certificate renewed successfully")
}
