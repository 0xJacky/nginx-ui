package cert

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/pkg/errors"
	"runtime"
	"strings"
	"time"
)

func AutoObtain() {
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
		renew(certModel)
	}
	logger.Info("AutoCert Worker End")
}

func renew(certModel *model.Cert) {
	confName := certModel.Filename

	log := &Logger{}
	log.SetCertModel(certModel)
	defer log.Exit()

	if len(certModel.Filename) == 0 {
		log.Error(errors.New("filename is empty"))
		return
	}

	if len(certModel.Domains) == 0 {
		log.Error(errors.New("domains list is empty, " +
			"try to reopen auto-cert for this config:" + confName))
		notification.Error("Renew Certificate Error", confName)
		return
	}

	if certModel.SSLCertificatePath == "" {
		log.Error(errors.New("ssl certificate path is empty, " +
			"try to reopen auto-cert for this config:" + confName))
		notification.Error("Renew Certificate Error", confName)
		return
	}

	cert, err := GetCertInfo(certModel.SSLCertificatePath)
	if err != nil {
		// Get certificate info error, ignore this certificate
		log.Error(errors.Wrap(err, "get certificate info error"))
		notification.Error("Renew Certificate Error", strings.Join(certModel.Domains, ", "))
		return
	}
	if int(time.Now().Sub(cert.NotBefore).Hours()/24) < settings.ServerSettings.CertRenewalInterval {
		// not after settings.ServerSettings.CertRenewalInterval, ignore
		return
	}

	// after 1 mo, reissue certificate
	logChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// support SAN certification
	payload := &ConfigPayload{
		ServerName:      certModel.Domains,
		ChallengeMethod: certModel.ChallengeMethod,
		DNSCredentialID: certModel.DnsCredentialID,
		KeyType:         certModel.GetKeyType(),
	}

	// errChan will be closed inside IssueCert
	go IssueCert(payload, logChan, errChan)

	go func() {
		for logString := range logChan {
			log.Info(strings.TrimSpace(logString))
		}
	}()

	// block, unless errChan closed
	for err := range errChan {
		log.Error(err)
		notification.Error("Renew Certificate Error", strings.Join(payload.ServerName, ", "))
		return
	}

	notification.Success("Renew Certificate Success", strings.Join(payload.ServerName, ", "))
}
