package certificate

import (
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
)

const (
	Success = "success"
	Info    = "info"
	Error   = "error"
)

type IssueCertResponse struct {
	Status            string `json:"status"`
	Message           string `json:"message"`
	SSLCertificate    string `json:"ssl_certificate,omitempty"`
	SSLCertificateKey string `json:"ssl_certificate_key,omitempty"`
}

func handleIssueCertLogChan(conn *websocket.Conn, log *cert.Logger, logChan chan string) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()

	for logString := range logChan {

		log.Info(logString)

		err := conn.WriteJSON(IssueCertResponse{
			Status:  Info,
			Message: logString,
		})

		if err != nil {
			logger.Error(err)
			return
		}
	}
}

func IssueCert(c *gin.Context) {
	var upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// upgrade http to websocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}

	defer func(ws *websocket.Conn) {
		_ = ws.Close()
	}(ws)

	// read
	payload := &cert.ConfigPayload{}

	err = ws.ReadJSON(payload)

	if err != nil {
		logger.Error(err)
		return
	}

	certModel, err := model.FirstOrCreateCert(c.Param("name"))

	if err != nil {
		logger.Error(err)
		return
	}

	logChan := make(chan string, 1)
	errChan := make(chan error, 1)

	log := &cert.Logger{}
	log.SetCertModel(&certModel)

	go cert.IssueCert(payload, logChan, errChan)

	go handleIssueCertLogChan(ws, log, logChan)

	// block, until errChan closes
	for err = range errChan {

		log.Error(err)

		// Save logs to db
		log.Exit()

		err = ws.WriteJSON(IssueCertResponse{
			Status:  Error,
			Message: err.Error(),
		})

		if err != nil {
			logger.Error(err)
			return
		}

		return
	}

	certDirName := strings.Join(payload.ServerName, "_")
	sslCertificatePath := nginx.GetConfPath("ssl", certDirName, "fullchain.cer")
	sslCertificateKeyPath := nginx.GetConfPath("ssl", certDirName, "private.key")

	err = certModel.Updates(&model.Cert{
		Domains:               payload.ServerName,
		SSLCertificatePath:    sslCertificatePath,
		SSLCertificateKeyPath: sslCertificateKeyPath,
	})

	if err != nil {
		logger.Error(err)
		err = ws.WriteJSON(IssueCertResponse{
			Status:  Error,
			Message: err.Error(),
		})
		return
	}

	// Save logs to db
	log.Exit()

	err = ws.WriteJSON(IssueCertResponse{
		Status:            Success,
		Message:           "Issued certificate successfully",
		SSLCertificate:    sslCertificatePath,
		SSLCertificateKey: sslCertificateKeyPath,
	})

	if err != nil {
		logger.Error(err)
		return
	}
}
