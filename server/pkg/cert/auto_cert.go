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

func AutoObtain() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("[AutoCert] Recover", err)
		}
	}()
	log.Println("[AutoCert] Start")
	autoCertList := model.GetAutoCertList()
	for _, certModel := range autoCertList {
		confName := certModel.Filename

		if certModel.SSLCertificatePath == "" {
			log.Println("[AutoCert] Error ssl_certificate_path is empty, " +
				"try to reopen auto-cert for this config:" + confName)
			continue
		}

		cert, err := GetCertInfo(certModel.SSLCertificatePath)
		if err != nil {
			log.Println("GetCertInfo Err", err)
			// Get certificate info error, ignore this domain
			continue
		}
		// every week
		if time.Now().Sub(cert.NotBefore).Hours()/24 < 7 {
			continue
		}
		//
		// after 1 mo, reissue certificate
		logChan := make(chan string, 1)
		errChan := make(chan error, 1)

		// support SAN certification
		go IssueCert(certModel.Domains, logChan, errChan)

		go handleIssueCertLogChan(logChan)

		// block, unless errChan closed
		for err = range errChan {
			log.Println("Error cert.IssueCert", err)
		}

		close(logChan)
	}
	log.Println("[AutoCert] End")
}
