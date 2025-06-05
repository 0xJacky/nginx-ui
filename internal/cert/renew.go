package cert

import (
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/uozi-tech/cosy"
)

func renew(payload *ConfigPayload, client *lego.Client, l *Logger) error {
	if payload.Resource == nil {
		return ErrPayloadResourceIsNil
	}

	options := &certificate.RenewOptions{
		Bundle:     true,
		MustStaple: payload.MustStaple,
	}

	cert, err := client.Certificate.RenewWithOptions(payload.Resource.GetResource(), options)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrRenewCert, err.Error())
	}

	payload.Resource = &model.CertificateResource{
		Resource:          cert,
		PrivateKey:        cert.PrivateKey,
		Certificate:       cert.Certificate,
		IssuerCertificate: cert.IssuerCertificate,
		CSR:               cert.CSR,
	}

	err = payload.WriteFile(l)
	if err != nil {
		return err
	}

	l.Info(translation.C("[Nginx UI] Certificate renewed successfully"))

	return nil
}
