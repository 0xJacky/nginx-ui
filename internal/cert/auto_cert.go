package cert

import (
	"runtime"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy/logger"
)

func AutoCert() {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, 1024)
			runtime.Stack(buf, false)
			logger.Errorf("%s\n%s", err, buf)
		}
	}()
	logger.Info("AutoCert Worker Started")
	autoCertList := model.GetAutoCertList()
	for _, certModel := range autoCertList {
		autoCert(certModel)
	}
	logger.Info("AutoCert Worker End")
}

func autoCert(certModel *model.Cert) {
	confName := certModel.Filename

	log := NewLogger()
	log.SetCertModel(certModel)
	defer log.Close()

	if len(certModel.Filename) == 0 {
		log.Error(ErrCertModelFilenameEmpty)
		return
	}

	if len(certModel.Domains) == 0 {
		log.Error(errors.New("domains list is empty, " +
			"try to reopen auto-cert for this config:" + confName))
		notification.Error("Renew Certificate Error", confName, nil)
		return
	}

	if certModel.SSLCertificatePath == "" {
		log.Error(errors.New("ssl certificate path is empty, " +
			"try to reopen auto-cert for this config:" + confName))
		notification.Error("Renew Certificate Error", confName, nil)
		return
	}

	certInfo, err := GetCertInfo(certModel.SSLCertificatePath)
	if err != nil {
		// Get certificate info error, ignore this certificate
		log.Error(errors.Wrap(err, "get certificate info error"))
		notification.Error("Renew Certificate Error", strings.Join(certModel.Domains, ", "), nil)
		return
	}

	// Calculate certificate age (days since NotBefore)
	certAge := int(time.Since(certInfo.NotBefore).Hours() / 24)
	// Calculate days until expiration
	daysUntilExpiration := int(time.Until(certInfo.NotAfter).Hours() / 24)

	// Skip renewal only if:
	// 1. Certificate age is less than renewal interval AND
	// 2. Certificate has more than 6 days remaining before expiration
	if certAge < settings.CertSettings.GetCertRenewalInterval() && daysUntilExpiration > 6 {
		// Certificate is too young and not expiring soon, ignore
		return
	}

	// after 1 mo, reissue certificate
	// support SAN certification
	payload := &ConfigPayload{
		CertID:                  certModel.ID,
		ServerName:              certModel.Domains,
		ChallengeMethod:         certModel.ChallengeMethod,
		DNSCredentialID:         certModel.DnsCredentialID,
		KeyType:                 certModel.GetKeyType(),
		NotBefore:               certInfo.NotBefore,
		MustStaple:              certModel.MustStaple,
		LegoDisableCNAMESupport: certModel.LegoDisableCNAMESupport,
		RevokeOld:               certModel.RevokeOld,
	}

	if certModel.Resource != nil {
		payload.Resource = &model.CertificateResource{
			Resource:          certModel.Resource.Resource,
			PrivateKey:        certModel.Resource.PrivateKey,
			Certificate:       certModel.Resource.Certificate,
			IssuerCertificate: certModel.Resource.IssuerCertificate,
			CSR:               certModel.Resource.CSR,
		}
	}

	err = IssueCert(payload, log)
	if err != nil {
		log.Error(err)
		notification.Error("Renew Certificate Error", strings.Join(payload.ServerName, ", "), nil)
		return
	}

	notification.Success("Renew Certificate Success", strings.Join(payload.ServerName, ", "), nil)
	err = SyncToRemoteServer(certModel)
	if err != nil {
		notification.Error("Sync Certificate Error", err.Error(), nil)
		return
	}
}
