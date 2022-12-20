package cert

import (
	"github.com/0xJacky/Nginx-UI/server/model"
	"log"
	"time"
)

func handleIssueCertLogChan(logChan chan string) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("[Auto Cert] handleIssueCertLogChan", err)
		}
	}()

	for logString := range logChan {
		log.Println("[Auto Cert] Info", logString)
	}
}

func AutoCert() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("[AutoCert] Recover", err)
		}
	}()
	log.Println("[AutoCert] Start")
	autoCertList := model.GetAutoCertList()
	for i := range autoCertList {
		domain := autoCertList[i].Domain

		certModel, err := model.FirstCert(domain)

		if err != nil {
			log.Println("[AutoCert] Error get certificate from database", err)
			continue
		}

		if certModel.SSLCertificatePath == "" {
			log.Println("[AutoCert] Error ssl_certificate_path is empty, " +
				"try to reopen auto-cert for this domain:" + domain)
			continue
		}

		cert, err := GetCertInfo(certModel.SSLCertificatePath)
		if err != nil {
			log.Println("GetCertInfo Err", err)
			// Get certificate info error, ignore this domain
			continue
		}
		// before 1 mo
		if time.Now().Before(cert.NotBefore.AddDate(0, 1, 0)) {
			continue
		}
		// after 1 mo, reissue certificate
		logChan := make(chan string, 1)
		errChan := make(chan error, 1)

		go IssueCert([]string{domain}, logChan, errChan)

		go handleIssueCertLogChan(logChan)

		// block, unless errChan closed
		for err = range errChan {
			log.Println("Error cert.IssueCert", err)
		}

		close(logChan)
	}
}
