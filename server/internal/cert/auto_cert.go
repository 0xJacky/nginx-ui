package cert

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/server/internal/logger"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/pkg/errors"
	"time"
)

func handleIssueCertLogChan(logChan chan string) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()

	for logString := range logChan {
		logger.Info("Auto Cert", logString)
	}
}

type AutoCertErrorLog struct {
	buffer []string
	cert   *model.Cert
}

func (t *AutoCertErrorLog) SetCertModel(cert *model.Cert) {
	t.cert = cert
}

func (t *AutoCertErrorLog) Push(text string, err error) {
	t.buffer = append(t.buffer, text+" "+err.Error())
	logger.Error("AutoCert", text, err)
}

func (t *AutoCertErrorLog) Exit(text string, err error) {
	t.buffer = append(t.buffer, text+" "+err.Error())
	logger.Error("AutoCert", text, err)

	if t.cert == nil {
		return
	}

	_ = t.cert.Updates(&model.Cert{
		Log: t.ToString(),
	})
}

func (t *AutoCertErrorLog) ToString() (content string) {

	for _, v := range t.buffer {
		content += fmt.Sprintf("[AutoCert Error] %s\n", v)
	}

	return
}

func AutoObtain() {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("AutoCert Recover", err)
		}
	}()
	logger.Info("AutoCert Worker Started")
	autoCertList := model.GetAutoCertList()
	for _, certModel := range autoCertList {
		confName := certModel.Filename

		errLog := &AutoCertErrorLog{}
		errLog.SetCertModel(certModel)

		if len(certModel.Filename) == 0 {
			errLog.Exit("", errors.New("filename is empty"))
			continue
		}

		if len(certModel.Domains) == 0 {
			errLog.Exit(confName, errors.New("domains list is empty, "+
				"try to reopen auto-cert for this config:"+confName))
			continue
		}

		if certModel.SSLCertificatePath != "" {
			cert, err := GetCertInfo(certModel.SSLCertificatePath)
			if err != nil {
				errLog.Push("get cert info", err)
				// Get certificate info error, ignore this domain
				continue
			}
			// every week
			if time.Now().Sub(cert.NotBefore).Hours()/24 < 7 {
				continue
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
		go IssueCert(payload, logChan, errChan)

		go handleIssueCertLogChan(logChan)

		// block, unless errChan closed
		for err := range errChan {
			errLog.Push("issue cert", err)
		}

		logStr := errLog.ToString()
		if logStr != "" {
			// store error log to db
			_ = certModel.Updates(&model.Cert{
				Log: errLog.ToString(),
			})
		} else {
			certModel.ClearLog()
		}

		close(logChan)
	}
	logger.Info("AutoCert Worker End")
}
