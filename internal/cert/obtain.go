package cert

import (
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/uozi-tech/cosy"
)

func obtain(payload *ConfigPayload, client *lego.Client, l *Logger) error {
	request := certificate.ObtainRequest{
		Domains:    payload.ServerName,
		Bundle:     true,
		MustStaple: payload.MustStaple,
	}

	l.Info(translation.C("[Nginx UI] Obtaining certificate"))
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrObtainCert, err.Error())
	}

	payload.Resource = &model.CertificateResource{
		Resource:          certificates,
		PrivateKey:        certificates.PrivateKey,
		Certificate:       certificates.Certificate,
		IssuerCertificate: certificates.IssuerCertificate,
		CSR:               certificates.CSR,
	}

	err = payload.WriteFile(l)
	if err != nil {
		return err
	}

	return nil
}
