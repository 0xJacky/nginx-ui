package cert

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/pkg/errors"
	"strings"
	"time"
)

func AutoObtain() {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("AutoCert Recover", err)
		}
	}()
	logger.Info("AutoCert Worker Started")
	autoCertList := model.GetAutoCertList()
	for _, certModel := range autoCertList {
		certModel := certModel
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
		return
	}

	if certModel.SSLCertificatePath != "" {
		cert, err := GetCertInfo(certModel.SSLCertificatePath)
		if err != nil {
			// Get certificate info error, ignore this certificate
			log.Error(errors.Wrap(err, "get certificate info error"))
			return
		}
		if time.Now().Sub(cert.NotBefore).Hours()/24 < 7 {
			// not between 1 week, ignore this certificate
			return
		}
	}
	// after 1 mo, reissue certificate
	logChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// support SAN certification
	payload := &ConfigPayload{
		ServerName:      certModel.Domains,
		ChallengeMethod: certModel.ChallengeMethod,
		DNSCredentialID: certModel.DnsCredentialID,
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
	}
}
