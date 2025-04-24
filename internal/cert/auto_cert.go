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
			logger.Error("AutoCert Recover", err, string(buf))
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
	if int(time.Now().Sub(certInfo.NotBefore).Hours()/24) < settings.CertSettings.GetCertRenewalInterval() {
		// not after settings.ServerSettings.RenewalInterval, ignore
		return
	}

	// after 1 mo, reissue certificate
	logChan := make(chan string, 1)
	errChan := make(chan error, 1)

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

	// errChan will be closed inside IssueCert
	go IssueCert(payload, log, errChan)

	// block, unless errChan closed
	for err := range errChan {
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

	close(logChan)
}
