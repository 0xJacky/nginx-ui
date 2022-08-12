package api

import (
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/pkg/cert"
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
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

func handleIssueCertLogChan(conn *websocket.Conn, logChan chan string) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("api.handleIssueCertLogChan recover", err)
		}
	}()

	for logString := range logChan {

		err := conn.WriteJSON(IssueCertResponse{
			Status:  Info,
			Message: logString,
		})

		if err != nil {
			log.Println("Error handleIssueCertLogChan", err)
			return
		}

	}
}

func IssueCert(c *gin.Context) {
	domain := c.Param("domain")

	var upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// upgrade http to websocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}

	defer func(ws *websocket.Conn) {
		err := ws.Close()
		if err != nil {
			log.Println("defer websocket close err", err)
		}
	}(ws)

	// read
	mt, message, err := ws.ReadMessage()

	if err != nil {
		log.Println(err)
		return
	}

	if mt != websocket.TextMessage || string(message) != "go" {
		return
	}

	logChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go cert.IssueCert(domain, logChan, errChan)

	go handleIssueCertLogChan(ws, logChan)

	// block, unless errChan closed
	for err = range errChan {
		log.Println("Error cert.IssueCert", err)

		err = ws.WriteJSON(IssueCertResponse{
			Status:  Error,
			Message: err.Error(),
		})

		if err != nil {
			log.Println(err)
			return
		}

		return
	}

	close(logChan)

	sslCertificatePath := nginx.GetNginxConfPath("ssl/" + domain + "/fullchain.cer")
	sslCertificateKeyPath := nginx.GetNginxConfPath("ssl/" + domain + "/" + domain + ".key")

	certModel, err := model.FirstCert(domain)

	if err != nil {
		log.Println(err)
		return
	}

	err = certModel.Updates(&model.Cert{
		SSLCertificatePath: sslCertificatePath,
	})

	if err != nil {
		log.Println(err)
		return
	}

	err = ws.WriteJSON(IssueCertResponse{
		Status:            Success,
		Message:           "Issued certificate successfully",
		SSLCertificate:    sslCertificatePath,
		SSLCertificateKey: sslCertificateKeyPath,
	})

	if err != nil {
		log.Println(err)
		return
	}

}
